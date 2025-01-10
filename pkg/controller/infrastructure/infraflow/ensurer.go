// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow/shared"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

// EnsureResourceGroup creates or updates the shoot's resource group.
func (fctx *FlowContext) EnsureResourceGroup(ctx context.Context) error {
	rg, err := fctx.ensureResourceGroup(ctx)
	if err != nil {
		return err
	}

	if err := fctx.inventory.Insert(*rg.ID); err != nil {
		return err
	}

	fctx.whiteboard.GetChild(ChildKeyIDs).Set(KindResourceGroup.String(), *rg.ID)
	return nil
}

func (fctx *FlowContext) ensureResourceGroup(ctx context.Context) (*armresources.ResourceGroup, error) {
	log := shared.LogFromContext(ctx)
	rgClient, err := fctx.factory.Group()
	if err != nil {
		return nil, err
	}

	rgCfg := fctx.adapter.ResourceGroup()
	rg, err := rgClient.Get(ctx, rgCfg.Name)
	if err != nil {
		return nil, err
	}

	if rg != nil {
		if location := ptr.Deref(rg.Location, ""); location != rgCfg.Location {
			// special case - return an error but do not proceed without user input.
			return nil, NewSpecMismatchError(rgCfg.AzureResourceMetadata, "location", rgCfg.Location, location,
				to.Ptr("This error is caused because the resource group location does not match the shoot's region. To proceed please delete the resource group"),
			)
		}

		return rg, nil
	}

	rg = &armresources.ResourceGroup{
		Location: to.Ptr(fctx.adapter.Region()),
	}

	log.Info("creating resource group", "name", fctx.adapter.ResourceGroupName())
	log.V(1).Info("creating resource group with the following spec", "spec", *rg)
	if rg, err = rgClient.CreateOrUpdate(ctx, fctx.adapter.ResourceGroupName(), *rg); err != nil {
		return nil, err
	}
	return rg, nil
}

// EnsureVirtualNetwork reconciles the shoot's virtual network. At the end of the step the VNet should be
// created or in the case of user-provided vnet verify that it exists.
func (fctx *FlowContext) EnsureVirtualNetwork(ctx context.Context) error {
	var vnet *armnetwork.VirtualNetwork
	var err error
	if fctx.adapter.VirtualNetworkConfig().Managed {
		vnet, err = fctx.ensureManagedVirtualNetwork(ctx)
		if err != nil {
			return err
		}

		if err := fctx.inventory.Insert(*vnet.ID); err != nil {
			return err
		}
	}

	vnet, err = fctx.ensureUserVirtualNetwork(ctx)
	if err != nil {
		return err
	}

	fctx.whiteboard.GetChild(ChildKeyIDs).Set(KindVirtualNetwork.String(), *vnet.ID)
	return nil
}

// EnsureVirtualNetwork creates or updates a Vnet
func (fctx *FlowContext) ensureManagedVirtualNetwork(ctx context.Context) (*armnetwork.VirtualNetwork, error) {
	log := shared.LogFromContext(ctx)
	vnetCfg := fctx.adapter.VirtualNetworkConfig()

	c, err := fctx.factory.Vnet()
	if err != nil {
		return nil, err
	}

	vnet, err := c.Get(ctx, vnetCfg.ResourceGroup, vnetCfg.Name)
	if err != nil {
		return nil, err
	}

	if vnet != nil {
		if location := ptr.Deref(vnet.Location, ""); location != fctx.adapter.Region() {
			log.Error(NewSpecMismatchError(vnetCfg.AzureResourceMetadata, "location", fctx.adapter.Region(), location, nil), "vnet can't be reconciled and has to be deleted")
			err = c.Delete(ctx, vnetCfg.ResourceGroup, vnetCfg.Name)
			if err != nil {
				return nil, err
			}
			fctx.inventory.Delete(*vnet.ID)
			vnet = nil
		}
	}

	vnet = vnetCfg.ToProvider(vnet)
	log.Info("reconciling virtual network", "name", vnetCfg.Name)
	log.V(1).Info("creating virtual network with spec", "spec", *vnet)
	vnet, err = c.CreateOrUpdate(ctx, vnetCfg.ResourceGroup, vnetCfg.Name, *vnet)
	if err != nil {
		return nil, err
	}

	return vnet, nil
}

func (fctx *FlowContext) ensureUserVirtualNetwork(ctx context.Context) (*armnetwork.VirtualNetwork, error) {
	log := shared.LogFromContext(ctx)
	vnetCfg := fctx.adapter.VirtualNetworkConfig()

	c, err := fctx.factory.Vnet()
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

	log.Info("found user virtual network", "name", vnetCfg.Name)
	return vnet, nil
}

// EnsureAvailabilitySet creates or updates an KindAvailabilitySet
func (fctx *FlowContext) EnsureAvailabilitySet(ctx context.Context) error {
	log := shared.LogFromContext(ctx)
	avsetCfg := fctx.adapter.AvailabilitySetConfig()
	if avsetCfg == nil {
		// should not reach here
		log.Info("skipping ensuring availability set")
		return nil
	}

	// complete AS migration.
	if v := fctx.whiteboard.GetChild(ChildKeyMigration).GetChild(KindAvailabilitySet.String()).Get(ChildKeyComplete); v != nil && *v == "true" {
		err := fctx.deleteAvailabilitySet(ctx, log)
		if err != nil {
			return err
		}
		return nil
	}

	avset, err := fctx.ensureAvailabilitySet(ctx, log)
	if err != nil {
		return err
	}

	err = fctx.inventory.Insert(*avset.ID)
	if err != nil {
		return err
	}
	fctx.whiteboard.GetChild(ChildKeyIDs).Set(KindAvailabilitySet.String(), *avset.ID)
	return nil
}
func (fctx *FlowContext) deleteAvailabilitySet(ctx context.Context, log logr.Logger) error {
	// try to delete the availability set. It can only  work if it does not contain any VMs.
	asClient, err := fctx.factory.AvailabilitySet()
	if err != nil {
		return err
	}

	av, err := asClient.Get(ctx, fctx.adapter.AvailabilitySetConfig().ResourceGroup, fctx.adapter.AvailabilitySetConfig().Name)
	if err != nil {
		return err
	}
	if av == nil {
		return nil
	}
	// if the AS contains no VMs then we attempt to delete it and complete the migration
	if len(av.Properties.VirtualMachines) == 0 {
		log.Info("Deleting Availability Set", "Name", *av.Name)
		if err := asClient.Delete(ctx, fctx.adapter.ResourceGroupName(), *av.Name); err != nil {
			return err
		}
		fctx.whiteboard.GetChild(ChildKeyIDs).Delete(KindAvailabilitySet.String())
		fctx.inventory.Delete(*av.ID)
		return nil
	}
	log.Info("Skipping deleting Availability Set because it still contains VMs", "Name", *av.Name)
	return nil
}

func (fctx *FlowContext) ensureAvailabilitySet(ctx context.Context, log logr.Logger) (*armcompute.AvailabilitySet, error) {
	asClient, err := fctx.factory.AvailabilitySet()
	if err != nil {
		return nil, err
	}

	avsetCfg := fctx.adapter.AvailabilitySetConfig()
	avset, err := asClient.Get(ctx, avsetCfg.ResourceGroup, avsetCfg.Name)
	if err != nil {
		return nil, err
	}

	if avset != nil {
		if location := ptr.Deref(avset.Location, ""); location != fctx.adapter.Region() {
			log.Error(NewSpecMismatchError(avsetCfg.AzureResourceMetadata, "location", fctx.adapter.Region(), location, nil), "will attempt to delete availability set due to irreconcilable error")
			err = asClient.Delete(ctx, avsetCfg.ResourceGroup, avsetCfg.Name)
			if err != nil {
				return nil, err
			}
		}

		// domain counts are immutable, therefore we need live with whatever is currently present.
		return avset, nil
	}

	avset = &armcompute.AvailabilitySet{
		Location: to.Ptr(fctx.adapter.Region()),
		// the DomainCounts are computed from the current InfrastructureStatus. They cannot be updated after shoot creation.
		Properties: &armcompute.AvailabilitySetProperties{
			PlatformFaultDomainCount:  avsetCfg.CountFaultDomains,
			PlatformUpdateDomainCount: avsetCfg.CountUpdateDomains,
		},
		SKU: &armcompute.SKU{Name: to.Ptr(string(armcompute.AvailabilitySetSKUTypesAligned))}, // equal to managed = True in tf
	}
	log.Info("reconciling availability set", "name", avset.Name)
	log.V(1).Info("reconciling availability set", "spec", *avset)
	return asClient.CreateOrUpdate(ctx, fctx.adapter.ResourceGroupName(), avsetCfg.Name, *avset)
}

// EnsureRouteTable creates or updates the route table
func (fctx *FlowContext) EnsureRouteTable(ctx context.Context) error {
	rt, err := fctx.ensureRouteTable(ctx)
	if err != nil {
		return err
	}

	err = fctx.inventory.Insert(*rt.ID)
	if err != nil {
		return nil
	}
	fctx.whiteboard.GetChild(ChildKeyIDs).Set(KindRouteTable.String(), *rt.ID)
	return nil
}

func (fctx *FlowContext) ensureRouteTable(ctx context.Context) (*armnetwork.RouteTable, error) {
	log := shared.LogFromContext(ctx)
	c, err := fctx.factory.RouteTables()
	if err != nil {
		return nil, err
	}

	rtCfg := fctx.adapter.RouteTableConfig()
	rt, err := c.Get(ctx, rtCfg.ResourceGroup, rtCfg.Name)
	if err != nil {
		return nil, err
	}

	if rt != nil {
		if location := ptr.Deref(rt.Location, ""); location != fctx.adapter.Region() {
			log.Error(NewSpecMismatchError(rtCfg.AzureResourceMetadata, "location", fctx.adapter.Region(), location, nil), "will attempt to delete route table due to irreconcilable error")
			err = c.Delete(ctx, rtCfg.ResourceGroup, rtCfg.Name)
			if err != nil {
				return nil, err
			}
			rt = nil
		}
	}

	rt = rtCfg.ToProvider(rt)
	log.Info("reconciling route table", "name", rtCfg.Name)
	log.V(1).Info("reconciling route table with spec", "spec", *rt)
	return c.CreateOrUpdate(ctx, rtCfg.ResourceGroup, rtCfg.Name, *rt)
}

// EnsureSecurityGroup creates or updates a KindSecurityGroup
func (fctx *FlowContext) EnsureSecurityGroup(ctx context.Context) error {
	log := shared.LogFromContext(ctx)
	sg, err := fctx.ensureSecurityGroup(ctx)
	if err != nil {
		return err
	}

	log.V(1).Info("adding to inventory", "id", *sg.ID)
	err = fctx.inventory.Insert(*sg.ID)
	if err != nil {
		return err
	}
	fctx.whiteboard.GetChild(ChildKeyIDs).Set(KindSecurityGroup.String(), *sg.ID)
	return nil
}

func (fctx *FlowContext) ensureSecurityGroup(ctx context.Context) (*armnetwork.SecurityGroup, error) {
	log := shared.LogFromContext(ctx)
	sgCfg := fctx.adapter.SecurityGroupConfig()

	c, err := fctx.factory.NetworkSecurityGroup()
	if err != nil {
		return nil, err
	}

	sg, err := c.Get(ctx, sgCfg.ResourceGroup, sgCfg.Name)
	if err != nil {
		return nil, err
	}

	if sg != nil {
		if location := ptr.Deref(sg.Location, ""); location != fctx.adapter.Region() {
			log.Error(NewSpecMismatchError(sgCfg.AzureResourceMetadata, "location", fctx.adapter.Region(), location, nil), "will attempt to delete security group due to irreconcilable error")
			err = c.Delete(ctx, sgCfg.ResourceGroup, sgCfg.Name)
			if err != nil {
				return nil, err
			}
			sg = nil
		}
	}

	sg = sgCfg.ToProvider(sg)
	log.Info("reconciling security group", "name", sgCfg.Name)
	log.V(1).Info("reconciling security group with spec", "spec", *sg)
	sg, err = c.CreateOrUpdate(ctx, sgCfg.ResourceGroup, sgCfg.Name, *sg)
	if err != nil {
		return nil, err
	}

	return sg, nil
}

// EnsurePublicIps reconciles the public IPs for the shoot.
func (fctx *FlowContext) EnsurePublicIps(ctx context.Context) error {
	return errors.Join(fctx.ensurePublicIps(ctx), fctx.ensureUserPublicIps(ctx))
}

func (fctx *FlowContext) ensureUserPublicIps(ctx context.Context) error {
	c, err := fctx.factory.PublicIP()
	if err != nil {
		return err
	}

	for _, ipFromConfig := range fctx.adapter.IpConfigs() {
		if !ipFromConfig.Managed {
			continue
		}
		err = errors.Join(err, fctx.ensureUserPublicIp(ctx, c, ipFromConfig))
	}
	return err
}

func (fctx *FlowContext) ensureUserPublicIp(ctx context.Context, c client.PublicIP, ipCfg PublicIPConfig) error {
	userIP, err := c.Get(ctx, ipCfg.ResourceGroup, ipCfg.Name, nil)
	if err != nil {
		return err
	} else if userIP == nil {
		return fmt.Errorf("failed to locate user public IP: %s, %s", ipCfg.ResourceGroup, ipCfg.Name)
	}

	fctx.whiteboard.GetChild(ChildKeyIDs).GetChild(ipCfg.ResourceGroup).GetChild(KindPublicIP.String()).Set(ipCfg.Name, *userIP.ID)
	return nil
}

func (fctx *FlowContext) ensurePublicIps(ctx context.Context) error {
	var (
		log         = shared.LogFromContext(ctx)
		toDelete    = map[string]string{}
		toReconcile = map[string]*armnetwork.PublicIPAddress{}
		joinError   error
	)

	c, err := fctx.factory.PublicIP()
	if err != nil {
		return err
	}

	currentIPs, err := c.List(ctx, fctx.adapter.ResourceGroupName())
	if err != nil {
		return err
	}
	currentIPs = Filter(currentIPs, func(address *armnetwork.PublicIPAddress) bool {
		// filter only these IpConfigs prefixed by the cluster name and that do not contain the CCM tags.
		return fctx.adapter.HasShootPrefix(address.Name) &&
			(address.Tags[azure.CCMServiceTagKey] == nil && address.Tags[azure.CCMLegacyServiceTagKey] == nil)
	})
	// obtain an indexed list of current IPs
	nameToCurrentIps := ToMap(currentIPs, func(t *armnetwork.PublicIPAddress) string {
		return *t.Name
	})

	desiredConfiguration := fctx.adapter.ManagedIpConfigs()
	for name, ip := range desiredConfiguration {
		toReconcile[name] = ip.ToProvider(nameToCurrentIps[name])
	}
	for _, inv := range fctx.inventory.ByKind(KindPublicIP) {
		if _, ok := nameToCurrentIps[inv.Name]; !ok {
			log.V(1).Info("removing public IP from inventory", "id", inv.String())
			fctx.inventory.Delete(inv.String())
		}
	}

	for name, current := range nameToCurrentIps {
		if err := fctx.inventory.Insert(*current.ID); err != nil {
			return err
		}
		// delete all the resources that are not in the list of target resources
		pipCfg, ok := desiredConfiguration[name]
		if !ok {
			log.Info("will delete public IP because it is not needed", "Resource Group", fctx.adapter.ResourceGroupName(), "Name", name)
			toDelete[name] = *current.ID
			continue
		}

		// delete all resources whose spec cannot be updated to match target spec.
		if ok, offender, v := ForceNewIp(current, toReconcile[pipCfg.Name]); ok {
			log.Info("will delete public IP because it can't be reconciled", "Resource Group", fctx.adapter.ResourceGroupName(), "Name", name, "Field", offender, "Value", v)
			toDelete[name] = *current.ID
			continue
		}
	}

	for ipName, ip := range toDelete {
		err := fctx.providerAccess.DeletePublicIP(ctx, fctx.adapter.ResourceGroupName(), ipName)
		if err != nil {
			joinError = errors.Join(joinError, err)
		} else {
			fctx.inventory.Delete(ip)
		}
	}

	if joinError != nil {
		return joinError
	}

	for ipName, ip := range toReconcile {
		ip, err = c.CreateOrUpdate(ctx, fctx.adapter.ResourceGroupName(), ipName, *ip)
		if err != nil {
			joinError = errors.Join(joinError, err)
			continue
		}

		if err := fctx.inventory.Insert(*ip.ID); err != nil {
			return err
		}
		fctx.whiteboard.GetChild(KindPublicIP.String()).GetChild(fctx.adapter.ResourceGroupName()).Set(ipName, *ip.ID)
	}

	return joinError
}

// EnsureNatGateways reconciles all the NAT Gateways for the shoot.
func (fctx *FlowContext) EnsureNatGateways(ctx context.Context) error {
	return fctx.ensureNatGateways(ctx)
}

// EnsureNatGateways creates or updates NAT Gateways. It also deletes old NATGateways.
func (fctx *FlowContext) ensureNatGateways(ctx context.Context) error {
	var (
		joinError   error
		log         = shared.LogFromContext(ctx)
		toDelete    = map[string]string{}
		toReconcile = map[string]*armnetwork.NatGateway{}
	)

	c, err := fctx.factory.NatGateway()
	if err != nil {
		return err
	}

	currentNats, err := c.List(ctx, fctx.adapter.ResourceGroupName())
	if err != nil {
		return err
	}
	// filter only thos prefixed by the cluster name.
	currentNats = Filter(currentNats, func(address *armnetwork.NatGateway) bool {
		return fctx.adapter.HasShootPrefix(address.Name)
	})

	// obtain an indexed list of current IPs
	nameToCurrentNats := ToMap(currentNats, func(t *armnetwork.NatGateway) string {
		return *t.Name
	})

	natsCfg := fctx.adapter.NatGatewayConfigs()
	for name, cfg := range natsCfg {
		target := cfg.ToProvider(nameToCurrentNats[name])
		for _, ip := range cfg.PublicIPList {
			target.Properties.PublicIPAddresses = append(target.Properties.PublicIPAddresses, &armnetwork.SubResource{ID: to.Ptr(GetIdFromTemplate(TemplatePublicIP, fctx.auth.SubscriptionID, ip.ResourceGroup, ip.Name))})
		}
		toReconcile[name] = target
	}

	for _, inv := range fctx.inventory.ByKind(KindNatGateway) {
		if _, ok := nameToCurrentNats[inv.Name]; !ok {
			log.V(1).Info("removing nat gateway from inventory", "id", inv.String())
			fctx.inventory.Delete(inv.String())
		}
	}

	for name, current := range nameToCurrentNats {
		if err := fctx.inventory.Insert(*current.ID); err != nil {
			return err
		}

		targetNat, ok := toReconcile[name]
		if !ok {
			log.Info("will delete NAT Gateway because it is not needed", "Resource Group", fctx.adapter.ResourceGroupName(), "Name", *current.Name)
			toDelete[name] = *current.ID
			continue
		}
		if ok, offender, v := ForceNewNat(current, targetNat); ok {
			log.Info("will delete NAT Gateway because it cannot be reconciled", "Resource Group", fctx.adapter.ResourceGroupName(), "Name", *current.Name, "Field", offender, "Value", v)
			toDelete[name] = *current.ID
			continue
		}
	}

	for natName, nat := range toDelete {
		err := fctx.providerAccess.DeleteNatGateway(ctx, fctx.adapter.ResourceGroupName(), natName)
		if err != nil {
			joinError = errors.Join(joinError, err)
		}
		fctx.inventory.Delete(nat)
	}
	if joinError != nil {
		return joinError
	}

	ipClient, _ := fctx.factory.PublicIP()
	ipAddresses := []string{}

	for name, nat := range toReconcile {
		nat, err := c.CreateOrUpdate(ctx, fctx.adapter.ResourceGroupName(), name, *nat)
		if err != nil {
			joinError = errors.Join(joinError, err)
			continue
		}
		if err := fctx.inventory.Insert(*nat.ID); err != nil {
			joinError = errors.Join(joinError, err)
			continue
		}
		fctx.whiteboard.GetChild(KindNatGateway.String()).Set(name, *nat.ID)

		for _, ip := range nat.Properties.PublicIPAddresses {
			resourceId, err := arm.ParseResourceID(*ip.ID)
			if err != nil {
				joinError = errors.Join(joinError, err)
				continue
			}
			ipObj, err := ipClient.Get(ctx, fctx.adapter.ResourceGroupName(), resourceId.Name, nil)
			if err != nil {
				joinError = errors.Join(joinError, err)
				continue
			}
			if ipObj == nil {
				continue
			}
			if ipObj.Properties.IPAddress != nil {
				ipAddresses = append(ipAddresses, *ipObj.Properties.IPAddress)
			}
		}
	}

	fctx.whiteboard.GetChild(KindNatGateway.String()).SetObject(KeyPublicIPAddresses, ipAddresses)

	return joinError
}

// EnsureSubnets creates or updates subnets.
func (fctx *FlowContext) EnsureSubnets(ctx context.Context) error {
	return fctx.ensureSubnets(ctx)
}

func (fctx *FlowContext) ensureSubnets(ctx context.Context) (err error) {
	var (
		log         = shared.LogFromContext(ctx)
		vnetRgroup  = fctx.adapter.VirtualNetworkConfig().ResourceGroup
		vnetName    = fctx.adapter.VirtualNetworkConfig().Name
		toDelete    = map[string]*armnetwork.Subnet{}
		toReconcile = map[string]*armnetwork.Subnet{}
		joinErr     error
	)

	c, err := fctx.factory.Subnet()
	if err != nil {
		return err
	}

	currentSubnets, err := c.List(ctx, vnetRgroup, vnetName)
	if err != nil {
		return err
	}

	// filteredSubnets are the subnets of this shoot. In a shared VNet scenario, it is not guaranteed that all subnets in the VNet belong to a particular shoot.
	filteredSubnets := Filter(currentSubnets, func(s *armnetwork.Subnet) bool {
		return fctx.adapter.IsOwnSubnetName(s.Name)
	})
	// mappedSubnets maps the unique subnet name to the subnet object.
	mappedSubnets := ToMap(filteredSubnets, func(s *armnetwork.Subnet) string {
		return *s.Name
	})
	// clean the current inventory and rebuild it.
	for _, resource := range fctx.inventory.ByKind(KindSubnet) {
		if _, ok := mappedSubnets[resource.Name]; !ok {
			log.V(1).Info("removing subnet from inventory", "id", resource.String())
			fctx.inventory.Delete(resource.String())
		}
	}

	zones := fctx.adapter.Zones()
	for _, z := range zones {
		actual := z.Subnet.ToProvider(mappedSubnets[z.Subnet.Name])

		rtCfg := fctx.adapter.RouteTableConfig()
		actual.Properties.RouteTable = &armnetwork.RouteTable{ID: to.Ptr(GetIdFromTemplate(TemplateRouteTable, fctx.auth.SubscriptionID, rtCfg.ResourceGroup, rtCfg.Name))}

		sgCfg := fctx.adapter.SecurityGroupConfig()
		actual.Properties.NetworkSecurityGroup = &armnetwork.SecurityGroup{ID: to.Ptr(GetIdFromTemplate(TemplateSecurityGroup, fctx.auth.SubscriptionID, sgCfg.ResourceGroup, sgCfg.Name))}

		if z.NatGateway != nil {
			actual.Properties.NatGateway = &armnetwork.SubResource{ID: to.Ptr(GetIdFromTemplate(TemplateNatGateway, fctx.auth.SubscriptionID, z.NatGateway.ResourceGroup, z.NatGateway.Name))}
		} else {
			// let's allow users to override the NAT Gateway config for a subnet, if that NGW is not managed by gardener.
			// It should only apply for existing subnets, hence we also check if actual.ID is not nil.
			if actual.ID != nil &&
				actual.Properties != nil &&
				actual.Properties.NatGateway != nil &&
				actual.Properties.NatGateway.ID != nil {
				resourceId, err := arm.ParseResourceID(*actual.Properties.NatGateway.ID)
				if err != nil {
					joinErr = errors.Join(joinErr, err)
					continue
				}
				// if this is a user-managed NAT gateway, do nothing. This is checked by looking at the resource group of the NGW.
				// In case that the NGW belongs to our RG, but it should not exist (z.NatGateway == nil), we remove the association.
				if resourceId.ResourceGroupName == fctx.adapter.ResourceGroupName() {
					actual.Properties.NatGateway = nil
				}
			}
		}
		toReconcile[z.Subnet.Name] = actual
	}

	for name, current := range mappedSubnets {
		if err := fctx.inventory.Insert(*current.ID); err != nil {
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
		fctx.inventory.Delete(*subnet.ID)
		fctx.whiteboard.GetChild(KindNatGateway.String()).Set(name, *subnet.ID)
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
		if err := fctx.inventory.Insert(*subnet.ID); err != nil {
			joinErr = errors.Join(joinErr, err)
			continue
		}
		fctx.whiteboard.GetChild(KindSubnet.String()).Set(name, *subnet.ID)
		if subnet.Properties.NatGateway != nil && subnet.Properties.NatGateway.ID != nil {
			fctx.whiteboard.GetChild(KindSubnet.String()).GetChild(KindNatGateway.String()).Set("id", *subnet.Properties.NatGateway.ID)
		}
	}

	return joinErr
}

// EnsureManagedIdentity reconciles the managed identity specificed in the config.
func (fctx *FlowContext) EnsureManagedIdentity(ctx context.Context) (err error) {
	if fctx.cfg.Identity == nil {
		return nil
	}

	c, err := fctx.factory.ManagedUserIdentity()
	if err != nil {
		return err
	}
	res, err := c.Get(ctx, fctx.cfg.Identity.ResourceGroup, fctx.cfg.Identity.Name)
	if err != nil {
		return err
	}
	if res.ID == nil || res.Properties.ClientID == nil {
		return nil
	}

	fctx.whiteboard.Set(KeyManagedIdentityClientId, *res.Properties.ClientID)
	fctx.whiteboard.Set(KeyManagedIdentityId, *res.ID)
	return err
}

// MigrateAvailabilitySet prepares an AS-based shoot to be migrated to VMSS-Flex.
func (fctx *FlowContext) MigrateAvailabilitySet(ctx context.Context) error {
	var (
		log = shared.LogFromContext(ctx)
		c   = fctx.client
	)

	// return early if the migration has already been complete
	if v := fctx.whiteboard.GetChild(ChildKeyMigration).GetChild(KindAvailabilitySet.String()).Get(ChildKeyComplete); v != nil && *v == "true" {
		return nil
	}
	// return early if the cluster does not have AS.
	if fctx.whiteboard.GetChild(ChildKeyIDs).Get(KindAvailabilitySet.String()) == nil {
		return nil
	}

	// IF VMOs are not needed, or the migration is already done, return early.
	if !helper.HasShootVmoMigrationAnnotation(fctx.cluster.Shoot.GetAnnotations()) {
		return nil
	}

	log.Info("Preparing for the migration to VMOs")
	scaleDownDeployment := func(ctx context.Context, key k8sclient.ObjectKey) error {
		log.Info("Scaling deployment to 0 replicas", "Name", key.String())
		deployment := &appsv1.Deployment{}
		if err := fctx.client.Get(ctx, k8sclient.ObjectKey{
			Namespace: fctx.infra.Namespace,
			Name:      azure.CloudControllerManagerName,
		}, deployment); k8sclient.IgnoreNotFound(err) != nil {
			return err
		} else if err != nil && apierrors.IsNotFound(err) {
			return nil
		}

		if ptr.Deref(deployment.Spec.Replicas, 1) == 0 {
			return nil
		}
		patch := k8sclient.MergeFrom(deployment.DeepCopy())
		deployment.Spec.Replicas = ptr.To(int32(0))
		// Apply the patch on the "scale" subresource
		if err := c.SubResource("scale").Patch(ctx, deployment, patch); err != nil {
			return fmt.Errorf("failed to patch deployment %s with replicas: %w", key.String(), err)
		}
		return nil
	}

	if err := flow.Parallel(func(ctx context.Context) error {
		return scaleDownDeployment(ctx, k8sclient.ObjectKey{Namespace: fctx.infra.Namespace, Name: azure.CloudControllerManagerName})
	}, func(ctx context.Context) error {
		// we want to scale CA down to avoid VMs getting created as they may claim internal subnet IPs in the case of an existing internal loadbalancer.
		return scaleDownDeployment(ctx, k8sclient.ObjectKey{Namespace: fctx.infra.Namespace, Name: "cluster-autoscaler"})
	}).RetryUntilTimeout(5*time.Second, defaultTimeout)(ctx); err != nil {
		return err
	}

	log.Info("Backing-up Public IPs to be migrated")
	// we will first back up the PIPs that we want to migrate. In the simplest case, we would only migrate PIPs in the shoot's RG. But the loadbalancer may reference PIPs from other RGs.
	// If we delete the LB we would have no way to recover the PIPs in other RGs, hence we will back it up.
	if err := fctx.BackupPIPsForBasicLBMigration(ctx); err != nil {
		return err
	}

	loadbalancerClient, err := fctx.factory.LoadBalancer()
	if err != nil {
		return err
	}
	loadbalancerName := fctx.adapter.TechnicalName()
	log.Info("Deleting load balancer", "Name", loadbalancerName)
	if err := loadbalancerClient.Delete(ctx, fctx.adapter.ResourceGroupName(), loadbalancerName); err != nil {
		return err
	}

	loadbalancerName = fmt.Sprintf("%s-internal", fctx.adapter.TechnicalName())
	log.Info("Deleting internal load balancer", "Name", loadbalancerName)
	if err := loadbalancerClient.Delete(ctx, fctx.adapter.ResourceGroupName(), loadbalancerName); err != nil {
		return err
	}

	if err := fctx.UpdatePublicIPs(ctx); err != nil {
		return err
	}
	fctx.whiteboard.GetChild(ChildKeyMigration).GetChild(KindAvailabilitySet.String()).Set(ChildKeyComplete, "true")
	return fctx.PersistState(ctx)
}

// GetInfrastructureStatus returns the infrastructure status.
func (fctx *FlowContext) GetInfrastructureStatus(_ context.Context) (*v1alpha1.InfrastructureStatus, error) {
	status := &v1alpha1.InfrastructureStatus{
		TypeMeta: infrastructure.StatusTypeMeta,
		Networks: v1alpha1.NetworkStatus{
			VNet: v1alpha1.VNetStatus{
				Name: fctx.adapter.VirtualNetworkConfig().Name,
			},
			Layout: v1alpha1.NetworkLayoutSingleSubnet,
		},
		ResourceGroup: v1alpha1.ResourceGroup{
			Name: fctx.adapter.ResourceGroupName(),
		},
		RouteTables: []v1alpha1.RouteTable{
			{
				Purpose: v1alpha1.PurposeNodes,
				Name:    fctx.adapter.RouteTableConfig().Name,
			},
		},
		SecurityGroups: []v1alpha1.SecurityGroup{
			{
				Purpose: v1alpha1.PurposeNodes,
				Name:    fctx.adapter.SecurityGroupConfig().Name,
			},
		},
		Zoned: fctx.cfg.Zoned,
	}

	if fctx.cfg.Networks.VNet.ResourceGroup != nil {
		status.Networks.VNet.ResourceGroup = to.Ptr(fctx.adapter.VirtualNetworkConfig().ResourceGroup)
	}

	if len(fctx.cfg.Networks.Zones) > 0 {
		status.Networks.Layout = v1alpha1.NetworkLayoutMultipleSubnet
	}

	zones := fctx.adapter.Zones()
	outboundAccessType := v1alpha1.OutboundAccessTypeNatGateway
	for _, z := range zones {
		subnet := v1alpha1.Subnet{
			Name:     z.Subnet.Name,
			Purpose:  v1alpha1.PurposeNodes,
			Zone:     z.Subnet.zone,
			Migrated: z.Migrated,
		}
		subnet.NatGatewayID = fctx.whiteboard.GetChild(KindSubnet.String()).GetChild(KindNatGateway.String()).Get("id")
		if subnet.NatGatewayID == nil {
			// if at least one of the "zones" does not have NATGateway enabled, mark the outbound access as OutboundAccessTypeLoadBalancer.
			outboundAccessType = v1alpha1.OutboundAccessTypeLoadBalancer
		}

		status.Networks.Subnets = append(status.Networks.Subnets, subnet)
	}
	status.Networks.OutboundAccessType = outboundAccessType

	if fctx.whiteboard.GetChild(ChildKeyIDs).Get(KindAvailabilitySet.String()) != nil {
		cfg := fctx.adapter.AvailabilitySetConfig()
		status.AvailabilitySets = []v1alpha1.AvailabilitySet{
			{
				Purpose:            v1alpha1.PurposeNodes,
				ID:                 GetIdFromTemplate(TemplateAvailabilitySet, fctx.auth.SubscriptionID, cfg.ResourceGroup, cfg.Name),
				Name:               cfg.Name,
				CountFaultDomains:  cfg.CountFaultDomains,
				CountUpdateDomains: cfg.CountUpdateDomains,
			},
		}
		if v := fctx.whiteboard.GetChild(ChildKeyMigration).GetChild(KindAvailabilitySet.String()).Get(ChildKeyComplete); v != nil && *v == "true" {
			status.MigratingToVMO = true
		}
	}

	if identity := fctx.cfg.Identity; identity != nil {
		status.Identity = &v1alpha1.IdentityStatus{
			ID:        *fctx.whiteboard.Get(KeyManagedIdentityId),
			ClientID:  *fctx.whiteboard.Get(KeyManagedIdentityClientId),
			ACRAccess: identity.ACRAccess != nil && *identity.ACRAccess,
		}
	}

	return status, nil
}

// GetInfrastructureState returns tha shoot's infrastructure state.
func (fctx *FlowContext) GetInfrastructureState() *runtime.RawExtension {
	state := &v1alpha1.InfrastructureState{
		TypeMeta:     helper.InfrastructureStateTypeMeta,
		ManagedItems: fctx.inventory.ToList(),
		Data:         fctx.whiteboard.ExportAsFlatMap(),
	}

	return &runtime.RawExtension{
		Object: state,
	}
}

// GetEgressIpCidrs retrieves the CIDRs of the IP ranges used for egress from the FlowContext
func (fctx *FlowContext) GetEgressIpCidrs() []string {
	if fctx.whiteboard.HasChild(KindNatGateway.String()) && fctx.whiteboard.GetChild(KindNatGateway.String()).HasObject(KeyPublicIPAddresses) {
		ipAddresses, ok := fctx.whiteboard.GetChild(KindNatGateway.String()).GetObject(KeyPublicIPAddresses).([]string)
		if !ok {
			return nil
		}
		cidrs := []string{}
		for _, address := range ipAddresses {
			cidrs = append(cidrs, address+"/32")
		}
		return cidrs
	}
	return nil
}

func (fctx *FlowContext) enrichStatusWithIdentity(_ context.Context, status *v1alpha1.InfrastructureStatus) error {
	if identity := fctx.cfg.Identity; identity != nil {
		status.Identity = &v1alpha1.IdentityStatus{
			ID:        *fctx.whiteboard.Get(KeyManagedIdentityId),
			ClientID:  *fctx.whiteboard.Get(KeyManagedIdentityClientId),
			ACRAccess: identity.ACRAccess != nil && *identity.ACRAccess,
		}
	}
	return nil
}

// DeleteResourceGroup deletes the shoot's resource group.
func (fctx *FlowContext) DeleteResourceGroup(ctx context.Context) error {
	c, err := fctx.factory.Group()
	if err != nil {
		return err
	}
	return c.Delete(ctx, fctx.adapter.ResourceGroupName())
}

// DeleteSubnetsInForeignGroup deletes all managed subnets in a foreign resource group
func (fctx *FlowContext) DeleteSubnetsInForeignGroup(ctx context.Context) error {
	vnetCfg := fctx.adapter.VirtualNetworkConfig()
	vnetRgroup := vnetCfg.ResourceGroup
	vnetName := vnetCfg.Name

	c, err := fctx.factory.Subnet()
	if err != nil {
		return err
	}

	currentSubnets, err := c.List(ctx, vnetRgroup, vnetName)

	// In case we cannot list any subnets at all, assume that the deletion succeeded at an earlier point in time.
	if client.FilterNotFoundError(err) != nil {
		return err
	}

	filteredSubnets := Filter(currentSubnets, func(s *armnetwork.Subnet) bool {
		return fctx.adapter.HasShootPrefix(s.Name)
	})

	var joinErr error
	for _, s := range filteredSubnets {
		err := c.Delete(ctx, vnetRgroup, vnetName, *s.Name)
		if err != nil {
			joinErr = errors.Join(joinErr, err)
		}
	}
	return joinErr
}

// DeleteLoadBalancers deletes all load balancers in shoots resource group
// This is a prerequisite for the deletion of the subnets in foreign resource group because
// internal load balancers might have a Frontend IP configuration referencing the
// foreign subnet which therefore can not be deleted. Since the Frontend IP configuration
// by its own can not be deleted, we remove the whole (all) load balancers.
func (fctx *FlowContext) DeleteLoadBalancers(ctx context.Context) error {
	c, err := fctx.factory.LoadBalancer()
	if err != nil {
		return err
	}

	resourceGroup := fctx.adapter.ResourceGroupName()

	loadBalancers, err := c.List(ctx, resourceGroup)

	// If we do not find any loadbalancers, assume the resource group was successfully deleted and we
	// are done.
	if client.FilterNotFoundError(err) != nil {
		return err
	}

	var joinErr error
	for _, lb := range loadBalancers {
		err := c.Delete(ctx, resourceGroup, *lb.Name)
		if err != nil {
			joinErr = errors.Join(joinErr, err)
		}
	}
	return joinErr
}
