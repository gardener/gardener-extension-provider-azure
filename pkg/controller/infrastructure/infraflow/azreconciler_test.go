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

package infraflow_test

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/Azure/azure-sdk-for-go/services/msi/mgmt/2018-11-30/msi"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	mockclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client/mock"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gofrs/uuid"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

var _ = Describe("AzureReconciler", func() {
	location := "westeurope"
	clusterName := "test_cluster"
	infra := &v1alpha1.Infrastructure{Spec: v1alpha1.InfrastructureSpec{Region: location}, ObjectMeta: metav1.ObjectMeta{Namespace: clusterName}}
	resourceGroupName := infra.Namespace //if not specified this is assumed name "t-i545428" // TODO what if resource group not given? by default Tf uses infra.Namespace
	vnetName := infra.Namespace          //if not specified this is assumed name "vnet-i545428"
	cluster := infrastructure.MakeCluster("11.0.0.0/16", "12.0.0.0/16", infra.Spec.Region, 1, 1)
	var factory *mockclient.MockFactory
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

				sut, err := infraflow.NewAzureReconciler(infra, cfg, cluster, factory)
				Expect(err).ToNot(HaveOccurred())
				err = sut.EnsureVnet(context.TODO())
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

					sut, err := infraflow.NewAzureReconciler(infra, cfg, cluster, factory)
					Expect(err).ToNot(HaveOccurred())
					err = sut.EnsureVnet(context.TODO())
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

				sut, err := infraflow.NewAzureReconciler(infra, cfg, cluster, factory)
				Expect(err).ToNot(HaveOccurred())
				err = sut.EnsureVnet(context.TODO())
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

			sut, err := infraflow.NewAzureReconciler(infra, cfg, cluster, factory)
			Expect(err).ToNot(HaveOccurred())
			_, err = sut.EnsureRouteTables(context.TODO())
			Expect(err).ToNot(HaveOccurred())
		})
	})
	Describe("Security group reconcilation", func() {
		cfg := newBasicConfig()
		It("calls the client with correct route table name", func() {
			mock := NewMockFactoryWrapper(resourceGroupName, location)

			mock.assertSecurityGroupCalled(clusterName + "-workers")
			factory = mock.GetFactory()

			sut, err := infraflow.NewAzureReconciler(infra, cfg, cluster, factory)
			Expect(err).ToNot(HaveOccurred())
			_, err = sut.EnsureSecurityGroups(context.TODO())
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
				sut, err := infraflow.NewAzureReconciler(infra, cfg, cluster, factory)
				Expect(err).ToNot(HaveOccurred())
				err = sut.EnsureAvailabilitySet(context.TODO())
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
				sut, err := infraflow.NewAzureReconciler(infra, cfg, cluster, factory)
				Expect(err).ToNot(HaveOccurred())
				err = sut.EnsureAvailabilitySet(context.TODO())
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
				sut, err := infraflow.NewAzureReconciler(infra, cfg, cluster, factory)
				Expect(err).ToNot(HaveOccurred())
				_, err = sut.EnsurePublicIPs(context.TODO())
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
				sut, err := infraflow.NewAzureReconciler(infra, cfg, cluster, factory)
				Expect(err).ToNot(HaveOccurred())
				_, err = sut.EnsurePublicIPs(context.TODO())
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
		Context("basic config with old ip (not NAT associated IP)", func() {
			cfg := newBasicConfig()
			It("should delete the old IP in the resource group during reconcilation", func() {
				ctrl := gomock.NewController(GinkgoT())
				factory := mockclient.NewMockFactory(ctrl)
				ip := mockclient.NewMockPublicIP(ctrl)
				ip.EXPECT().List(gomock.Any(), resourceGroupName).Return([]*armnetwork.PublicIPAddress{{Name: to.Ptr("old-ip")}}, nil)
				ip.EXPECT().Delete(gomock.Any(), resourceGroupName, "old-ip").Return(fmt.Errorf("delete error to not call create IP in test"))
				factory.EXPECT().PublicIP().Return(ip, nil)

				sut, err := infraflow.NewAzureReconciler(infra, cfg, cluster, factory)
				Expect(err).ToNot(HaveOccurred())

				_, err = sut.EnsurePublicIPs(context.TODO())
				Expect(err).To(HaveOccurred())
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

					sut, err := infraflow.NewAzureReconciler(infra, cfg, cluster, factory)
					Expect(err).ToNot(HaveOccurred())
					_, err = sut.EnsureNatGateways(context.TODO(), map[string][]*armnetwork.PublicIPAddress{"test_cluster-nodes-z1": {{
						ID: ipId,
					},
					}})
					Expect(err).ToNot(HaveOccurred())
				})
			})
			Context("with single subnet and NAT (old nat), then disabled", func() {
				cfg := newBasicConfig()
				cfg.Networks.NatGateway = &azure.NatGatewayConfig{
					Zone:    to.Ptr(int32(1)),
					Enabled: true,
				}
				It("deletes the old NAT during reconcilation", func() {
					ctrl := gomock.NewController(GinkgoT())
					factory := mockclient.NewMockFactory(ctrl)
					nat := mockclient.NewMockNatGateway(ctrl)
					nat.EXPECT().List(gomock.Any(), resourceGroupName).Return([]*armnetwork.NatGateway{{Name: to.Ptr("old-nat")}}, nil)
					nat.EXPECT().Delete(gomock.Any(), resourceGroupName, "old-nat").Return(fmt.Errorf("delete error to not call create NAT in test"))
					factory.EXPECT().NatGateway().Return(nat, nil)

					sut, err := infraflow.NewAzureReconciler(infra, cfg, cluster, factory)
					Expect(err).ToNot(HaveOccurred())

					_, err = sut.EnsureNatGateways(context.TODO(), map[string][]*armnetwork.PublicIPAddress{})
					Expect(err).To(HaveOccurred())
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
					mock.assertSubnetCalled(vnetName, MatchAnyOfStrings([]string{"test_cluster-nodes-z2", "test_cluster-nodes-z1"})).Times(2)
					factory = mock.GetFactory()

					sut, err := infraflow.NewAzureReconciler(infra, cfg, cluster, factory)
					Expect(err).ToNot(HaveOccurred())
					err = sut.EnsureSubnets(context.TODO(), armnetwork.SecurityGroup{}, armnetwork.RouteTable{}, map[string]*armnetwork.NatGateway{})
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
					sut, err := infraflow.NewAzureReconciler(infra, cfg, cluster, factory)
					Expect(err).ToNot(HaveOccurred())
					res := make(map[string][]*armnetwork.PublicIPAddress)
					err = sut.EnrichResponseWithUserManagedIPs(context.TODO(), res)
					Expect(err).ToNot(HaveOccurred())
					Expect(res).To(HaveKey("test_cluster-nodes-z1"))
					Expect(res).To(HaveKey("test_cluster-nodes-z2"))
				})
			})
		})
		Describe("Infrastructure Status", func() {
			Context("Basic zonal cluster with 2 zones", func() {
				cfg := newBasicConfig()
				cfg.Networks.Zones = []azure.Zone{{Name: 1, CIDR: "10.0.0.0/16", NatGateway: &azure.ZonedNatGatewayConfig{Enabled: true, IPAddresses: []azure.ZonedPublicIPReference{{Name: "my-ip", ResourceGroup: resourceGroupName}}}}, {Name: 2, CIDR: "10.1.0.0/16"}}
				cfg.Zoned = true
				It("returns the correct (static) infrastructure status without AvailabilitySet and identity enrichment", func() {
					sut, err := infraflow.NewAzureReconciler(infra, cfg, cluster, factory)
					Expect(err).ToNot(HaveOccurred())
					infra, err := sut.GetInfrastructureStatus(context.TODO())
					Expect(err).ToNot(HaveOccurred())

					Expect(infra.TypeMeta).To(Equal(infrastructure.StatusTypeMeta))
					Expect(infra.ResourceGroup.Name).To(Equal(resourceGroupName))
					Expect(infra.Networks.VNet.Name).To(Not(BeEmpty()))
					Expect(infra.RouteTables).To(Not(BeEmpty()))
					Expect(infra.SecurityGroups).To(Not(BeEmpty()))
					Expect(infra.SecurityGroups).To(Not(BeEmpty()))
					Expect(infra.Networks.Subnets).To(HaveLen(2))
				})
			})
			Context("Basic non-zoned cluster with identity", func() {
				cfg := newBasicConfig()
				cfg.Zoned = false
				cfg.Identity = &azure.IdentityConfig{Name: "my-identity", ResourceGroup: resourceGroupName}

				It("enriches the status with the AvailabilitySet and identity", func() {
					ctrl := gomock.NewController(GinkgoT())
					aclient := mockclient.NewMockAvailabilitySet(ctrl)
					aclient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(&armcompute.AvailabilitySet{ID: to.Ptr("av-id")}, nil)

					iclient := mockclient.NewMockManagedUserIdentity(ctrl)
					identity := &msi.Identity{ID: to.Ptr("identity-id"), UserAssignedIdentityProperties: &msi.UserAssignedIdentityProperties{ClientID: to.Ptr(uuid.FromStringOrNil("69359037-9599-48e7-b8f2-48393c019135"))}}

					iclient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(identity, nil)

					factory := mockclient.NewMockFactory(ctrl)
					factory.EXPECT().AvailabilitySet().Return(aclient, nil)
					factory.EXPECT().ManagedUserIdentity().Return(iclient, nil)

					sut, err := infraflow.NewAzureReconciler(infra, cfg, cluster, factory)
					Expect(err).ToNot(HaveOccurred())
					infra, err := sut.GetInfrastructureStatus(context.TODO())
					Expect(err).ToNot(HaveOccurred())
					Expect(infra.AvailabilitySets).To(HaveLen(1))
					Expect(infra.AvailabilitySets[0].ID).To(Equal("av-id"))

					Expect(infra.Identity.ID).To(Equal("identity-id"))
				})
			})
		})
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
