package infraflow

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

type TfReconciler struct {
	tf      TerraformAdapter
	factory client.NewFactory
}

func NewTfReconciler(infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig, cluster *controller.Cluster, factory client.NewFactory) (*TfReconciler, error) {
	tfAdapter, err := NewTerraformAdapter(infra, cfg, cluster)
	return &TfReconciler{tfAdapter, factory}, err
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
		parameters := armcompute.AvailabilitySet{
			Location: to.Ptr(f.tf.Region()),
			Properties: &armcompute.AvailabilitySetProperties{
				PlatformFaultDomainCount:  to.Ptr(f.tf.CountFaultDomains()),
				PlatformUpdateDomainCount: to.Ptr(f.tf.CountUpdateDomains()),
			},
			SKU: &armcompute.SKU{Name: to.Ptr(string(armcompute.AvailabilitySetSKUTypesAligned))}, // equal to managed = True in tf
		}
		_, err = asClient.CreateOrUpdate(ctx, f.tf.ResourceGroup(), f.tf.AvailabilitySetName(), parameters)
		return err
	} else {
		return nil
	}
}

func (f TfReconciler) PublicIPs(ctx context.Context) (map[string]armnetwork.PublicIPAddressesClientCreateOrUpdateResponse, error) {
	res := make(map[string]armnetwork.PublicIPAddressesClientCreateOrUpdateResponse)
	client, err := f.factory.PublicIP()
	if err != nil {
		return res, err
	}
	for _, ip := range f.tf.IPs() {
		resp, err := client.CreateOrUpdate(ctx, f.tf.ResourceGroup(), ip.name, armnetwork.PublicIPAddress{
			Location: to.Ptr(f.tf.Region()),
			SKU:      &armnetwork.PublicIPAddressSKU{Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard)},
			Properties: &armnetwork.PublicIPAddressPropertiesFormat{
				PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
			},
			// TODO zones prop?
		})
		if err != nil {
			return res, err
		}
		res[ip.subnetName] = resp
	}
	return res, nil
}

func (f TfReconciler) NatGateways(ctx context.Context, ips map[string]armnetwork.PublicIPAddressesClientCreateOrUpdateResponse) (res map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse, err error) {
	res = make(map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse)
	client, err := f.factory.NatGateway()
	if err != nil {
		return res, err
	}
	nats := f.tf.Nats()
	for _, nat := range nats {
		if !nat.enabled {
			continue
		}
		resp, err := client.CreateOrUpdate(ctx, f.tf.ResourceGroup(), nat.name, armnetwork.NatGateway{
			Properties: &armnetwork.NatGatewayPropertiesFormat{
				PublicIPAddresses: []*armnetwork.SubResource{{ID: ips[nat.subnetName].ID}},
			},
		})
		if err != nil {
			return res, err
		}
		res[nat.subnetName] = resp
	}
	return res, nil
}
