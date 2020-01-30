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
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	"github.com/gardener/gardener-extensions/pkg/controller"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func makeCluster(pods, services string, region string, countFaultDomain, countUpdateDomain int) *controller.Cluster {
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
				ProviderConfig: &gardencorev1beta1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: cloudProfileConfigJSON,
					},
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
		infra      *extensionsv1alpha1.Infrastructure
		config     *api.InfrastructureConfig
		cluster    *controller.Cluster
		clientAuth *internal.ClientAuth

		testServiceEndpoint = "Microsoft.Test"
		countFaultDomain    = 1
		countUpdateDomain   = 2
	)

	BeforeEach(func() {
		var (
			VNetName = "vnet"
			TestCIDR = "10.1.0.0/16"
			VNetCIDR = TestCIDR
		)
		config = &api.InfrastructureConfig{
			Networks: api.NetworkConfig{
				VNet: api.VNet{
					Name: &VNetName,
					CIDR: &VNetCIDR,
				},
				Workers:          TestCIDR,
				ServiceEndpoints: []string{testServiceEndpoint},
			},
			Zoned: true,
		}

		rawconfig := &apiv1alpha1.InfrastructureConfig{
			Networks: apiv1alpha1.NetworkConfig{
				VNet: apiv1alpha1.VNet{
					Name: &VNetName,
					CIDR: &VNetCIDR,
				},
				Workers:          TestCIDR,
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
				ProviderConfig: &runtime.RawExtension{
					Object: rawconfig,
				},
			},
		}

		cluster = makeCluster("11.0.0.0/16", "12.0.0.0/16", infra.Spec.Region, countFaultDomain, countUpdateDomain)
		clientAuth = &internal.ClientAuth{
			TenantID:       "tenant_id",
			ClientSecret:   "client_secret",
			ClientID:       "client_id",
			SubscriptionID: "subscription_id",
		}
	})

	Describe("#ComputeTerraformerChartValues", func() {
		It("should correctly compute the terraformer chart values for a zoned cluster", func() {
			values, err := ComputeTerraformerChartValues(infra, clientAuth, config, cluster)
			expectedValues := map[string]interface{}{
				"azure": map[string]interface{}{
					"subscriptionID": clientAuth.SubscriptionID,
					"tenantID":       clientAuth.TenantID,
					"region":         infra.Spec.Region,
				},
				"create": map[string]interface{}{
					"resourceGroup":   true,
					"vnet":            true,
					"availabilitySet": false,
				},
				"resourceGroup": map[string]interface{}{
					"name": infra.Namespace,
					"vnet": map[string]interface{}{
						"name": infra.Namespace,
						"cidr": config.Networks.Workers,
					},
					"subnet": map[string]interface{}{
						"serviceEndpoints": []string{testServiceEndpoint},
					},
				},
				"clusterName": infra.Namespace,
				"networks": map[string]interface{}{
					"worker": config.Networks.Workers,
				},
				"outputKeys": map[string]interface{}{
					"resourceGroupName": TerraformerOutputKeyResourceGroupName,
					"vnetName":          TerraformerOutputKeyVNetName,
					"subnetName":        TerraformerOutputKeySubnetName,
					"routeTableName":    TerraformerOutputKeyRouteTableName,
					"securityGroupName": TerraformerOutputKeySecurityGroupName,
				},
			}
			Expect(err).To(Not(HaveOccurred()))
			Expect(values).To(BeEquivalentTo(expectedValues))
		})

		It("should correctly compute the terraformer chart values for a non zoned cluster", func() {
			config.Zoned = false
			values, err := ComputeTerraformerChartValues(infra, clientAuth, config, cluster)
			Expect(err).To(Not(HaveOccurred()))
			expectedValues := map[string]interface{}{
				"azure": map[string]interface{}{
					"subscriptionID":     clientAuth.SubscriptionID,
					"tenantID":           clientAuth.TenantID,
					"region":             infra.Spec.Region,
					"countUpdateDomains": countUpdateDomain,
					"countFaultDomains":  countFaultDomain,
				},
				"create": map[string]interface{}{
					"resourceGroup":   true,
					"vnet":            true,
					"availabilitySet": true,
				},
				"resourceGroup": map[string]interface{}{
					"name": infra.Namespace,
					"vnet": map[string]interface{}{
						"name": infra.Namespace,
						"cidr": config.Networks.Workers,
					},
					"subnet": map[string]interface{}{
						"serviceEndpoints": []string{testServiceEndpoint},
					},
				},
				"clusterName": infra.Namespace,
				"networks": map[string]interface{}{
					"worker": config.Networks.Workers,
				},
				"outputKeys": map[string]interface{}{
					"resourceGroupName":   TerraformerOutputKeyResourceGroupName,
					"vnetName":            TerraformerOutputKeyVNetName,
					"subnetName":          TerraformerOutputKeySubnetName,
					"routeTableName":      TerraformerOutputKeyRouteTableName,
					"securityGroupName":   TerraformerOutputKeySecurityGroupName,
					"availabilitySetID":   TerraformerOutputKeyAvailabilitySetID,
					"availabilitySetName": TerraformerOutputKeyAvailabilitySetName,
				},
			}
			Expect(values).To(BeEquivalentTo(expectedValues))
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
			values, err := ComputeTerraformerChartValues(infra, clientAuth, config, cluster)
			expectedValues := map[string]interface{}{
				"azure": map[string]interface{}{
					"subscriptionID": clientAuth.SubscriptionID,
					"tenantID":       clientAuth.TenantID,
					"region":         infra.Spec.Region,
				},
				"create": map[string]interface{}{
					"resourceGroup":   true,
					"vnet":            false,
					"availabilitySet": false,
				},
				"resourceGroup": map[string]interface{}{
					"name": infra.Namespace,
					"vnet": map[string]interface{}{
						"name":          existingVnetName,
						"resourceGroup": existingVnetResourceGroup,
					},
					"subnet": map[string]interface{}{
						"serviceEndpoints": []string{testServiceEndpoint},
					},
				},
				"clusterName": infra.Namespace,
				"networks": map[string]interface{}{
					"worker": config.Networks.Workers,
				},
				"outputKeys": map[string]interface{}{
					"resourceGroupName": TerraformerOutputKeyResourceGroupName,
					"vnetName":          TerraformerOutputKeyVNetName,
					"vnetResourceGroup": TerraformerOutputKeyVNetResourceGroup,
					"subnetName":        TerraformerOutputKeySubnetName,
					"routeTableName":    TerraformerOutputKeyRouteTableName,
					"securityGroupName": TerraformerOutputKeySecurityGroupName,
				},
			}
			Expect(err).To(Not(HaveOccurred()))
			Expect(values).To(BeEquivalentTo(expectedValues))
		})
	})

	Describe("#StatusFromTerraformState", func() {
		var (
			vnetName, subnetName, routeTableName, availabilitySetID, availabilitySetName, securityGroupName, resourceGroupName string
			state                                                                                                              *TerraformState
		)

		BeforeEach(func() {
			vnetName = "vnet_name"
			subnetName = "subnet_name"
			routeTableName = "routTable_name"
			availabilitySetID, availabilitySetName = "as_id", "as_name"
			securityGroupName = "sg_name"
			resourceGroupName = "rg_name"
			state = &TerraformState{
				VNetName:            vnetName,
				SubnetName:          subnetName,
				RouteTableName:      routeTableName,
				AvailabilitySetID:   "",
				AvailabilitySetName: "",
				SecurityGroupName:   securityGroupName,
				ResourceGroupName:   resourceGroupName,
			}
		})

		It("should correctly compute the status for zoned cluster", func() {
			status := StatusFromTerraformState(state)
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
				},
				Zoned: true,
			}))
		})

		It("should correctly compute the status for non zoned cluster", func() {
			state.AvailabilitySetID = availabilitySetID
			state.AvailabilitySetName = availabilitySetName
			status := StatusFromTerraformState(state)
			Expect(status).To(Equal(&apiv1alpha1.InfrastructureStatus{
				TypeMeta: StatusTypeMeta,
				ResourceGroup: apiv1alpha1.ResourceGroup{
					Name: resourceGroupName,
				},
				RouteTables: []apiv1alpha1.RouteTable{
					{Name: routeTableName, Purpose: apiv1alpha1.PurposeNodes},
				},
				AvailabilitySets: []apiv1alpha1.AvailabilitySet{
					{Name: availabilitySetName, ID: availabilitySetID, Purpose: apiv1alpha1.PurposeNodes},
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
				},
				Zoned: false,
			}))
		})

	})
})
