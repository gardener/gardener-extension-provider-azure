package infraflow_test

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	mockclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client/mock"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newBasicConfig() *azure.InfrastructureConfig {
	return &azure.InfrastructureConfig{
		//ResourceGroup: &azure.ResourceGroup{Name: resourceGroupName},
		Networks: azure.NetworkConfig{
			VNet: azure.VNet{
				//Name:          to.Ptr(vnetName), // only specify when using existing group
				//ResourceGroup: to.Ptr(resourceGroupName),
				CIDR: to.Ptr("10.0.0.0/8"),
			},
			Workers:          to.Ptr("10.0.0.0/16"),
			ServiceEndpoints: []string{},
			/// TODO how to specify multi subnet.. resource group not needed?
			//Zones:            []azure.Zone{{Name: 1, CIDR: "10.0.0.0/16", NatGateway: &azure.ZonedNatGatewayConfig{Enabled: true, IPAddresses: []azure.ZonedPublicIPReference{{Name: "my-ip", ResourceGroup: resourceGroupName}}}}, {Name: 2, CIDR: "10.1.0.0/16"}}, // subnets
		},
	}
}

// will also work for new Reonciler
var _ = Describe("TfReconciler", func() {
	location := "westeurope"
	clusterName := "test_cluster"
	infra := &v1alpha1.Infrastructure{Spec: v1alpha1.InfrastructureSpec{Region: location}, ObjectMeta: metav1.ObjectMeta{Namespace: clusterName}}
	resourceGroupName := infra.Namespace //if not specified this is assumed name "t-i545428" // TODO what if resource group not given? by default Tf uses infra.Namespace
	vnetName := infra.Namespace          //if not specified this is assumed name "vnet-i545428"
	cluster := infrastructure.MakeCluster("11.0.0.0/16", "12.0.0.0/16", infra.Spec.Region, 1, 1)
	var factory *mockclient.MockNewFactory
	Describe("Vnet reconcilation", func() {
		Context("new vnet", func() {
			cfg := newBasicConfig()
			It("calls the client with the correct parameters: vnet name, resource group, region ,cidr", func() {
				mock := NewMockFactoryWrapper(resourceGroupName, location)
				parameters := armnetwork.VirtualNetwork{
					Location: to.Ptr(location),
					Properties: &armnetwork.VirtualNetworkPropertiesFormat{
						AddressSpace: &armnetwork.AddressSpace{
							AddressPrefixes: []*string{cfg.Networks.VNet.CIDR},
						},
					},
				}
				mock.assertVnetCalledWithParameters(vnetName, parameters)
				factory = mock.GetFactory()

				sut, err := infraflow.NewTfReconciler(infra, cfg, cluster, factory)
				Expect(err).ToNot(HaveOccurred())
				sut.Vnet(context.TODO())
			})
			Context("with ddosId", func() {
				ddosId := "ddos-plan-id"
				cfg := newBasicConfig()
				cfg.Networks.VNet.DDosProtectionPlanID = to.Ptr(ddosId)
				It("calls the client with the correct parameters: vnet name, resource group, region ,cidr, ddos id", func() {
					mock := NewMockFactoryWrapper(resourceGroupName, location)
					parameters := armnetwork.VirtualNetwork{
						Location: to.Ptr(location),
						Properties: &armnetwork.VirtualNetworkPropertiesFormat{
							AddressSpace: &armnetwork.AddressSpace{
								AddressPrefixes: []*string{cfg.Networks.VNet.CIDR},
							},
						},
					}
					parameters.Properties.DdosProtectionPlan = &armnetwork.SubResource{ID: to.Ptr(ddosId)}
					parameters.Properties.EnableDdosProtection = to.Ptr(true)
					mock.assertVnetCalledWithParameters(vnetName, parameters)
					factory = mock.GetFactory()

					sut, err := infraflow.NewTfReconciler(infra, cfg, cluster, factory)
					Expect(err).ToNot(HaveOccurred())
					sut.Vnet(context.TODO())
				})

			})
		})
		Context("with existing vnet", func() {
			cfg := newBasicConfig()
			cfg.Networks.VNet.Name = to.Ptr("existing-vnet")
			cfg.Networks.VNet.ResourceGroup = to.Ptr("existing-rg")
			It("does not reconcile", func() {
				mock := NewMockFactoryWrapper(resourceGroupName, location)
				factory = mock.GetFactory()

				sut, err := infraflow.NewTfReconciler(infra, cfg, cluster, factory)
				Expect(err).ToNot(HaveOccurred())
				sut.Vnet(context.TODO())
			})
		})
	})
	Describe("Route table reconcilation", func() {
		cfg := newBasicConfig()
		It("calls the client with correct route table name", func() {
			mock := NewMockFactoryWrapper(resourceGroupName, location)
			//parameters := armnetwork.RouteTable{
			//	Location:   to.Ptr(location),
			//	Properties: &armnetwork.RouteTablePropertiesFormat{
			//		//AddressSpace: &armnetwork.AddressSpace{
			//		//	AddressPrefixes: []*string{cfg.Networks.VNet.CIDR},
			//		//},
			//	},
			//}
			mock.assertRouteTableCalled("worker_route_table")
			factory = mock.GetFactory()

			sut, err := infraflow.NewTfReconciler(infra, cfg, cluster, factory)
			Expect(err).ToNot(HaveOccurred())
			sut.RouteTables(context.TODO())
		})
	})
	Describe("Security group reconcilation", func() {
		cfg := newBasicConfig()
		It("calls the client with correct route table name", func() {
			mock := NewMockFactoryWrapper(resourceGroupName, location)
			//parameters := armnetwork.RouteTable{
			//	Location:   to.Ptr(location),
			//	Properties: &armnetwork.RouteTablePropertiesFormat{
			//		//AddressSpace: &armnetwork.AddressSpace{
			//		//	AddressPrefixes: []*string{cfg.Networks.VNet.CIDR},
			//		//},
			//	},
			//}
			mock.assertSecurityGroupCalled("test_cluster-workers")
			factory = mock.GetFactory()

			sut, err := infraflow.NewTfReconciler(infra, cfg, cluster, factory)
			Expect(err).ToNot(HaveOccurred())
			sut.SecurityGroups(context.TODO())
		})
	})
	Describe("Availability set reconcilation", func() {
		cfg := newBasicConfig()
		It("calls the client with correct availability set name and parameters", func() {
			mock := NewMockFactoryWrapper(resourceGroupName, location)
			parameters := armcompute.AvailabilitySet{
				Location: to.Ptr(location),
				Properties: &armcompute.AvailabilitySetProperties{
					PlatformFaultDomainCount:  to.Ptr(int32(1)),
					PlatformUpdateDomainCount: to.Ptr(int32(1)),
				},
				SKU: &armcompute.SKU{Name: to.Ptr(string(armcompute.AvailabilitySetSKUTypesAligned))}, // equal to managed = True in tf
			}
			mock.assertAvailabilitySetCalledWithParameters("test_cluster-workers", parameters)
			factory = mock.GetFactory()

			sut, err := infraflow.NewTfReconciler(infra, cfg, cluster, factory)
			Expect(err).ToNot(HaveOccurred())
			sut.AvailabilitySet(context.TODO())
		})
	})

})

type MatchParameters (armnetwork.VirtualNetwork)

func (m MatchParameters) Matches(x interface{}) bool {
	bytes, _ := armnetwork.VirtualNetwork(m).MarshalJSON()
	Otherbytes, _ := x.(armnetwork.VirtualNetwork).MarshalJSON()
	println(string(bytes))
	println(string(Otherbytes))
	return string(bytes) == string(Otherbytes)
}

func (m MatchParameters) String() string {
	bytes, _ := armnetwork.VirtualNetwork(m).MarshalJSON()
	return string(bytes)
}
