package infraflow

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"k8s.io/utils/pointer"
)

type TfReconciler struct {
	tf      TerraformAdapter
	factory client.NewFactory
}

func NewTfReconciler(infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig, cluster *controller.Cluster, factory client.NewFactory) (*TfReconciler, error) {
	tfAdapter, err := NewTerraformAdapter(infra, cfg, cluster)
	return &TfReconciler{tfAdapter, factory}, err
}

func (f TfReconciler) GetInfrastructureStatus(ctx context.Context, cfg *azure.InfrastructureConfig) (*v1alpha1.InfrastructureStatus, error) {
	status := f.tf.StaticInfrastructureStatus(cfg)
	// enrich with Identity
	client, err := f.factory.ManagedUserIdentity()
	if err != nil {
		return nil, err
	}
	if identity := f.tf.Identity(); identity != nil {
		res, err := client.Get(ctx, identity.ResourceGroup, identity.Name)
		if err != nil {
			return nil, err
		}
		if res.ID == nil || res.ClientID == nil {
			return status, nil
		}

		status.Identity = &v1alpha1.IdentityStatus{
			ID:       *res.ID,
			ClientID: res.ClientID.String(),
		}
	}
	// enrich with AvailabilitySet
	if f.tf.isCreate(TfAvailabilitySet) {
		client, err := f.factory.AvailabilitySet()
		if err != nil {
			return nil, err
		}
		avset := f.tf.AvailabilitySet()
		res, err := client.Get(ctx, f.tf.ResourceGroup(), avset.Name)
		if err != nil {
			return nil, err
		}
		status.AvailabilitySets = append(status.AvailabilitySets, v1alpha1.AvailabilitySet{
			Name:               avset.Name,
			ID:                 *res.ID,
			CountFaultDomains:  pointer.Int32Ptr(avset.CountFaultDomains),
			CountUpdateDomains: pointer.Int32Ptr(avset.CountUpdateDomains),
			Purpose:            v1alpha1.PurposeNodes,
		})
	}
	return status, nil
}

func (f TfReconciler) Delete(ctx context.Context) error {
	client, err := f.factory.ResourceGroup()
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
	return client.Delete(ctx, f.tf.ResourceGroup())
}

func (f TfReconciler) deleteForeignSubnets(ctx context.Context) error {
	subnetClient, err := f.factory.Subnet()
	if err != nil {
		return err
	}
	subnets := f.tf.Subnets()
	for _, subnet := range subnets {
		err := subnetClient.Delete(ctx, *f.tf.Vnet().ResourceGroup(), f.tf.Vnet().Name(), subnet.name)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f TfReconciler) Vnet(ctx context.Context) error {
	if f.tf.isCreate(TfVnet) {
		client, err := f.factory.Vnet()
		if err != nil {
			return err
		}
		return ReconcileVnetFromTf(ctx, f.tf, client)
	} else {
		return nil
	}
}

func (f TfReconciler) RouteTables(ctx context.Context) (armnetwork.RouteTable, error) {
	client, err := f.factory.RouteTables()
	if err != nil {
		return armnetwork.RouteTable{}, err
	}
	return ReconcileRouteTablesFromTf(f.tf, client, ctx)
}

func (f TfReconciler) SecurityGroups(ctx context.Context) (armnetwork.SecurityGroupsClientCreateOrUpdateResponse, error) {
	client, err := f.factory.SecurityGroups()
	if err != nil {
		return armnetwork.SecurityGroupsClientCreateOrUpdateResponse{}, err
	}
	return ReconcileSecurityGroupsFromTf(f.tf, client, ctx)
}

func (f TfReconciler) AvailabilitySet(ctx context.Context) error {
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

func (f TfReconciler) PublicIPs(ctx context.Context) (map[string]armnetwork.PublicIPAddress, error) {
	res := make(map[string]armnetwork.PublicIPAddress)
	client, err := f.factory.PublicIP()
	if err != nil {
		return res, err
	}
	err = f.deleteOldNatIPs(client, ctx)
	if err != nil {
		return res, err
	}
	ips := f.tf.EnabledNats()
	if len(ips) == 0 {
		return res, nil
	}
	for _, ip := range ips {
		params := armnetwork.PublicIPAddress{
			Location: to.Ptr(f.tf.Region()),
			SKU:      &armnetwork.PublicIPAddressSKU{Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard)},
			Properties: &armnetwork.PublicIPAddressPropertiesFormat{
				PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
			},
		}
		if ip.Zone() != nil {
			params.Zones = []*string{ip.Zone()}
		}
		resp, err := client.CreateOrUpdate(ctx, f.tf.ResourceGroup(), ip.IpName(), params)
		if err != nil {
			return res, err
		}
		res[ip.SubnetName()] = resp.PublicIPAddress
	}
	return res, nil
}

func (f TfReconciler) EnrichResponseWithUserManagedIPs(ctx context.Context, res map[string]armnetwork.PublicIPAddress) error {
	ips := f.tf.UserManagedIPs()
	if len(ips) == 0 {
		return nil
	}
	client, err := f.factory.PublicIP()
	if err != nil {
		return err
	}
	for _, ip := range ips {
		resp, err := client.Get(ctx, ip.ResourceGroup, ip.Name)
		if err == nil {
			res[ip.SubnetName] = armnetwork.PublicIPAddress{
				ID: resp.ID,
			}
		} else {
			return err
		}
	}
	return nil
}

func checkAllNatsWithFn(name string, nats []natTf, check func(nat natTf, name string) bool) bool {
	for _, n := range nats {
		if check(n, name) {
			return true
		}
	}
	return false
}

func (f TfReconciler) NatGateways(ctx context.Context, ips map[string]armnetwork.PublicIPAddress) (res map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse, err error) {
	res = make(map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse)
	client, err := f.factory.NatGateway()
	if err != nil {
		return res, err
	}
	err = f.deleteOldNatGateways(client, ctx)
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
		ipResource, ok := ips[nat.SubnetName()] // TODO should fail if not found
		if ok {
			params.Properties.PublicIPAddresses = []*armnetwork.SubResource{{ID: ipResource.ID}}
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

func (f TfReconciler) ResourceGroup(ctx context.Context) error {
	rgClient, err := f.factory.ResourceGroup()
	if err != nil {
		return err
	}
	return rgClient.CreateOrUpdate(ctx, f.tf.ResourceGroup(), f.tf.Region())
}

func (f TfReconciler) deleteOldNatIPs(client client.NewPublicIP, ctx context.Context) error {
	existingIPs, err := client.GetAll(ctx, f.tf.ResourceGroup())
	if err != nil {
		return err
	}
	for _, ip := range existingIPs {
		if ip.Name == nil {
			continue
		}
		isIpInNats := checkAllNatsWithFn(*ip.Name, f.tf.EnabledNats(), func(nat natTf, name string) bool { return nat.IpName() == name })
		if !isIpInNats {
			err := client.Delete(ctx, f.tf.ResourceGroup(), *ip.Name)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (f TfReconciler) deleteOldNatGateways(client client.NatGateway, ctx context.Context) error {
	existingNats, err := client.GetAll(ctx, f.tf.ResourceGroup())
	if err != nil {
		return err
	}
	for _, nat := range existingNats {
		if nat.Name == nil {
			continue
		}
		isNatInNats := checkAllNatsWithFn(*nat.Name, f.tf.EnabledNats(), func(nat natTf, name string) bool { return nat.NatName() == name })
		if !isNatInNats {
			err := client.Delete(ctx, f.tf.ResourceGroup(), *nat.Name)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (f TfReconciler) Subnets(ctx context.Context, securityGroup armnetwork.SecurityGroup, routeTable armnetwork.RouteTable, nats map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse) (err error) {
	subnetClient, err := f.factory.Subnet()
	if err != nil {
		return err
	}
	subnets := f.tf.Subnets()
	for _, subnet := range subnets {
		endpoints := make([]*armnetwork.ServiceEndpointPropertiesFormat, 0)
		for _, endpoint := range subnet.serviceEndpoints {
			endpoints = append(endpoints, &armnetwork.ServiceEndpointPropertiesFormat{
				Service: to.Ptr(endpoint),
			})
		}

		parameters := armnetwork.Subnet{
			//Name: to.Ptr(subnet.name),
			Properties: &armnetwork.SubnetPropertiesFormat{
				AddressPrefix:    to.Ptr(subnet.cidr),
				ServiceEndpoints: endpoints, // TODO associate security group?, route table?
				NetworkSecurityGroup: &armnetwork.SecurityGroup{
					ID: securityGroup.ID,
				},
				RouteTable: &armnetwork.RouteTable{
					ID: routeTable.ID,
				},
			},
		}
		nat, ok := nats[subnet.name]
		if ok {
			parameters.Properties.NatGateway = &armnetwork.SubResource{
				ID: nat.ID,
			}
		}

		vnetRgroup := f.tf.Vnet().ResourceGroup() // try to use existing vnet resource
		if vnetRgroup == nil {
			vnetRgroup = to.Ptr(f.tf.ResourceGroup()) // expect that it was created previously
		}
		err = subnetClient.CreateOrUpdate(ctx, *vnetRgroup, f.tf.Vnet().Name(), subnet.name, parameters)
	}
	return err
}
