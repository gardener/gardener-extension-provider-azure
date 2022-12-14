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

// AzureReconciler allows to reconcile the individual cloud resources based on the Terraform configuration logic
type AzureReconciler struct {
	tf      TerraformAdapter
	factory client.Factory
}

// NewAzureReconciler creates a new TfReconciler
func NewAzureReconciler(infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig, cluster *controller.Cluster, factory client.Factory) (*AzureReconciler, error) {
	tfAdapter, err := NewTerraformAdapter(infra, cfg, cluster)
	return &AzureReconciler{tfAdapter, factory}, err
}

// GetInfrastructureStatus returns the infrastructure status
func (f AzureReconciler) GetInfrastructureStatus(ctx context.Context, cfg *azure.InfrastructureConfig) (*v1alpha1.InfrastructureStatus, error) {
	status := f.tf.StaticInfrastructureStatus(cfg)
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

func (f AzureReconciler) enrichStatusWithAvailabilitySet(ctx context.Context, status *v1alpha1.InfrastructureStatus) error {
	if f.tf.isCreate(TfAvailabilitySet) {
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

func (f AzureReconciler) enrichStatusWithIdentity(ctx context.Context, status *v1alpha1.InfrastructureStatus) error {
	client, err := f.factory.ManagedUserIdentity()
	if err != nil {
		return err
	}
	if identity := f.tf.Identity(); identity != nil {
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
func (f AzureReconciler) Delete(ctx context.Context) error {
	client, err := f.factory.Group()
	if err != nil {
		return err
	}
	// delete associated resources from other RG (otherwise deletion blocks)
	if !f.tf.isCreate(TfVnet) {
		err := f.deleteForeignSubnets(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete foreign subnet: %w", err)
		}
	}
	return client.DeleteIfExists(ctx, f.tf.ResourceGroup())
}

func (f AzureReconciler) deleteForeignSubnets(ctx context.Context) error {
	subnetClient, err := f.factory.Subnet()
	if err != nil {
		return err
	}
	subnets := f.tf.Zones()
	for _, subnet := range subnets {
		err := subnetClient.Delete(ctx, *f.tf.Vnet().ResourceGroup(), f.tf.Vnet().Name(), subnet.SubnetName())
		if err != nil {
			return err
		}
	}
	return nil
}

// Vnet creates or updates a Vnet
func (f AzureReconciler) Vnet(ctx context.Context) error {
	if f.tf.isCreate(TfVnet) {
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

		cidr, ok := f.tf.Vnet()["cidr"]
		if ok {
			parameters.Properties.AddressSpace.AddressPrefixes = []*string{to.Ptr(cidr.(string))}
		}

		ddosId, ok := f.tf.Vnet()["ddosProtectionPlanID"]
		if ok {
			ddosIdString := ddosId.(string)
			parameters.Properties.EnableDdosProtection = to.Ptr(true)
			parameters.Properties.DdosProtectionPlan = &armnetwork.SubResource{ID: to.Ptr(ddosIdString)}
		}

		rgroup := f.tf.ResourceGroup()
		vnet := f.tf.Vnet()["name"].(string)
		return client.CreateOrUpdate(ctx, rgroup, vnet, parameters)
	} else {
		return nil
	}
}

// RouteTables creates or updates a RouteTable
func (f AzureReconciler) RouteTables(ctx context.Context) (armnetwork.RouteTable, error) {
	client, err := f.factory.RouteTables()
	if err != nil {
		return armnetwork.RouteTable{}, err
	}
	routeTableName := f.tf.RouteTableName()
	parameters := armnetwork.RouteTable{
		Location: to.Ptr(f.tf.Region()),
	}
	resp, err := client.CreateOrUpdate(ctx, f.tf.ResourceGroup(), routeTableName, parameters)

	return resp.RouteTable, err
}

// SecurityGroups creates or updates a SecurityGroup
func (f AzureReconciler) SecurityGroups(ctx context.Context) (*network.SecurityGroup, error) {
	client, err := f.factory.NetworkSecurityGroup()
	if err != nil {
		return nil, err
	}
	name := f.tf.SecurityGroupName()
	parameters := network.SecurityGroup{
		Location: to.Ptr(f.tf.Region()),
	}
	resp, err := client.CreateOrUpdate(ctx, f.tf.ResourceGroup(), name, parameters)
	return resp, err
}

// AvailabilitySet creates or updates an AvailabilitySet
func (f AzureReconciler) AvailabilitySet(ctx context.Context) error {
	if f.tf.isCreate(TfAvailabilitySet) {
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
		return nil
	}
}

// PublicIPs creates or updates PublicIPs for the NATs
func (f AzureReconciler) PublicIPs(ctx context.Context) (map[string][]network.PublicIPAddress, error) {
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
func (f AzureReconciler) EnrichResponseWithUserManagedIPs(ctx context.Context, res map[string][]network.PublicIPAddress) error {
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
func (f AzureReconciler) NatGateways(ctx context.Context, ips map[string][]network.PublicIPAddress) (res map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse, err error) {
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
		params := armnetwork.NatGateway{

			Properties: &armnetwork.NatGatewayPropertiesFormat{
				IdleTimeoutInMinutes: nat.idleConnectionTimeoutMinutes,
			},
			Location: to.Ptr(f.tf.Region()),
			SKU:      &armnetwork.NatGatewaySKU{Name: to.Ptr(armnetwork.NatGatewaySKUNameStandard)},
		}
		ipResources, ok := ips[nat.SubnetName()] // TODO should fail if not found
		if ok {
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
			//continue // TODO skip or return?
			return res, err
		}
		res[nat.SubnetName()] = resp
	}
	return res, nil
}

// ResourceGroup creates or updates the resource group
func (f AzureReconciler) ResourceGroup(ctx context.Context) error {
	rgClient, err := f.factory.Group()
	if err != nil {
		return err
	}
	return rgClient.CreateOrUpdate(ctx, f.tf.ResourceGroup(), f.tf.Region())
}

func (f AzureReconciler) deleteOldNatIPs(ctx context.Context, client client.PublicIP) error {
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

func (f AzureReconciler) deleteOldNatGateways(ctx context.Context, client client.NatGateway) error {
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
func (f AzureReconciler) Subnets(ctx context.Context, securityGroup armnetwork.SecurityGroup, routeTable armnetwork.RouteTable, nats map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse) (err error) {
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
