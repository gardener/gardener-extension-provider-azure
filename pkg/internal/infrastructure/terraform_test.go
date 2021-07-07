// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package infrastructure

import (
	"encoding/json"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"

	"github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
)

func makeCluster(pods, services string, region string, countFaultDomain, countUpdateDomain int32) *controller.Cluster {
	var (
		shoot = gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Networking: gardencorev1beta1.Networking{
					Pods:     &pods,
					Services: &services,
				},
			},
		}
		cloudProfileConfig = apiv1alpha1.CloudProfileConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
				Kind:       "CloudProfileConfig",
			},
			CountFaultDomains: []apiv1alpha1.DomainCount{
				{Region: region, Count: countFaultDomain},
			},
			CountUpdateDomains: []apiv1alpha1.DomainCount{
				{Region: region, Count: countUpdateDomain},
			},
		}
		cloudProfileConfigJSON, _ = json.Marshal(cloudProfileConfig)
		cloudProfile              = gardencorev1beta1.CloudProfile{
			Spec: gardencorev1beta1.CloudProfileSpec{
				ProviderConfig: &runtime.RawExtension{
					Raw: cloudProfileConfigJSON,
				},
			},
		}
	)

	return &controller.Cluster{
		Shoot:        &shoot,
		CloudProfile: &cloudProfile,
	}
}

var _ = Describe("Terraform", func() {
	var (
		infra   *extensionsv1alpha1.Infrastructure
		config  *api.InfrastructureConfig
		cluster *controller.Cluster

		testServiceEndpoint = "Microsoft.Test"
		countFaultDomain    = int32(1)
		countUpdateDomain   = int32(2)
	)

	BeforeEach(func() {
		var (
			TestCIDR = "10.1.0.0/16"
			VNetCIDR = TestCIDR
		)
		config = &api.InfrastructureConfig{
			Networks: api.NetworkConfig{
				VNet: api.VNet{
					CIDR: &VNetCIDR,
				},
				Workers:          &TestCIDR,
				ServiceEndpoints: []string{},
			},
			Zoned: true,
		}

		rawconfig := &apiv1alpha1.InfrastructureConfig{
			Networks: apiv1alpha1.NetworkConfig{
				VNet: apiv1alpha1.VNet{
					CIDR: &VNetCIDR,
				},
				Workers:          &TestCIDR,
				ServiceEndpoints: []string{testServiceEndpoint},
			},
		}

		infra = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},

			Spec: extensionsv1alpha1.InfrastructureSpec{
				Region: "eu-west-1",
				SecretRef: corev1.SecretReference{
					Namespace: "foo",
					Name:      "azure-credentials",
				},
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					ProviderConfig: &runtime.RawExtension{
						Object: rawconfig,
					},
				},
			},
		}

		cluster = makeCluster("11.0.0.0/16", "12.0.0.0/16", infra.Spec.Region, countFaultDomain, countUpdateDomain)
	})

	Describe("#ComputeTerraformerTemplateValues", func() {
		var (
			expectedValues              map[string]interface{}
			expectedNetworksValues      map[string]interface{}
			expectedSubnetValues        map[string]interface{}
			expectedAzureValues         map[string]interface{}
			expectedCreateValues        map[string]interface{}
			expectedOutputKeysValues    map[string]interface{}
			expectedResourceGroupValues map[string]interface{}
			expectedIdentityValues      map[string]interface{}
			expectedNatGatewayValues    map[string]interface{}
		)

		BeforeEach(func() {
			expectedAzureValues = map[string]interface{}{
				"region": infra.Spec.Region,
			}
			expectedCreateValues = map[string]interface{}{
				"resourceGroup":   true,
				"vnet":            true,
				"availabilitySet": false,
			}
			expectedResourceGroupValues = map[string]interface{}{
				"name": infra.Namespace,
				"vnet": map[string]interface{}{
					"name": infra.Namespace,
					"cidr": *config.Networks.Workers,
				},
			}

			expectedOutputKeysValues = map[string]interface{}{
				"resourceGroupName": TerraformerOutputKeyResourceGroupName,
				"vnetName":          TerraformerOutputKeyVNetName,
				"subnetName":        TerraformerOutputKeySubnetName,
				"routeTableName":    TerraformerOutputKeyRouteTableName,
				"securityGroupName": TerraformerOutputKeySecurityGroupName,
			}

			expectedNatGatewayValues = map[string]interface{}{
				"enabled": false,
			}

			expectedSubnetValues = map[string]interface{}{
				"cidr":             *config.Networks.Workers,
				"natGateway":       expectedNatGatewayValues,
				"serviceEndpoints": []string{},
			}

			expectedNetworksValues = map[string]interface{}{
				"subnets": []map[string]interface{}{
					expectedSubnetValues,
				},
			}

			expectedValues = map[string]interface{}{
				"azure":         expectedAzureValues,
				"create":        expectedCreateValues,
				"resourceGroup": expectedResourceGroupValues,
				"identity":      expectedIdentityValues,
				"clusterName":   infra.Namespace,
				"networks":      expectedNetworksValues,
				"outputKeys":    expectedOutputKeysValues,
			}
		})

		It("should correctly compute the terraformer chart values for a zoned cluster", func() {
			values, err := ComputeTerraformerTemplateValues(infra, config, cluster)
			Expect(err).To(Not(HaveOccurred()))
			Expect(values).To(BeEquivalentTo(expectedValues))
		})

		Context("Cluster with primary availabilityset (non zoned)", func() {
			BeforeEach(func() {
				config.Zoned = false
				expectedCreateValues["availabilitySet"] = true
			})

			It("should correctly compute the terraformer chart values for a cluster with primary availabilityset (non zoned)", func() {
				expectedAzureValues["countUpdateDomains"] = countUpdateDomain
				expectedAzureValues["countFaultDomains"] = countFaultDomain
				expectedOutputKeysValues["availabilitySetID"] = TerraformerOutputKeyAvailabilitySetID
				expectedOutputKeysValues["availabilitySetName"] = TerraformerOutputKeyAvailabilitySetName
				expectedOutputKeysValues["countFaultDomains"] = TerraformerOutputKeyCountFaultDomains
				expectedOutputKeysValues["countUpdateDomains"] = TerraformerOutputKeyCountUpdateDomains

				values, err := ComputeTerraformerTemplateValues(infra, config, cluster)
				Expect(err).To(Not(HaveOccurred()))
				Expect(values).To(BeEquivalentTo(expectedValues))
			})

			It("should correctly compute the terraformer chart values for cluster with primary availabilityset (non zoned) w/ status", func() {
				countFaultDomains := int32(3)
				countUpdateDomains := int32(5)

				expectedAzureValues["countUpdateDomains"] = countUpdateDomains
				expectedAzureValues["countFaultDomains"] = countFaultDomains
				expectedOutputKeysValues["availabilitySetID"] = TerraformerOutputKeyAvailabilitySetID
				expectedOutputKeysValues["availabilitySetName"] = TerraformerOutputKeyAvailabilitySetName
				expectedOutputKeysValues["countFaultDomains"] = TerraformerOutputKeyCountFaultDomains
				expectedOutputKeysValues["countUpdateDomains"] = TerraformerOutputKeyCountUpdateDomains

				status := apiv1alpha1.InfrastructureStatus{
					TypeMeta: metav1.TypeMeta{
						Kind:       "InfrastructureStatus",
						APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
					},
					AvailabilitySets: []apiv1alpha1.AvailabilitySet{
						{CountFaultDomains: &countFaultDomains, CountUpdateDomains: &countUpdateDomains, Purpose: apiv1alpha1.PurposeNodes},
					},
				}

				rawStatus, err := json.Marshal(status)
				Expect(err).To(Not(HaveOccurred()))

				infra.Status = extensionsv1alpha1.InfrastructureStatus{
					DefaultStatus: extensionsv1alpha1.DefaultStatus{
						ProviderStatus: &runtime.RawExtension{
							Raw: rawStatus,
						},
					},
				}

				values, err := ComputeTerraformerTemplateValues(infra, config, cluster)
				Expect(err).To(Not(HaveOccurred()))
				Expect(values).To(BeEquivalentTo(expectedValues))
			})
		})

		Context("Cluster with VMO (non zoned)", func() {
			BeforeEach(func() {
				config.Zoned = false
				cluster.Shoot.Annotations = map[string]string{
					azure.ShootVmoUsageAnnotation: "true",
				}
			})

			It("should correctly compute the terraformer chart values", func() {
				values, err := ComputeTerraformerTemplateValues(infra, config, cluster)
				Expect(err).To(Not(HaveOccurred()))
				Expect(values).To(BeEquivalentTo(expectedValues))
			})

			It("should correctly compute the terraformer chart values with existing infrastrucutre status", func() {
				infrastructureStatus := apiv1alpha1.InfrastructureStatus{
					TypeMeta: metav1.TypeMeta{
						Kind:       "InfrastructureStatus",
						APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
					},
				}

				rawStatus, err := json.Marshal(infrastructureStatus)
				Expect(err).To(Not(HaveOccurred()))
				infra.Status = extensionsv1alpha1.InfrastructureStatus{
					DefaultStatus: extensionsv1alpha1.DefaultStatus{
						ProviderStatus: &runtime.RawExtension{
							Raw: rawStatus,
						},
					},
				}

				values, err := ComputeTerraformerTemplateValues(infra, config, cluster)
				Expect(err).To(Not(HaveOccurred()))
				Expect(values).To(BeEquivalentTo(expectedValues))
			})

			It("should throw an error as cluster already use primary availabilityset", func() {
				infrastructureStatus := apiv1alpha1.InfrastructureStatus{
					TypeMeta: metav1.TypeMeta{
						Kind:       "InfrastructureStatus",
						APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
					},
					AvailabilitySets: []apiv1alpha1.AvailabilitySet{
						{
							CountFaultDomains:  &countFaultDomain,
							CountUpdateDomains: &countUpdateDomain,
							Purpose:            apiv1alpha1.PurposeNodes,
						},
					},
				}

				rawStatus, err := json.Marshal(infrastructureStatus)
				Expect(err).To(Not(HaveOccurred()))
				infra.Status = extensionsv1alpha1.InfrastructureStatus{
					DefaultStatus: extensionsv1alpha1.DefaultStatus{
						ProviderStatus: &runtime.RawExtension{
							Raw: rawStatus,
						},
					},
				}

				_, err = ComputeTerraformerTemplateValues(infra, config, cluster)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cannot use vmss orchestration mode VM (VMO) as this cluster already used an availability set"))
			})
		})

		It("should correctly compute the terraformer chart values for a cluster deployed in an existing vnet", func() {
			var (
				existingVnetName          = "test"
				existingVnetResourceGroup = "test-rg"
			)
			config.Networks.VNet = api.VNet{
				Name:          &existingVnetName,
				ResourceGroup: &existingVnetResourceGroup,
			}

			expectedCreateValues["vnet"] = false
			expectedResourceGroupValues["vnet"] = map[string]interface{}{
				"name":          existingVnetName,
				"resourceGroup": existingVnetResourceGroup,
			}
			expectedOutputKeysValues["vnetName"] = TerraformerOutputKeyVNetName
			expectedOutputKeysValues["vnetResourceGroup"] = TerraformerOutputKeyVNetResourceGroup

			values, err := ComputeTerraformerTemplateValues(infra, config, cluster)
			Expect(err).To(Not(HaveOccurred()))
			Expect(values).To(BeEquivalentTo(expectedValues))
		})

		It("should correctly compute the terraformer chart values for a cluster with Azure Service Endpoints", func() {
			var serviceEndpointList = []string{testServiceEndpoint}
			config.Networks.ServiceEndpoints = serviceEndpointList
			expectedSubnetValues["serviceEndpoints"] = serviceEndpointList
			values, err := ComputeTerraformerTemplateValues(infra, config, cluster)
			Expect(err).To(Not(HaveOccurred()))
			Expect(values).To(BeEquivalentTo(expectedValues))
		})

		It("should correctly compute terraform chart values with identity", func() {
			var (
				identityName          = "identity-name"
				identityResourceGroup = "identity-rg"
			)
			config.Identity = &api.IdentityConfig{
				Name:          identityName,
				ResourceGroup: identityResourceGroup,
			}

			identityValues := map[string]interface{}{
				"name":          identityName,
				"resourceGroup": identityResourceGroup,
			}
			expectedValues["identity"] = identityValues
			expectedOutputKeysValues["identityID"] = TerraformerOutputKeyIdentityID
			expectedOutputKeysValues["identityClientID"] = TerraformerOutputKeyIdentityClientID

			values, err := ComputeTerraformerTemplateValues(infra, config, cluster)
			Expect(err).To(Not(HaveOccurred()))
			Expect(values).To(BeEquivalentTo(expectedValues))
		})

		Context("NatGateway", func() {
			It("should correctly compute terraform chart values with NatGateway", func() {
				config.Networks.NatGateway = &api.NatGatewayConfig{
					Enabled: true,
				}
				expectedNatGatewayValues["enabled"] = true
				values, err := ComputeTerraformerTemplateValues(infra, config, cluster)
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(BeEquivalentTo(expectedValues))
			})

			It("should correctly compute terraform chart values with NatGateway's Timeout value", func() {
				var timeout int32 = 30
				config.Networks.NatGateway = &api.NatGatewayConfig{
					Enabled:                      true,
					IdleConnectionTimeoutMinutes: &timeout,
				}
				expectedNatGatewayValues["enabled"] = true
				expectedNatGatewayValues["idleConnectionTimeoutMinutes"] = timeout
				values, err := ComputeTerraformerTemplateValues(infra, config, cluster)
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(BeEquivalentTo(expectedValues))
			})

			It("should correctly compute terraform chart values with zonal NatGateway", func() {
				config.Networks.NatGateway = &api.NatGatewayConfig{
					Enabled: true,
					Zone:    pointer.Int32Ptr(1),
				}
				expectedNatGatewayValues["enabled"] = true
				expectedNatGatewayValues["zone"] = int32(1)
				values, err := ComputeTerraformerTemplateValues(infra, config, cluster)
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(BeEquivalentTo(expectedValues))
			})

			It("should correctly compute terraform chart values with zonal public ip addresses", func() {
				var (
					ipName          = "public-ip-1-name"
					ipResourceGroup = "public-ip-1-resource-group"
				)

				config.Networks.NatGateway = &api.NatGatewayConfig{
					Enabled: true,
					Zone:    pointer.Int32Ptr(1),
					IPAddresses: []api.PublicIPReference{{
						Name:          ipName,
						ResourceGroup: ipResourceGroup,
						Zone:          int32(1),
					}},
				}
				expectedNatGatewayValues["enabled"] = true
				expectedNatGatewayValues["zone"] = int32(1)
				expectedNatGatewayValues["ipAddresses"] = []map[string]interface{}{{
					"name":          ipName,
					"resourceGroup": ipResourceGroup,
				}}

				values, err := ComputeTerraformerTemplateValues(infra, config, cluster)
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(BeEquivalentTo(expectedValues))
			})
		})

		Context("cluster using zones", func() {
			var (
				TestCIDR  = "10.1.0.0/24"
				TestCIDR2 = "10.1.1.0/24"
				VNetCIDR  = "10.1.0.0/16"
			)

			BeforeEach(func() {
				config.Networks = api.NetworkConfig{
					VNet: api.VNet{
						CIDR: &VNetCIDR,
					},
					Zones: []api.Zone{
						{
							Name:             1,
							CIDR:             TestCIDR,
							ServiceEndpoints: []string{},
						},
						{
							Name:             2,
							CIDR:             TestCIDR2,
							ServiceEndpoints: []string{},
						},
					},
				}

				expectedNetworksValues["subnets"] = []map[string]interface{}{
					{
						"cidr":             TestCIDR,
						"natGateway":       expectedNatGatewayValues,
						"serviceEndpoints": []string{},
					},
					{
						"cidr":             TestCIDR2,
						"natGateway":       expectedNatGatewayValues,
						"serviceEndpoints": []string{},
					},
				}
			})

			It("should correctly compute terraform chart with zones", func() {
				values, err := ComputeTerraformerTemplateValues(infra, config, cluster)
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(BeEquivalentTo(expectedValues))
			})

			It("should correctly compute terraform chart with zones with NAT", func() {
				config.Networks.Zones = []api.Zone{
					{
						Name:             1,
						CIDR:             TestCIDR,
						ServiceEndpoints: []string{},
						NatGateway: &api.NatGatewayConfig{
							Enabled: true,
						},
					},
					{
						Name:             2,
						CIDR:             TestCIDR2,
						ServiceEndpoints: []string{},
						NatGateway: &api.NatGatewayConfig{
							Enabled: true,
						},
					},
				}
				expectedNatGatewayValues["enabled"] = true

				values, err := ComputeTerraformerTemplateValues(infra, config, cluster)
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(BeEquivalentTo(expectedValues))
			})
		})
	})

	Describe("#StatusFromTerraformState", func() {
		var (
			vnetName, subnetName, routeTableName, availabilitySetID, availabilitySetName, securityGroupName, resourceGroupName string
			state                                                                                                              *TerraformState
			config                                                                                                             *api.InfrastructureConfig
		)

		BeforeEach(func() {
			workers := "1.1.0.0/16"

			vnetName = "vnet_name"
			subnetName = "subnet_name"
			routeTableName = "routTable_name"
			availabilitySetID, availabilitySetName = "as_id", "as_name"
			securityGroupName = "sg_name"
			resourceGroupName = "rg_name"
			config = &api.InfrastructureConfig{
				Networks: api.NetworkConfig{
					Workers: &workers,
				},
				Zoned: false,
			}
			state = &TerraformState{
				VNetName:            vnetName,
				SubnetNames:         []string{subnetName},
				RouteTableName:      routeTableName,
				AvailabilitySetID:   "",
				AvailabilitySetName: "",
				SecurityGroupName:   securityGroupName,
				ResourceGroupName:   resourceGroupName,
			}
		})

		It("should correctly compute the status for zoned cluster", func() {
			config.Zoned = true
			status := StatusFromTerraformState(config, state)
			Expect(status).To(Equal(&apiv1alpha1.InfrastructureStatus{
				TypeMeta: StatusTypeMeta,
				ResourceGroup: apiv1alpha1.ResourceGroup{
					Name: resourceGroupName,
				},
				RouteTables: []apiv1alpha1.RouteTable{
					{Name: routeTableName, Purpose: apiv1alpha1.PurposeNodes},
				},
				SecurityGroups: []apiv1alpha1.SecurityGroup{
					{Name: securityGroupName, Purpose: apiv1alpha1.PurposeNodes},
				},
				AvailabilitySets: []apiv1alpha1.AvailabilitySet{},
				Networks: apiv1alpha1.NetworkStatus{
					VNet: apiv1alpha1.VNetStatus{
						Name: vnetName,
					},
					Subnets: []apiv1alpha1.Subnet{
						{
							Purpose: apiv1alpha1.PurposeNodes,
							Name:    subnetName,
						},
					},
					Topology: apiv1alpha1.TopologyZonalSingleSubnet,
				},
				Zoned:                      true,
				NatGatewayPublicIPMigrated: true,
			}))
		})

		It("should correctly compute the status for non zoned cluster", func() {
			state.AvailabilitySetID = availabilitySetID
			state.AvailabilitySetName = availabilitySetName
			state.CountFaultDomains = 2
			state.CountUpdateDomains = 5
			status := StatusFromTerraformState(config, state)
			Expect(status).To(Equal(&apiv1alpha1.InfrastructureStatus{
				TypeMeta: StatusTypeMeta,
				ResourceGroup: apiv1alpha1.ResourceGroup{
					Name: resourceGroupName,
				},
				RouteTables: []apiv1alpha1.RouteTable{
					{Name: routeTableName, Purpose: apiv1alpha1.PurposeNodes},
				},
				AvailabilitySets: []apiv1alpha1.AvailabilitySet{
					{Name: availabilitySetName, ID: availabilitySetID, Purpose: apiv1alpha1.PurposeNodes,
						CountFaultDomains: pointer.Int32Ptr(2), CountUpdateDomains: pointer.Int32Ptr(5)},
				},
				SecurityGroups: []apiv1alpha1.SecurityGroup{
					{Name: securityGroupName, Purpose: apiv1alpha1.PurposeNodes},
				},
				Networks: apiv1alpha1.NetworkStatus{
					VNet: apiv1alpha1.VNetStatus{
						Name: vnetName,
					},
					Subnets: []apiv1alpha1.Subnet{
						{
							Purpose: apiv1alpha1.PurposeNodes,
							Name:    subnetName,
						},
					},
					Topology: apiv1alpha1.TopologyRegional,
				},
				Zoned:                      false,
				NatGatewayPublicIPMigrated: true,
			}))
		})

		It("should correctly compute the status for cluster with identity", func() {
			var (
				identityID       = "identity-id"
				identityClientID = "identity-client-id"
			)
			state.IdentityID = identityID
			state.IdentityClientID = identityClientID

			status := StatusFromTerraformState(config, state)
			Expect(status).To(Equal(&apiv1alpha1.InfrastructureStatus{
				TypeMeta: StatusTypeMeta,
				ResourceGroup: apiv1alpha1.ResourceGroup{
					Name: resourceGroupName,
				},
				RouteTables: []apiv1alpha1.RouteTable{
					{Name: routeTableName, Purpose: apiv1alpha1.PurposeNodes},
				},
				AvailabilitySets: []apiv1alpha1.AvailabilitySet{},
				SecurityGroups: []apiv1alpha1.SecurityGroup{
					{Name: securityGroupName, Purpose: apiv1alpha1.PurposeNodes},
				},
				Networks: apiv1alpha1.NetworkStatus{
					VNet: apiv1alpha1.VNetStatus{
						Name: vnetName,
					},
					Subnets: []apiv1alpha1.Subnet{
						{
							Purpose: apiv1alpha1.PurposeNodes,
							Name:    subnetName,
						},
					},
					Topology: apiv1alpha1.TopologyRegional,
				},
				Identity: &apiv1alpha1.IdentityStatus{
					ID:        identityID,
					ClientID:  identityClientID,
					ACRAccess: false,
				},
				Zoned:                      false,
				NatGatewayPublicIPMigrated: true,
			}))
		})

		It("should correctly compute the status for zoned cluster with multiple subnets", func() {
			var (
				zone1       = "1"
				zone2       = "2"
				subnetName1 = "subnet1"
				subnetName2 = "subnet2"
			)
			config.Zoned = true
			config.Networks = api.NetworkConfig{
				Zones: []api.Zone{
					{
						Name: 1,
					},
					{
						Name: 2,
					},
				},
			}
			state.SubnetNames = []string{subnetName1, subnetName2}

			status := StatusFromTerraformState(config, state)
			Expect(status).To(Equal(&apiv1alpha1.InfrastructureStatus{
				TypeMeta: StatusTypeMeta,
				ResourceGroup: apiv1alpha1.ResourceGroup{
					Name: resourceGroupName,
				},
				RouteTables: []apiv1alpha1.RouteTable{
					{Name: routeTableName, Purpose: apiv1alpha1.PurposeNodes},
				},
				SecurityGroups: []apiv1alpha1.SecurityGroup{
					{Name: securityGroupName, Purpose: apiv1alpha1.PurposeNodes},
				},
				AvailabilitySets: []apiv1alpha1.AvailabilitySet{},
				Networks: apiv1alpha1.NetworkStatus{
					VNet: apiv1alpha1.VNetStatus{
						Name: vnetName,
					},
					Subnets: []apiv1alpha1.Subnet{
						{
							Purpose: apiv1alpha1.PurposeNodes,
							Name:    subnetName1,
							Zone:    &zone1,
						},
						{
							Purpose: apiv1alpha1.PurposeNodes,
							Name:    subnetName2,
							Zone:    &zone2,
						},
					},
					Topology: apiv1alpha1.TopologyZonal,
				},
				Zoned:                      true,
				NatGatewayPublicIPMigrated: true,
			}))
		})
	})
})
