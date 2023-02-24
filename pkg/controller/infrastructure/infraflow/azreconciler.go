// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package infraflow

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-05-01/network"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"k8s.io/utils/pointer"
)

// azureReconciler allows to reconcile the individual cloud resources
type azureReconciler struct {
	tf      TerraformAdapter
	factory client.Factory
}

// NewAzureReconciler creates a new azureReconciler
func NewAzureReconciler(infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig, cluster *controller.Cluster, factory client.Factory) (*azureReconciler, error) {
	tfAdapter, err := NewTerraformAdapter(infra, cfg, cluster)
	return &azureReconciler{tf: tfAdapter, factory: factory}, err
}

// GetInfrastructureStatus returns the infrastructure status
func (f azureReconciler) GetInfrastructureStatus(ctx context.Context) (*v1alpha1.InfrastructureStatus, error) {
	status := f.tf.StaticInfrastructureStatus()
	err := f.enrichStatusWithIdentity(ctx, status)
	if err != nil {
		return status, err
	}
	err = f.enrichStatusWithAvailabilitySet(ctx, status)
	if err != nil {
		return status, err
	}
	return status, nil
}

func (f azureReconciler) enrichStatusWithAvailabilitySet(ctx context.Context, status *v1alpha1.InfrastructureStatus) error {
	if f.tf.isCreate(AvailabilitySet) {
		client, err := f.factory.AvailabilitySet()
		if err != nil {
			return err
		}
		avset := f.tf.AvailabilitySet()
		res, err := client.Get(ctx, f.tf.ResourceGroup(), avset.Name)
		if err != nil {
			return err
		}
		status.AvailabilitySets = append(status.AvailabilitySets, v1alpha1.AvailabilitySet{
			Name:               avset.Name,
			ID:                 *res.ID,
			CountFaultDomains:  pointer.Int32Ptr(avset.CountFaultDomains),
			CountUpdateDomains: pointer.Int32Ptr(avset.CountUpdateDomains),
			Purpose:            v1alpha1.PurposeNodes,
		})
	}
	return nil
}

func (f azureReconciler) enrichStatusWithIdentity(ctx context.Context, status *v1alpha1.InfrastructureStatus) error {
	if identity := f.tf.Identity(); identity != nil {
		client, err := f.factory.ManagedUserIdentity()
		if err != nil {
			return err
		}
		res, err := client.Get(ctx, identity.ResourceGroup, identity.Name)
		if err != nil {
			return err
		}
		if res.ID == nil || res.ClientID == nil {
			return nil
		}

		status.Identity = &v1alpha1.IdentityStatus{
			ID:       *res.ID,
			ClientID: res.ClientID.String(),
		}
	}
	return nil
}

// Delete deletes all resources managed by the reconciler
func (f azureReconciler) Delete(ctx context.Context) error {
	client, err := f.factory.Group()
	if err != nil {
		return err
	}
	if err := f.deleteSubnetsInForeignGroup(ctx); err != nil {
		return fmt.Errorf("failed to delete foreign subnet: %w", err)
	}
	return client.DeleteIfExists(ctx, f.tf.ResourceGroup())
}

// deleteSubnetsInForeignGroup deletes all managed subnets in a foreign resource group
func (f azureReconciler) deleteSubnetsInForeignGroup(ctx context.Context) error {
	if !f.tf.isCreate(Vnet) {
		subnetClient, err := f.factory.Subnet()
		if err != nil {
			return err
		}
		subnets := f.tf.Zones()
		for _, subnet := range subnets {
			resourceGroup := *f.tf.Vnet().ResourceGroup() // safe because we manage a foreign vnet
			err := subnetClient.Delete(ctx, resourceGroup, f.tf.Vnet().Name(), subnet.SubnetName())
			if err != nil {
				return err
			}
		}
		if err != nil {
			return fmt.Errorf("failed to delete foreign subnet: %w", err)
		}
	}
	return nil
}

// Vnet creates or updates a Vnet
func (f azureReconciler) Vnet(ctx context.Context) error {
	if f.tf.isCreate(Vnet) {
		client, err := f.factory.Vnet()
		if err != nil {
			return err
		}
		parameters := armnetwork.VirtualNetwork{
			Location: to.Ptr(f.tf.Region()),
			Properties: &armnetwork.VirtualNetworkPropertiesFormat{
				AddressSpace: &armnetwork.AddressSpace{},
			},
		}

		cidr := f.tf.Vnet().Cidr() // only supports single cidr range
		if cidr != nil {
			parameters.Properties.AddressSpace.AddressPrefixes = []*string{cidr}
		}

		ddosId := f.tf.Vnet().DDosProtectionPlanID()
		if ddosId != nil {
			parameters.Properties.EnableDdosProtection = to.Ptr(true)
			parameters.Properties.DdosProtectionPlan = &armnetwork.SubResource{ID: ddosId}
		}
		return client.CreateOrUpdate(ctx, f.tf.ResourceGroup(), f.tf.Vnet().Name(), parameters)
	} else {
		return nil // TODO update foreign vnet?
	}
}

// RouteTables creates or updates a RouteTable
func (f azureReconciler) RouteTables(ctx context.Context) (armnetwork.RouteTable, error) {
	client, err := f.factory.RouteTables()
	if err != nil {
		return armnetwork.RouteTable{}, err
	}
	parameters := armnetwork.RouteTable{
		Location: to.Ptr(f.tf.Region()),
	}
	resp, err := client.CreateOrUpdate(ctx, f.tf.ResourceGroup(), f.tf.RouteTableName(), parameters)

	return resp.RouteTable, err
}

// SecurityGroups creates or updates a SecurityGroup
func (f azureReconciler) SecurityGroups(ctx context.Context) (*armnetwork.SecurityGroup, error) {
	client, err := f.factory.NetworkSecurityGroup()
	if err != nil {
		return nil, err
	}
	parameters := armnetwork.SecurityGroup{
		Location: to.Ptr(f.tf.Region()),
	}
	resp, err := client.CreateOrUpdate(ctx, f.tf.ResourceGroup(), f.tf.SecurityGroupName(), parameters)
	return resp, err
}

// AvailabilitySet creates or updates an AvailabilitySet
func (f azureReconciler) AvailabilitySet(ctx context.Context) error {
	if f.tf.isCreate(AvailabilitySet) {
		asClient, err := f.factory.AvailabilitySet()
		if err != nil {
			return err
		}
		avset := f.tf.AvailabilitySet()
		parameters := armcompute.AvailabilitySet{
			Location: to.Ptr(f.tf.Region()),
			Properties: &armcompute.AvailabilitySetProperties{
				PlatformFaultDomainCount:  to.Ptr(avset.CountFaultDomains),
				PlatformUpdateDomainCount: to.Ptr(avset.CountUpdateDomains),
			},
			SKU: &armcompute.SKU{Name: to.Ptr(string(armcompute.AvailabilitySetSKUTypesAligned))}, // equal to managed = True in tf
		}
		_, err = asClient.CreateOrUpdate(ctx, f.tf.ResourceGroup(), avset.Name, parameters)
		return err
	} else {
		return nil // TODO update foreign avset?
	}
}

// PublicIPs creates or updates PublicIPs for the NATs
func (f azureReconciler) PublicIPs(ctx context.Context) (map[string][]network.PublicIPAddress, error) {
	res := make(map[string][]network.PublicIPAddress)
	client, err := f.factory.PublicIP()
	if err != nil {
		return res, err
	}
	err = f.deleteOldNatIPs(ctx, client)
	if err != nil {
		return res, err
	}
	ips := f.tf.EnabledNats()
	if len(ips) == 0 {
		return res, nil
	}
	for _, ip := range ips {
		params := network.PublicIPAddress{
			Location: to.Ptr(f.tf.Region()),
			Sku:      &network.PublicIPAddressSku{Name: network.PublicIPAddressSkuNameStandard},
			PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
				PublicIPAllocationMethod: network.Static,
			},
		}
		if ip.Zone() != nil {
			params.Zones = &[]string{*ip.Zone()}
		}
		resp, err := client.CreateOrUpdate(ctx, f.tf.ResourceGroup(), ip.IpName(), params)
		if err != nil {
			return res, err
		}
		res[ip.SubnetName()] = append(res[ip.SubnetName()], *resp)

	}
	return res, nil
}

// EnrichResponseWithUserManagedIPs adds the IDs of user managed IPs to the input map of associated IPs of the NATs
func (f azureReconciler) EnrichResponseWithUserManagedIPs(ctx context.Context, res map[string][]network.PublicIPAddress) error {
	ips := f.tf.UserManagedIPs()
	if len(ips) == 0 {
		return nil
	}
	client, err := f.factory.PublicIP()
	if err != nil {
		return err
	}
	for _, ip := range ips {
		resp, err := client.Get(ctx, ip.ResourceGroup, ip.Name, "")
		if err == nil {
			res[ip.SubnetName] = append(res[ip.SubnetName], network.PublicIPAddress{
				ID: resp.ID,
			})
		} else {
			return err
		}
	}
	return nil
}

func checkAllZonesWithFn(name string, zones []zoneTf, check func(zone zoneTf, name string) bool) bool {
	for _, n := range zones {
		if check(n, name) {
			return true
		}
	}
	return false
}

// NatGateways creates or updates NAT Gateways. It also deletes old NATGateways.
func (f azureReconciler) NatGateways(ctx context.Context, ips map[string][]network.PublicIPAddress) (res map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse, err error) {
	res = make(map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse)
	client, err := f.factory.NatGateway()
	if err != nil {
		return res, err
	}
	err = f.deleteOldNatGateways(ctx, client)
	if err != nil {
		return res, err
	}
	for _, nat := range f.tf.EnabledNats() {
		resp, err := f.createOrUpdateNatGateway(ctx, nat, ips, client)
		if err != nil {
			return res, err
		}
		res[nat.SubnetName()] = resp
	}
	return res, nil
}

func (f azureReconciler) createOrUpdateNatGateway(ctx context.Context, nat zoneTf, ips map[string][]network.PublicIPAddress, client client.NatGateway) (armnetwork.NatGatewaysClientCreateOrUpdateResponse, error) {
	params := armnetwork.NatGateway{
		Properties: &armnetwork.NatGatewayPropertiesFormat{
			IdleTimeoutInMinutes: nat.idleConnectionTimeoutMinutes,
		},
		Location: to.Ptr(f.tf.Region()),
		SKU:      &armnetwork.NatGatewaySKU{Name: to.Ptr(armnetwork.NatGatewaySKUNameStandard)},
	}
	ipResources, ok := ips[nat.SubnetName()]
	if !ok {
		return armnetwork.NatGatewaysClientCreateOrUpdateResponse{}, fmt.Errorf("no public IP found for NAT Gateway %s", nat.NatName())
	} else {
		params.Properties.PublicIPAddresses = []*armnetwork.SubResource{}
		for _, ip := range ipResources {
			params.Properties.PublicIPAddresses = append(params.Properties.PublicIPAddresses, &armnetwork.SubResource{ID: ip.ID})
		}
	}
	if nat.Zone() != nil {
		params.Zones = []*string{nat.Zone()}
	}
	resp, err := client.CreateOrUpdate(ctx, f.tf.ResourceGroup(), nat.NatName(), params)
	if err != nil {
		return armnetwork.NatGatewaysClientCreateOrUpdateResponse{}, err
	}
	return resp, nil
}

// ResourceGroup creates or updates the resource group
func (f azureReconciler) ResourceGroup(ctx context.Context) error {
	rgClient, err := f.factory.Group()
	if err != nil {
		return err
	}
	return rgClient.CreateOrUpdate(ctx, f.tf.ResourceGroup(), f.tf.Region())
}

// delete IPs of NAT Gateways that got disabled
func (f azureReconciler) deleteOldNatIPs(ctx context.Context, client client.PublicIP) error {
	existingIPs, err := client.GetAll(ctx, f.tf.ResourceGroup())
	if err != nil {
		return err
	}
	for _, ip := range existingIPs {
		if ip.Name == nil {
			continue
		}
		isIpInNats := checkAllZonesWithFn(*ip.Name, f.tf.EnabledNats(), func(nat zoneTf, name string) bool { return nat.IpName() == name })
		if !isIpInNats {
			err := client.Delete(ctx, f.tf.ResourceGroup(), *ip.Name)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// delete NAT Gateways that got disabled
func (f azureReconciler) deleteOldNatGateways(ctx context.Context, client client.NatGateway) error {
	existingNats, err := client.GetAll(ctx, f.tf.ResourceGroup())
	if err != nil {
		return err
	}
	for _, nat := range existingNats {
		if nat.Name == nil {
			continue
		}
		isNatInNats := checkAllZonesWithFn(*nat.Name, f.tf.EnabledNats(), func(nat zoneTf, name string) bool { return nat.NatName() == name })
		if !isNatInNats {
			err := client.Delete(ctx, f.tf.ResourceGroup(), *nat.Name)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Subnets creates or updates subnets
func (f azureReconciler) Subnets(ctx context.Context, securityGroup armnetwork.SecurityGroup, routeTable armnetwork.RouteTable, nats map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse) (err error) {
	subnetClient, err := f.factory.Subnet()
	if err != nil {
		return err
	}
	subnets := f.tf.Zones()
	for _, subnet := range subnets {
		endpoints := make([]*armnetwork.ServiceEndpointPropertiesFormat, 0)
		for _, endpoint := range subnet.serviceEndpoints {
			endpoints = append(endpoints, &armnetwork.ServiceEndpointPropertiesFormat{
				Service: to.Ptr(endpoint),
			})
		}

		parameters := armnetwork.Subnet{
			Properties: &armnetwork.SubnetPropertiesFormat{
				AddressPrefix:    to.Ptr(subnet.cidr),
				ServiceEndpoints: endpoints,
				NetworkSecurityGroup: &armnetwork.SecurityGroup{
					ID: securityGroup.ID,
				},
				RouteTable: &armnetwork.RouteTable{
					ID: routeTable.ID,
				},
			},
		}
		nat, ok := nats[subnet.SubnetName()]
		if ok {
			parameters.Properties.NatGateway = &armnetwork.SubResource{
				ID: nat.ID,
			}
		}

		vnetRgroup := f.tf.Vnet().ResourceGroup() // try to use existing vnet resource
		if vnetRgroup == nil {
			vnetRgroup = to.Ptr(f.tf.ResourceGroup()) // expect that it was created previously
		}
		err = subnetClient.CreateOrUpdate(ctx, *vnetRgroup, f.tf.Vnet().Name(), subnet.SubnetName(), parameters)
	}
	return err
}
