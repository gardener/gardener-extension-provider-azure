package infraflow_test

import (
	"context"
	"encoding/base64"
	"flag"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	mockclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client/mock"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	secretYamlPath = flag.String("secret-path", "", "Yaml file with secret including Azure credentials")
)

func newBasicConfig() *azure.InfrastructureConfig {
	return &azure.InfrastructureConfig{
		Networks: azure.NetworkConfig{
			VNet: azure.VNet{
				CIDR: to.Ptr("10.0.0.0/8"),
			},
			Workers:          to.Ptr("10.0.0.0/16"),
			ServiceEndpoints: []string{},
		},
	}

}

var _ = BeforeSuite(func() {
	flag.Parse()
})

// will also work for new Reconciler
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
				err = sut.Vnet(context.TODO())
				Expect(err).ToNot(HaveOccurred())
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
					err = sut.Vnet(context.TODO())
					Expect(err).ToNot(HaveOccurred())
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
				err = sut.Vnet(context.TODO())
				Expect(err).ToNot(HaveOccurred())

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
			Expect(sut.RouteTables(context.TODO())).To(Succeed())
		})
	})
	Describe("Security group reconcilation", func() {
		cfg := newBasicConfig()
		It("calls the client with correct route table name", func() {
			mock := NewMockFactoryWrapper(resourceGroupName, location)

			mock.assertSecurityGroupCalled(clusterName + "-workers")
			factory = mock.GetFactory()

			sut, err := infraflow.NewTfReconciler(infra, cfg, cluster, factory)
			Expect(err).ToNot(HaveOccurred())
			_, err = sut.SecurityGroups(context.TODO())
			Expect(err).ToNot(HaveOccurred())

		})
	})
	Describe("Availability set reconcilation", func() {
		Context("zoned cluster", func() {
			cfg := newBasicConfig() // cannot share varible in describe
			cfg.Zoned = true
			It("does not create availability set", func() {
				mock := NewMockFactoryWrapper(resourceGroupName, location)
				factory = mock.GetFactory()
				sut, err := infraflow.NewTfReconciler(infra, cfg, cluster, factory)
				Expect(err).ToNot(HaveOccurred())
				err = sut.AvailabilitySet(context.TODO())
				Expect(err).ToNot(HaveOccurred())
			})
		})
		Context("non-zoned cluster", func() {
			cfg := newBasicConfig()
			cfg.Zoned = false
			It("create the client with correct availability set name and parameters", func() {
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
				err = sut.AvailabilitySet(context.TODO())
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
	Describe("PublicIP reconcilation", func() {
		Context("with 2 zones, no NAT enabled and user managed IP", func() {
			cfg := newBasicConfig()
			cfg.Networks.NatGateway = &azure.NatGatewayConfig{
				Zone:    to.Ptr(int32(1)),
				Enabled: false,
			}
			cfg.Networks.Zones = []azure.Zone{{Name: 1, CIDR: "10.0.0.0/16", NatGateway: &azure.ZonedNatGatewayConfig{Enabled: false, IPAddresses: []azure.ZonedPublicIPReference{{Name: "my-ip", ResourceGroup: resourceGroupName}}}}, {Name: 2, CIDR: "10.1.0.0/16"}}
			BeforeEach(func() {
				mock := NewMockFactoryWrapper(resourceGroupName, location)
				mock.assertPublicIPCalledWithoutCreation()
				factory = mock.GetFactory()
			})
			It("does not create NAT IPs and does not update user-managed public IPs", func() {
				sut, err := infraflow.NewTfReconciler(infra, cfg, cluster, factory)
				Expect(err).ToNot(HaveOccurred())
				_, err = sut.PublicIPs(context.TODO())
				Expect(err).ToNot(HaveOccurred())
			})
		})
		Context("with 2 zones, 1 NAT enabled and user managed IP", func() {
			cfg := newBasicConfig()
			cfg.Networks.Zones = []azure.Zone{{Name: 1, CIDR: "10.0.0.0/16", NatGateway: &azure.ZonedNatGatewayConfig{Enabled: true, IPAddresses: []azure.ZonedPublicIPReference{{Name: "my-ip", ResourceGroup: resourceGroupName}}}}, {Name: 2, CIDR: "10.1.0.0/16"}}
			BeforeEach(func() {
				mock := NewMockFactoryWrapper(resourceGroupName, location)
				parameters := armnetwork.PublicIPAddress{
					Location: to.Ptr(location),
					SKU:      &armnetwork.PublicIPAddressSKU{Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard)},
					Properties: &armnetwork.PublicIPAddressPropertiesFormat{
						PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
					},
					Zones: []*string{to.Ptr("1")},
				}
				mock.assertPublicIPCalledWithParameters(MatchAnyOfStrings([]string{"test_cluster-nat-gateway-z1-ip"}), parameters)
				factory = mock.GetFactory()
			})
			It("only creates NAT IP for 1 zone and does not update user-managed public IPs", func() {
				sut, err := infraflow.NewTfReconciler(infra, cfg, cluster, factory)
				Expect(err).ToNot(HaveOccurred())
				_, err = sut.PublicIPs(context.TODO())
				Expect(err).ToNot(HaveOccurred())

			})
		})
		Context("single zoned with NAT enabled", func() {
			cfg := newBasicConfig()
			cfg.Networks.NatGateway = &azure.NatGatewayConfig{
				Zone: to.Ptr(int32(1)),
			}
			BeforeEach(func() {
				mock := NewMockFactoryWrapper(resourceGroupName, location)
				parameters := armnetwork.PublicIPAddress{
					Location: to.Ptr(location),
					SKU:      &armnetwork.PublicIPAddressSKU{Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard)},
					Properties: &armnetwork.PublicIPAddressPropertiesFormat{
						PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
					},
				}
				mock.assertPublicIPCalledWithParameters(MatchAnyOfStrings([]string{"test_cluster-nat-gateway-ip"}), parameters)
				factory = mock.GetFactory()
			})
		})
		Context("with single net and NAT, then disabled (NOT MOCKED)", func() { // TODO replace with mock?
			cfg := newBasicConfig()
			cfg.Networks.NatGateway = &azure.NatGatewayConfig{
				Zone:    to.Ptr(int32(1)),
				Enabled: true,
			}
			It("should delete the old NAT associated IP after reconcilation", func() {
				auth := readAuthFromFile(*secretYamlPath)
				newFactory, err := client.NewAzureClientFactoryV2(auth)
				Expect(err).ToNot(HaveOccurred())

				sut, err := infraflow.NewTfReconciler(infra, cfg, cluster, newFactory)
				Expect(err).ToNot(HaveOccurred())
				Expect(sut.ResourceGroup(context.TODO())).To(Succeed())
				rgroupC, err := newFactory.ResourceGroup()
				Expect(err).ToNot(HaveOccurred())
				defer Expect(rgroupC.Delete(context.TODO(), resourceGroupName)).To(Succeed())

				_, err = sut.PublicIPs(context.TODO())
				Expect(err).ToNot(HaveOccurred())

				cfg.Networks.NatGateway.Enabled = false
				// new init due to
				sut, err = infraflow.NewTfReconciler(infra, cfg, cluster, newFactory)
				Expect(err).ToNot(HaveOccurred())

				_, err = sut.PublicIPs(context.TODO())
				Expect(err).ToNot(HaveOccurred())

				client, err := newFactory.PublicIP()
				Expect(err).ToNot(HaveOccurred())
				nats, err := client.GetAll(context.TODO(), resourceGroupName)
				Expect(err).ToNot(HaveOccurred())
				Expect(nats).To(BeEmpty())
			})
		})
		Describe("Nat gateway reconcilation", func() {
			cfg := newBasicConfig()
			Context("with 2 zones and 1 with NAT", func() {
				cfg.Networks.Zones = []azure.Zone{{Name: 1, CIDR: "10.0.0.0/16", NatGateway: &azure.ZonedNatGatewayConfig{Enabled: true, IPAddresses: []azure.ZonedPublicIPReference{{Name: "my-ip", ResourceGroup: resourceGroupName}}}}, {Name: 2, CIDR: "10.1.0.0/16"}}
				ipId := to.Ptr("ip-id")
				It("calls the client with correct nat gateway name and parameters", func() {
					mock := NewMockFactoryWrapper(resourceGroupName, location)
					parameters := armnetwork.NatGateway{
						Location: to.Ptr(location),
						Properties: &armnetwork.NatGatewayPropertiesFormat{
							PublicIPAddresses: []*armnetwork.SubResource{
								{
									ID: ipId,
								},
							},
							IdleTimeoutInMinutes: nil,
						},
						SKU:   &armnetwork.NatGatewaySKU{Name: to.Ptr(armnetwork.NatGatewaySKUNameStandard)},
						Zones: []*string{to.Ptr("1")},
					}
					mock.assertNatGatewayCalledWithParameters("test_cluster-nat-gateway-z1", parameters)
					factory = mock.GetFactory()

					sut, err := infraflow.NewTfReconciler(infra, cfg, cluster, factory)
					Expect(err).ToNot(HaveOccurred())
					_, err = sut.NatGateways(context.TODO(), map[string][]armnetwork.PublicIPAddress{"test_cluster-nodes-z1": {{
						ID: ipId,
					},
					}})
					Expect(err).ToNot(HaveOccurred())
				})
			})
			Context("with single net and NAT, then disabled (NOT MOCKED)", func() { // TODO replace with mock
				cfg := newBasicConfig()
				cfg.Networks.NatGateway = &azure.NatGatewayConfig{
					Zone:    to.Ptr(int32(1)),
					Enabled: true,
				}
				//ipId := to.Ptr("ip-id")
				It("deletes the old NAT after reconcilation", func() {
					auth := readAuthFromFile(*secretYamlPath)
					newFactory, err := client.NewAzureClientFactoryV2(auth)
					Expect(err).ToNot(HaveOccurred())

					sut, err := infraflow.NewTfReconciler(infra, cfg, cluster, newFactory)
					Expect(err).ToNot(HaveOccurred())
					Expect(sut.ResourceGroup(context.TODO())).To(Succeed())
					rgroupC, err := newFactory.ResourceGroup()
					Expect(err).ToNot(HaveOccurred())
					defer func() {
						Expect(rgroupC.Delete(context.TODO(), resourceGroupName)).To(Succeed())
					}()

					_, err = sut.NatGateways(context.TODO(), map[string][]armnetwork.PublicIPAddress{})
					Expect(err).ToNot(HaveOccurred())

					cfg.Networks.NatGateway.Enabled = false
					// new init due to
					sut, err = infraflow.NewTfReconciler(infra, cfg, cluster, newFactory)
					Expect(err).ToNot(HaveOccurred())
					_, err = sut.NatGateways(context.TODO(), map[string][]armnetwork.PublicIPAddress{})
					Expect(err).ToNot(HaveOccurred())

					client, err := newFactory.NatGateway()
					Expect(err).ToNot(HaveOccurred())
					nats, err := client.GetAll(context.TODO(), resourceGroupName)
					Expect(err).ToNot(HaveOccurred())
					Expect(nats).To(BeEmpty())
				})
			})
		})
		Describe("Subnet reconcilation", func() {
			cfg := newBasicConfig()
			Context("with 2 zones", func() {
				cfg.Networks.Zones = []azure.Zone{{Name: 1, CIDR: "10.0.0.0/16", NatGateway: &azure.ZonedNatGatewayConfig{Enabled: true, IPAddresses: []azure.ZonedPublicIPReference{{Name: "my-ip", ResourceGroup: resourceGroupName}}}}, {Name: 2, CIDR: "10.1.0.0/16"}}
				It("calls the client with correct nat gateway name and parameters", func() {
					mock := NewMockFactoryWrapper(resourceGroupName, location)
					//parameters := armnetwork.NatGateway{
					//	Location: to.Ptr(location),
					//	Properties: &armnetwork.NatGatewayPropertiesFormat{
					//		PublicIPAddresses: []*armnetwork.SubResource{
					//			{
					//				ID: to.Ptr("/subscriptions/123/resourceGroups/test_rg/providers/Microsoft.Network/publicIPAddresses/test_cluster-nat-gateway-z1-ip"),
					//			},
					//		},
					//	},
					//}
					mock.assertSubnetCalled(vnetName, MatchAnyOfStrings([]string{"test_cluster-z2", "test_cluster-z1"})).Times(2)
					factory = mock.GetFactory()

					sut, err := infraflow.NewTfReconciler(infra, cfg, cluster, factory)
					Expect(err).ToNot(HaveOccurred())
					err = sut.Subnets(context.TODO(), armnetwork.SecurityGroup{}, armnetwork.RouteTable{}, map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse{})
					Expect(err).ToNot(HaveOccurred())

				})
			})

		})
		Describe("Enrich IP reponse", func() {
			Context("with 2 zones with user managed IPs for each", func() {
				cfg := newBasicConfig()
				cfg.Zoned = true
				cfg.Networks.Zones = []azure.Zone{{Name: 1, CIDR: "10.0.0.0/16", NatGateway: &azure.ZonedNatGatewayConfig{Enabled: true, IPAddresses: []azure.ZonedPublicIPReference{{Name: "my-ip1", ResourceGroup: resourceGroupName}}}}, {Name: 2, CIDR: "10.1.0.0/16", NatGateway: &azure.ZonedNatGatewayConfig{Enabled: true, IPAddresses: []azure.ZonedPublicIPReference{{Name: "my-ip2", ResourceGroup: resourceGroupName}}}}}

				BeforeEach(func() {
					mock := NewMockFactoryWrapper(resourceGroupName, location)
					mock.assertPublicIPGet(resourceGroupName, MatchAnyOfStrings([]string{"my-ip1", "my-ip2"})).Times(2)
					factory = mock.GetFactory()
				})
				It("enriches with 2 user managed IPs", func() {
					sut, err := infraflow.NewTfReconciler(infra, cfg, cluster, factory)
					Expect(err).ToNot(HaveOccurred())
					res := make(map[string][]armnetwork.PublicIPAddress)
					err = sut.EnrichResponseWithUserManagedIPs(context.TODO(), res)
					Expect(err).ToNot(HaveOccurred())
					Expect(res).To(HaveKey("test_cluster-z1"))
					Expect(res).To(HaveKey("test_cluster-z2"))
				})
			})
		})
		//Describe("Get Infrastructure status", func() {
		//	foreignName := "foreign-name"
		//	//Expect(err).ToNot(HaveOccurred())

		//	//var cleanupHandle framework.CleanupActionHandle
		//	//cleanupHandle = framework.AddCleanupAction(func() {
		//	//	Expect(ignoreAzureNotFoundError(teardownResourceGroup(ctx, clientSet, foreignName))).To(Succeed())
		//	//	framework.RemoveCleanupAction(cleanupHandle)
		//	//})
		//	auth := readAuthFromFile(*secretYamlPath)
		//	clientId = &auth.ClientID
		//	clientSecret = &auth.ClientSecret
		//	subscriptionId = &auth.SubscriptionID
		//	tenantId = &auth.TenantID

		//	Expect(prepareNewResourceGroup(ctx, log, clientSet, foreignName, location)).To(Succeed())
		//	Expect(prepareNewIdentity(ctx, log, clientSet, foreignName, foreignName, *region)).To(Succeed())

		//})

	})
})

type ProviderSecret struct {
	Data internal.ClientAuth `yaml:"data"`
}

// TODO share with infra test
func readAuthFromFile(fileName string) internal.ClientAuth {
	secret := ProviderSecret{}
	data, err := os.ReadFile(fileName)
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(data, &secret)
	if err != nil {
		panic(err)
	}
	secret.Data.ClientID = decodeString(secret.Data.ClientID)
	secret.Data.ClientSecret = decodeString(secret.Data.ClientSecret)
	secret.Data.SubscriptionID = decodeString(secret.Data.SubscriptionID)
	secret.Data.TenantID = decodeString(secret.Data.TenantID)
	return secret.Data
}

func decodeString(s string) string {
	res, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return string(res)
}

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
