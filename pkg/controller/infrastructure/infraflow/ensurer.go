//  Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package infraflow

import (
	"context"
	"errors"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

// EnsureResourceGroup creates or updates the shoot's resource group.
func (f *FlowContext) EnsureResourceGroup(ctx context.Context) error {
	log := f.LogFromContext(ctx)
	rg, err := f.ensureResourceGroup(ctx)
	if err != nil {
		return err
	}

	if err := f.inventory.Insert(*rg.ID); err != nil {
		return err
	}

	f.whiteboard.GetChild(ChildKeyIDs).Set(KindResourceGroup.String(), *rg.ID)

	// force persist to add at least one item to the inventory list.
	if perr := f.PersistState(ctx, true); perr != nil {
		log.Info("persisting state failed", "error", perr)
	}
	return nil
}

func (f *FlowContext) ensureResourceGroup(ctx context.Context) (*armresources.ResourceGroup, error) {
	log := f.LogFromContext(ctx)
	rgClient, err := f.factory.Group()
	if err != nil {
		return nil, err
	}

	rgCfg := f.adapter.ResourceGroup()
	rg, err := rgClient.Get(ctx, rgCfg.Name)
	if err != nil {
		return nil, err
	}

	if rg != nil {
		if location := pointer.StringDeref(rg.Location, ""); location != rgCfg.Location {
			// special case - return an error but do not proceed without user input.
			return nil, NewSpecMismatchError(rgCfg.AzureResourceMetadata, "location", rgCfg.Location, location,
				to.Ptr("This error is caused because the resource group location does not match the shoot's region. To proceed please delete the resource group"),
			)
		}

		return rg, nil
	}

	rg = &armresources.ResourceGroup{
		Location: to.Ptr(f.adapter.Region()),
	}

	log.V(2).Info("creating resource group", "name", f.adapter.ResourceGroupName())
	log.V(5).Info("creating resource group with the following spec", "spec", *rg)
	if rg, err = rgClient.CreateOrUpdate(ctx, f.adapter.ResourceGroupName(), *rg); err != nil {
		return nil, err
	}
	return rg, nil
}

// EnsureVirtualNetwork reconciles the shoot's virtual network. At the end of the step the VNet should be
// created or in the case of user-provided vnet verify that it exists.
func (f *FlowContext) EnsureVirtualNetwork(ctx context.Context) error {
	var vnet *armnetwork.VirtualNetwork
	var err error
	if f.adapter.VirtualNetworkConfig().Managed {
		vnet, err = f.ensureManagedVirtualNetwork(ctx)
		if err != nil {
			return err
		}

		if err := f.inventory.Insert(*vnet.ID); err != nil {
			return err
		}
	}

	vnet, err = f.ensureUserVirtualNetwork(ctx)
	if err != nil {
		return err
	}

	f.whiteboard.GetChild(ChildKeyIDs).Set(KindVirtualNetwork.String(), *vnet.ID)
	return nil
}

// EnsureVirtualNetwork creates or updates a Vnet
func (f *FlowContext) ensureManagedVirtualNetwork(ctx context.Context) (*armnetwork.VirtualNetwork, error) {
	log := f.LogFromContext(ctx)
	vnetCfg := f.adapter.VirtualNetworkConfig()

	c, err := f.factory.Vnet()
	if err != nil {
		return nil, err
	}

	vnet, err := c.Get(ctx, vnetCfg.ResourceGroup, vnetCfg.Name)
	if err != nil {
		return nil, err
	}

	if vnet != nil {
		if location := pointer.StringDeref(vnet.Location, ""); location != f.adapter.Region() {
			log.Error(NewSpecMismatchError(vnetCfg.AzureResourceMetadata, "location", f.adapter.Region(), location, nil), "vnet can't be reconciled and has to be deleted")
			err = c.Delete(ctx, vnetCfg.ResourceGroup, vnetCfg.Name)
			if err != nil {
				return nil, err
			}
			f.inventory.Delete(*vnet.ID)
			vnet = nil
		}
	}

	vnet = vnetCfg.ToProvider(vnet)
	log.V(2).Info("reconciling virtual network", "name", vnetCfg.Name)
	log.V(5).Info("creating virtual network with spec", "spec", *vnet)
	vnet, err = c.CreateOrUpdate(ctx, vnetCfg.ResourceGroup, vnetCfg.Name, *vnet)
	if err != nil {
		return nil, err
	}

	return vnet, nil
}

func (f *FlowContext) ensureUserVirtualNetwork(ctx context.Context) (*armnetwork.VirtualNetwork, error) {
	log := f.LogFromContext(ctx)
	vnetCfg := f.adapter.VirtualNetworkConfig()

	c, err := f.factory.Vnet()
	if err != nil {
		return nil, err
	}

	vnet, err := c.Get(ctx, vnetCfg.ResourceGroup, vnetCfg.Name)
	if err != nil {
		return nil, err
	}

	if vnet == nil {
		return nil, NewTerminalConditionError(vnetCfg.AzureResourceMetadata, fmt.Errorf("user vnet not found"))
	}

	log.V(5).Info("found user virtual network", "name", vnetCfg.Name)
	return vnet, nil
}

// EnsureAvailabilitySet creates or updates an KindAvailabilitySet
func (f *FlowContext) EnsureAvailabilitySet(ctx context.Context) error {
	log := f.LogFromContext(ctx)
	avsetCfg := f.adapter.AvailabilitySetConfig()
	if avsetCfg == nil {
		// should not reach here
		log.Info("skipping ensuring availability set")
		return nil
	}

	avset, err := f.ensureAvailabilitySet(ctx, log, *avsetCfg)
	if err != nil {
		return err
	}

	err = f.inventory.Insert(*avset.ID)
	if err != nil {
		return err
	}
	f.whiteboard.GetChild(ChildKeyIDs).Set(KindAvailabilitySet.String(), *avset.ID)
	return nil
}

func (f *FlowContext) ensureAvailabilitySet(ctx context.Context, log logr.Logger, avsetCfg AvailabilitySetConfig) (*armcompute.AvailabilitySet, error) {
	asClient, err := f.factory.AvailabilitySet()
	if err != nil {
		return nil, err
	}

	avset, err := asClient.Get(ctx, avsetCfg.ResourceGroup, avsetCfg.Name)
	if err != nil {
		return nil, err
	}

	if avset != nil {
		if location := pointer.StringDeref(avset.Location, ""); location != f.adapter.Region() {
			log.Error(NewSpecMismatchError(avsetCfg.AzureResourceMetadata, "location", f.adapter.Region(), location, nil), "will attempt to delete availability set due to irreconcilable error")
			err = asClient.Delete(ctx, avsetCfg.ResourceGroup, avsetCfg.Name)
			if err != nil {
				return nil, err
			}
		}

		// domain counts are immutable, therefore we need live with whatever is currently present.
		return avset, nil
	}

	avset = &armcompute.AvailabilitySet{
		Location: to.Ptr(f.adapter.Region()),
		// the DomainCounts are computed from the current InfrastructureStatus. They cannot be updated after shoot creation.
		Properties: &armcompute.AvailabilitySetProperties{
			PlatformFaultDomainCount:  avsetCfg.CountFaultDomains,
			PlatformUpdateDomainCount: avsetCfg.CountUpdateDomains,
		},
		SKU: &armcompute.SKU{Name: to.Ptr(string(armcompute.AvailabilitySetSKUTypesAligned))}, // equal to managed = True in tf
	}
	log.V(2).Info("reconciling availability set", "name", avset.Name)
	log.V(5).Info("reconciling availability set", "spec", *avset)
	return asClient.CreateOrUpdate(ctx, f.adapter.ResourceGroupName(), avsetCfg.Name, *avset)
}

// EnsureRouteTable creates or updates the route table
func (f *FlowContext) EnsureRouteTable(ctx context.Context) error {
	rt, err := f.ensureRouteTable(ctx)
	if err != nil {
		return err
	}

	err = f.inventory.Insert(*rt.ID)
	if err != nil {
		return nil
	}
	f.whiteboard.GetChild(ChildKeyIDs).Set(KindRouteTable.String(), *rt.ID)
	return nil
}

func (f *FlowContext) ensureRouteTable(ctx context.Context) (*armnetwork.RouteTable, error) {
	log := f.LogFromContext(ctx)
	c, err := f.factory.RouteTables()
	if err != nil {
		return nil, err
	}

	rtCfg := f.adapter.RouteTableConfig()
	rt, err := c.Get(ctx, rtCfg.ResourceGroup, rtCfg.Name)
	if err != nil {
		return nil, err
	}

	if rt != nil {
		if location := pointer.StringDeref(rt.Location, ""); location != f.adapter.Region() {
			log.Error(NewSpecMismatchError(rtCfg.AzureResourceMetadata, "location", f.adapter.Region(), location, nil), "will attempt to delete route table due to irreconcilable error")
			err = c.Delete(ctx, rtCfg.ResourceGroup, rtCfg.Name)
			if err != nil {
				return nil, err
			}
			rt = nil
		}
	}

	rt = rtCfg.ToProvider(rt)
	log.V(2).Info("reconciling route table", "name", rtCfg.Name)
	log.V(5).Info("reconciling route table with spec", "spec", *rt)
	return c.CreateOrUpdate(ctx, rtCfg.ResourceGroup, rtCfg.Name, *rt)
}

// EnsureSecurityGroup creates or updates a KindSecurityGroup
func (f *FlowContext) EnsureSecurityGroup(ctx context.Context) error {
	log := f.LogFromContext(ctx)
	sg, err := f.ensureSecurityGroup(ctx)
	if err != nil {
		return err
	}

	log.V(5).Info("adding to inventory", *sg.ID)
	err = f.inventory.Insert(*sg.ID)
	if err != nil {
		return err
	}
	f.whiteboard.GetChild(ChildKeyIDs).Set(KindSecurityGroup.String(), *sg.ID)
	return nil
}

func (f *FlowContext) ensureSecurityGroup(ctx context.Context) (*armnetwork.SecurityGroup, error) {
	log := f.LogFromContext(ctx)
	sgCfg := f.adapter.SecurityGroupConfig()

	c, err := f.factory.NetworkSecurityGroup()
	if err != nil {
		return nil, err
	}

	sg, err := c.Get(ctx, sgCfg.ResourceGroup, sgCfg.Name)
	if err != nil {
		return nil, err
	}

	if sg != nil {
		if location := pointer.StringDeref(sg.Location, ""); location != f.adapter.Region() {
			log.Error(NewSpecMismatchError(sgCfg.AzureResourceMetadata, "location", f.adapter.Region(), location, nil), "will attempt to delete security group due to irreconcilable error")
			err = c.Delete(ctx, sgCfg.ResourceGroup, sgCfg.Name)
			if err != nil {
				return nil, err
			}
			sg = nil
		}
	}

	sg = sgCfg.ToProvider(sg)
	log.V(2).Info("reconciling security group", "name", sgCfg.Name)
	log.V(5).Info("reconciling security group with spec", "spec", *sg)
	sg, err = c.CreateOrUpdate(ctx, sgCfg.ResourceGroup, sgCfg.Name, *sg)
	if err != nil {
		return nil, err
	}

	return sg, nil
}

// EnsurePublicIps reconciles the public IPs for the shoot.
func (f *FlowContext) EnsurePublicIps(ctx context.Context) error {
	return errors.Join(f.ensurePublicIps(ctx), f.ensureUserPublicIps(ctx))
}

func (f *FlowContext) ensureUserPublicIps(ctx context.Context) error {
	c, err := f.factory.PublicIP()
	if err != nil {
		return err
	}

	for _, ipFromConfig := range f.adapter.IpConfigs() {
		if !ipFromConfig.Managed {
			continue
		}
		err = errors.Join(err, f.ensureUserPublicIp(ctx, c, ipFromConfig))
	}
	return err
}

func (f *FlowContext) ensureUserPublicIp(ctx context.Context, c client.PublicIP, ipCfg PublicIPConfig) error {
	userIP, err := c.Get(ctx, ipCfg.ResourceGroup, ipCfg.Name, nil)
	if err != nil {
		return err
	} else if userIP == nil {
		return fmt.Errorf(fmt.Sprintf("failed to locate user public IP: %s, %s", ipCfg.ResourceGroup, ipCfg.Name))
	}

	f.whiteboard.GetChild(ChildKeyIDs).GetChild(ipCfg.ResourceGroup).GetChild(KindPublicIP.String()).Set(ipCfg.Name, *userIP.ID)
	return nil
}

func (f *FlowContext) ensurePublicIps(ctx context.Context) error {
	var (
		log         = f.LogFromContext(ctx)
		toDelete    = map[string]string{}
		toReconcile = map[string]*armnetwork.PublicIPAddress{}
		joinError   error
	)

	c, err := f.factory.PublicIP()
	if err != nil {
		return err
	}

	currentIPs, err := c.List(ctx, f.adapter.ResourceGroupName())
	if err != nil {
		return err
	}
	currentIPs = Filter(currentIPs, func(address *armnetwork.PublicIPAddress) bool {
		// filter only these IpConfigs prefixed by the cluster name and that do not contain the CCM tags.
		return f.adapter.HasShootPrefix(address.Name) && address.Tags["k8s-azure-service"] == nil
	})
	// obtain an indexed list of current IPs
	nameToCurrentIps := ToMap(currentIPs, func(t *armnetwork.PublicIPAddress) string {
		return *t.Name
	})

	desiredConfiguration := f.adapter.ManagedIpConfigs()
	for name, ip := range desiredConfiguration {
		toReconcile[name] = ip.ToProvider(nameToCurrentIps[name])
	}
	for _, inv := range f.inventory.ByKind(KindPublicIP) {
		if ip, ok := nameToCurrentIps[inv]; !ok {
			f.inventory.Delete(*ip.ID)
		}
	}

	for name, current := range nameToCurrentIps {
		if err := f.inventory.Insert(*current.ID); err != nil {
			return err
		}
		// delete all the resources that are not in the list of target resources
		pipCfg, ok := desiredConfiguration[name]
		if !ok {
			log.Info("will delete public IP because it is not needed", "Resource Group", f.adapter.ResourceGroupName(), "Name", name)
			toDelete[name] = *current.ID
			continue
		}

		// delete all resources whose spec cannot be updated to match target spec.
		if ok, offender, v := ForceNewIp(current, toReconcile[pipCfg.Name]); ok {
			log.Info("will delete public IP because it can't be reconciled", "Resource Group", f.adapter.ResourceGroupName(), "Name", name, "Field", offender, "Value", v)
			toDelete[name] = *current.ID
			continue
		}
	}

	for ipName, ip := range toDelete {
		err := f.provider.DeletePublicIP(ctx, f.adapter.ResourceGroupName(), ipName)
		if err != nil {
			joinError = errors.Join(joinError, err)
		}
		f.inventory.Delete(ip)
	}

	if joinError != nil {
		return joinError
	}

	for ipName, ip := range toReconcile {
		ip, err = c.CreateOrUpdate(ctx, f.adapter.ResourceGroupName(), ipName, *ip)
		if err != nil {
			joinError = errors.Join(joinError, err)
			continue
		}

		if err := f.inventory.Insert(*ip.ID); err != nil {
			return err
		}
		f.whiteboard.GetChild(KindPublicIP.String()).GetChild(f.adapter.ResourceGroupName()).Set(ipName, *ip.ID)
	}

	return joinError
}

// EnsureNatGateways reconciles all the NAT Gateways for the shoot.
func (f *FlowContext) EnsureNatGateways(ctx context.Context) error {
	err := f.ensureNatGateways(ctx)
	return err
}

// EnsureNatGateways creates or updates NAT Gateways. It also deletes old NATGateways.
func (f *FlowContext) ensureNatGateways(ctx context.Context) error {
	// func (f *FlowContext) ensureNatGateways(ctx context.Context, ipMapping map[AzureResourceMetadata]string) error {
	var (
		log         = f.LogFromContext(ctx)
		joinError   error
		toDelete    = map[string]string{}
		toReconcile = map[string]*armnetwork.NatGateway{}
	)

	c, err := f.factory.NatGateway()
	if err != nil {
		return err
	}

	currentNats, err := c.List(ctx, f.adapter.ResourceGroupName())
	if err != nil {
		return err
	}
	// filter only thos prefixed by the cluster name.
	currentNats = Filter(currentNats, func(address *armnetwork.NatGateway) bool {
		return f.adapter.HasShootPrefix(address.Name)
	})

	// obtain an indexed list of current IPs
	nameToCurrentNats := ToMap(currentNats, func(t *armnetwork.NatGateway) string {
		return *t.Name
	})

	natsCfg := f.adapter.NatGatewayConfigs()
	for name, cfg := range natsCfg {
		target := cfg.ToProvider(nameToCurrentNats[name])
		for _, ip := range cfg.PublicIPList {
			target.Properties.PublicIPAddresses = append(target.Properties.PublicIPAddresses, &armnetwork.SubResource{ID: to.Ptr(GetIdFromTemplate(TemplatePublicIP, f.auth.SubscriptionID, ip.ResourceGroup, ip.Name))})
		}
		toReconcile[name] = target
	}

	for _, inv := range f.inventory.ByKind(KindNatGateway) {
		if nat, ok := nameToCurrentNats[inv]; !ok {
			f.inventory.Delete(*nat.ID)
		}
	}

	for name, current := range nameToCurrentNats {
		if err := f.inventory.Insert(*current.ID); err != nil {
			return err
		}

		targetNat, ok := toReconcile[name]
		if !ok {
			log.Info("will delete NAT Gateway because it is not needed", "Resource Group", f.adapter.ResourceGroupName(), "Name", *current.Name)
			toDelete[name] = *current.ID
			continue
		}
		if ok, offender, v := ForceNewNat(current, targetNat); ok {
			log.Info("will delete NAT Gateway because it cannot be reconciled", "Resource Group", f.adapter.ResourceGroupName(), "Name", *current.Name, "Field", offender, "Value", v)
			toDelete[name] = *current.ID
			continue
		}
	}
	if joinError != nil {
		return joinError
	}

	for natName, nat := range toDelete {
		err := f.provider.DeleteNatGateway(ctx, f.adapter.ResourceGroupName(), natName)
		if err != nil {
			joinError = errors.Join(joinError, err)
		}
		f.inventory.Delete(nat)
	}
	if joinError != nil {
		return joinError
	}

	for name, nat := range toReconcile {
		nat, err := c.CreateOrUpdate(ctx, f.adapter.ResourceGroupName(), name, *nat)
		if err != nil {
			joinError = errors.Join(joinError, err)
			continue
		}
		if err := f.inventory.Insert(*nat.ID); err != nil {
			joinError = errors.Join(joinError, err)
			continue
		}
		f.whiteboard.GetChild(KindNatGateway.String()).Set(name, *nat.ID)
	}

	return joinError
}

// EnsureSubnets creates or updates subnets.
func (f *FlowContext) EnsureSubnets(ctx context.Context) error {
	return f.ensureSubnets(ctx)
}

func (f *FlowContext) ensureSubnets(ctx context.Context) (err error) {
	var (
		log         = f.LogFromContext(ctx)
		vnetRgroup  = f.adapter.VirtualNetworkConfig().ResourceGroup
		vnetName    = f.adapter.VirtualNetworkConfig().Name
		toDelete    = map[string]*armnetwork.Subnet{}
		toReconcile = map[string]*armnetwork.Subnet{}
		joinErr     error
	)

	c, err := f.factory.Subnet()
	if err != nil {
		return err
	}

	currentSubnets, err := c.List(ctx, vnetRgroup, vnetName)
	if err != nil {
		return err
	}

	filteredSubnets := Filter(currentSubnets, func(s *armnetwork.Subnet) bool {
		return f.adapter.HasShootPrefix(s.Name)
	})
	mappedSubnets := ToMap(filteredSubnets, func(s *armnetwork.Subnet) string {
		return *s.Name
	})

	for _, name := range f.inventory.ByKind(KindSubnet) {
		if subnet, ok := mappedSubnets[name]; !ok {
			f.inventory.Delete(*subnet.ID)
		}
	}

	zones := f.adapter.Zones()
	for _, z := range zones {
		actual := z.Subnet.ToProvider(mappedSubnets[z.Subnet.Name])
		rtCfg := f.adapter.RouteTableConfig()
		sgCfg := f.adapter.SecurityGroupConfig()
		actual.Properties.RouteTable = &armnetwork.RouteTable{ID: to.Ptr(GetIdFromTemplate(TemplateRouteTable, f.auth.SubscriptionID, rtCfg.ResourceGroup, rtCfg.Name))}
		actual.Properties.NetworkSecurityGroup = &armnetwork.SecurityGroup{ID: to.Ptr(GetIdFromTemplate(TemplateSecurityGroup, f.auth.SubscriptionID, sgCfg.ResourceGroup, sgCfg.Name))}
		if z.NatGateway != nil {
			actual.Properties.NatGateway = &armnetwork.SubResource{ID: to.Ptr(GetIdFromTemplate(TemplateNatGateway, f.auth.SubscriptionID, z.NatGateway.ResourceGroup, z.NatGateway.Name))}
		}
		toReconcile[z.Subnet.Name] = actual
	}

	for name, current := range mappedSubnets {
		if err := f.inventory.Insert(*current.ID); err != nil {
			return err
		}

		target, ok := toReconcile[name]
		if !ok {
			log.Info("will delete subnet because it is not needed", "Resource Group", vnetRgroup, "Name", *current.Name)
			toDelete[name] = current
			continue
		}
		if ok, offender, v := ForceNewSubnet(current, target); ok {
			log.Info("will delete subnet because it cannot be reconciled", "Resource Group", vnetRgroup, "Name", *current.Name, "Field", offender, "Value", v)
			toDelete[name] = current
			continue
		}
	}

	for name, subnet := range toDelete {
		err := c.Delete(ctx, vnetRgroup, vnetName, name)
		if err != nil {
			joinErr = errors.Join(joinErr, err)
		}
		f.inventory.Delete(*subnet.ID)
		f.whiteboard.GetChild(KindNatGateway.String()).Set(name, *subnet.ID)
	}
	if joinErr != nil {
		return joinErr
	}

	for name, subnet := range toReconcile {
		subnet, err = c.CreateOrUpdate(ctx, vnetRgroup, vnetName, name, *subnet)
		if err != nil {
			joinErr = errors.Join(joinErr, err)
			continue
		}
		if err := f.inventory.Insert(*subnet.ID); err != nil {
			joinErr = errors.Join(joinErr, err)
			continue
		}
		f.whiteboard.GetChild(KindSubnet.String()).Set(name, *subnet.ID)
	}

	return joinErr
}

// EnsureManagedIdentity reconciles the managed identity specificed in the config.
func (f *FlowContext) EnsureManagedIdentity(ctx context.Context) (err error) {
	if f.cfg.Identity == nil {
		return nil
	}

	c, err := f.factory.ManagedUserIdentity()
	if err != nil {
		return err
	}
	res, err := c.Get(ctx, f.cfg.Identity.ResourceGroup, f.cfg.Identity.Name)
	if err != nil {
		return err
	}
	if res.ID == nil || res.ClientID == nil {
		return nil
	}

	f.whiteboard.Set(KeyManagedIdentityClientId, res.ClientID.String())
	f.whiteboard.Set(KeyManagedIdentityId, *res.ID)
	return err
}

// GetInfrastructureStatus returns the infrastructure status.
func (f *FlowContext) GetInfrastructureStatus(_ context.Context) (*v1alpha1.InfrastructureStatus, error) {
	status := &v1alpha1.InfrastructureStatus{
		TypeMeta: infrastructure.StatusTypeMeta,
		Networks: v1alpha1.NetworkStatus{
			VNet: v1alpha1.VNetStatus{
				Name: f.adapter.VirtualNetworkConfig().ResourceGroup,
			},
			Layout: v1alpha1.NetworkLayoutSingleSubnet,
		},
		ResourceGroup: v1alpha1.ResourceGroup{
			Name: f.adapter.ResourceGroupName(),
		},
		RouteTables: []v1alpha1.RouteTable{
			{
				Purpose: v1alpha1.PurposeNodes,
				Name:    f.adapter.RouteTableConfig().Name,
			},
		},
		SecurityGroups: []v1alpha1.SecurityGroup{
			{
				Purpose: v1alpha1.PurposeNodes,
				Name:    f.adapter.SecurityGroupConfig().Name,
			},
		},
		Zoned: f.cfg.Zoned,
	}

	if f.cfg.Networks.VNet.ResourceGroup != nil {
		status.Networks.VNet.ResourceGroup = to.Ptr(f.adapter.VirtualNetworkConfig().ResourceGroup)
	}

	if len(f.cfg.Networks.Zones) > 0 {
		status.Networks.Layout = v1alpha1.NetworkLayoutMultipleSubnet
	}

	zones := f.adapter.Zones()
	for _, z := range zones {
		status.Networks.Subnets = append(status.Networks.Subnets, v1alpha1.Subnet{
			Name:     z.Subnet.Name,
			Purpose:  v1alpha1.PurposeNodes,
			Zone:     z.Subnet.zone,
			Migrated: z.Migrated,
		})
	}

	if cfg := f.adapter.AvailabilitySetConfig(); cfg != nil {
		status.AvailabilitySets = []v1alpha1.AvailabilitySet{
			{
				Purpose:            v1alpha1.PurposeNodes,
				ID:                 GetIdFromTemplate(TemplateAvailabilitySet, f.auth.SubscriptionID, cfg.ResourceGroup, cfg.Name),
				Name:               cfg.Name,
				CountFaultDomains:  cfg.CountFaultDomains,
				CountUpdateDomains: cfg.CountUpdateDomains,
			},
		}
	}

	if identity := f.cfg.Identity; identity != nil {
		status.Identity = &v1alpha1.IdentityStatus{
			ID:        *f.whiteboard.Get(KeyManagedIdentityId),
			ClientID:  *f.whiteboard.Get(KeyManagedIdentityClientId),
			ACRAccess: identity.ACRAccess != nil && *identity.ACRAccess,
		}
	}

	return status, nil
}

// GetInfrastructureState returns tha shoot's infrastructure state.
func (f *FlowContext) GetInfrastructureState() (*runtime.RawExtension, error) {
	state := &v1alpha1.InfrastructureState{
		TypeMeta:     helper.InfrastructureStateTypeMeta,
		ManagedItems: f.inventory.ToList(),
	}

	return &runtime.RawExtension{
		Object: state,
	}, nil
}

func (f *FlowContext) enrichStatusWithIdentity(_ context.Context, status *v1alpha1.InfrastructureStatus) error {
	if identity := f.cfg.Identity; identity != nil {
		status.Identity = &v1alpha1.IdentityStatus{
			ID:        *f.whiteboard.Get(KeyManagedIdentityId),
			ClientID:  *f.whiteboard.Get(KeyManagedIdentityClientId),
			ACRAccess: identity.ACRAccess != nil && *identity.ACRAccess,
		}
	}
	return nil
}

// DeleteResourceGroup deletes the shoot's resource group.
func (f *FlowContext) DeleteResourceGroup(ctx context.Context) error {
	c, err := f.factory.Group()
	if err != nil {
		return err
	}
	return c.Delete(ctx, f.adapter.ResourceGroupName())
}

// DeleteSubnetsInForeignGroup deletes all managed subnets in a foreign resource group
func (f *FlowContext) DeleteSubnetsInForeignGroup(ctx context.Context) error {
	vnetCfg := f.adapter.VirtualNetworkConfig()
	if vnetCfg.Managed {
		return nil
	}

	vnetRgroup := vnetCfg.ResourceGroup
	vnetName := vnetCfg.Name

	c, err := f.factory.Subnet()
	if err != nil {
		return err
	}

	currentSubnets, err := c.List(ctx, vnetRgroup, vnetName)
	if err != nil {
		return err
	}

	filteredSubnets := Filter(currentSubnets, func(s *armnetwork.Subnet) bool {
		return f.adapter.HasShootPrefix(s.Name)
	})

	var joinErr error
	for _, s := range filteredSubnets {
		err := c.Delete(ctx, vnetRgroup, vnetName, *s.Name)
		if err != nil {
			joinErr = errors.Join(joinErr, err)
			continue
		}
	}
	return joinErr
}
