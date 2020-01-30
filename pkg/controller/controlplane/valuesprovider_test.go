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

package controlplane

import (
	"context"
	"encoding/json"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	extensionscontroller "github.com/gardener/gardener-extensions/pkg/controller"
	mockclient "github.com/gardener/gardener-extensions/pkg/mock/controller-runtime/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

const (
	namespace       = "test"
	maxNodes  int32 = 0
)

var _ = Describe("ValuesProvider", func() {
	var (
		ctrl *gomock.Controller

		// Build scheme
		scheme = runtime.NewScheme()
		_      = apisazure.AddToScheme(scheme)

		cp = &extensionsv1alpha1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "control-plane",
				Namespace: namespace,
			},
			Spec: extensionsv1alpha1.ControlPlaneSpec{
				Region: "eu-west-1a",
				SecretRef: corev1.SecretReference{
					Name:      v1beta1constants.SecretNameCloudProvider,
					Namespace: namespace,
				},
				ProviderConfig: &runtime.RawExtension{
					Raw: encode(&apisazure.ControlPlaneConfig{
						CloudControllerManager: &apisazure.CloudControllerManagerConfig{
							FeatureGates: map[string]bool{
								"CustomResourceValidation": true,
							},
						},
					}),
				},
				InfrastructureProviderStatus: &runtime.RawExtension{
					Raw: encode(&apisazure.InfrastructureStatus{
						ResourceGroup: apisazure.ResourceGroup{
							Name: "rg-abcd1234",
						},
						Networks: apisazure.NetworkStatus{
							VNet: apisazure.VNetStatus{
								Name: "vnet-abcd1234",
							},
							Subnets: []apisazure.Subnet{
								{
									Name:    "subnet-abcd1234-nodes",
									Purpose: "nodes",
								},
							},
						},
						SecurityGroups: []apisazure.SecurityGroup{
							{
								Purpose: "nodes",
								Name:    "security-group-name-workers",
							},
						},
						RouteTables: []apisazure.RouteTable{
							{
								Purpose: "nodes",
								Name:    "route-table-name",
							},
						},
						AvailabilitySets: []apisazure.AvailabilitySet{
							{
								Name:    "availability-set-name",
								Purpose: "nodes",
								ID:      "/my/azure/id",
							},
						},
						Zoned: false,
					}),
				},
			},
		}

		cpNoSubnet = &extensionsv1alpha1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "control-plane",
				Namespace: namespace,
			},
			Spec: extensionsv1alpha1.ControlPlaneSpec{
				Region: "eu-west-1a",
				SecretRef: corev1.SecretReference{
					Name:      v1beta1constants.SecretNameCloudProvider,
					Namespace: namespace,
				},
				ProviderConfig: &runtime.RawExtension{
					Raw: encode(&apisazure.ControlPlaneConfig{
						CloudControllerManager: &apisazure.CloudControllerManagerConfig{
							FeatureGates: map[string]bool{
								"CustomResourceValidation": true,
							},
						},
					}),
				},
				InfrastructureProviderStatus: &runtime.RawExtension{
					Raw: encode(&apisazure.InfrastructureStatus{
						ResourceGroup: apisazure.ResourceGroup{
							Name: "rg-abcd1234",
						},
						Networks: apisazure.NetworkStatus{
							VNet: apisazure.VNetStatus{
								Name: "vnet-abcd1234",
							},
							Subnets: []apisazure.Subnet{
								{
									Name:    "subnet-abcd1234-nodes",
									Purpose: "internal",
								},
							},
						},
						SecurityGroups: []apisazure.SecurityGroup{
							{
								Purpose: "nodes",
								Name:    "security-group-name-workers",
							},
						},
						RouteTables: []apisazure.RouteTable{
							{
								Purpose: "nodes",
								Name:    "route-table-name",
							},
						},
						AvailabilitySets: []apisazure.AvailabilitySet{
							{
								Name:    "availability-set-name",
								Purpose: "nodes",
								ID:      "/my/azure/id",
							},
						},
						Zoned: false,
					}),
				},
			},
		}

		cpNoAvailabilitySet = &extensionsv1alpha1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "control-plane",
				Namespace: namespace,
			},
			Spec: extensionsv1alpha1.ControlPlaneSpec{
				Region: "eu-west-1a",
				SecretRef: corev1.SecretReference{
					Name:      v1beta1constants.SecretNameCloudProvider,
					Namespace: namespace,
				},
				ProviderConfig: &runtime.RawExtension{
					Raw: encode(&apisazure.ControlPlaneConfig{
						CloudControllerManager: &apisazure.CloudControllerManagerConfig{
							FeatureGates: map[string]bool{
								"CustomResourceValidation": true,
							},
						},
					}),
				},
				InfrastructureProviderStatus: &runtime.RawExtension{
					Raw: encode(&apisazure.InfrastructureStatus{
						ResourceGroup: apisazure.ResourceGroup{
							Name: "rg-abcd1234",
						},
						Networks: apisazure.NetworkStatus{
							VNet: apisazure.VNetStatus{
								Name: "vnet-abcd1234",
							},
							Subnets: []apisazure.Subnet{
								{
									Name:    "subnet-abcd1234-nodes",
									Purpose: "nodes",
								},
							},
						},
						SecurityGroups: []apisazure.SecurityGroup{
							{
								Purpose: "nodes",
								Name:    "security-group-name-workers",
							},
						},
						RouteTables: []apisazure.RouteTable{
							{
								Purpose: "nodes",
								Name:    "route-table-name",
							},
						},
						Zoned: false,
					}),
				},
			},
		}

		cpNoSecurityGroups = &extensionsv1alpha1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "control-plane",
				Namespace: namespace,
			},
			Spec: extensionsv1alpha1.ControlPlaneSpec{
				Region: "eu-west-1a",
				SecretRef: corev1.SecretReference{
					Name:      v1beta1constants.SecretNameCloudProvider,
					Namespace: namespace,
				},
				ProviderConfig: &runtime.RawExtension{
					Raw: encode(&apisazure.ControlPlaneConfig{
						CloudControllerManager: &apisazure.CloudControllerManagerConfig{
							FeatureGates: map[string]bool{
								"CustomResourceValidation": true,
							},
						},
					}),
				},
				InfrastructureProviderStatus: &runtime.RawExtension{
					Raw: encode(&apisazure.InfrastructureStatus{
						ResourceGroup: apisazure.ResourceGroup{
							Name: "rg-abcd1234",
						},
						Networks: apisazure.NetworkStatus{
							VNet: apisazure.VNetStatus{
								Name: "vnet-abcd1234",
							},
							Subnets: []apisazure.Subnet{
								{
									Name:    "subnet-abcd1234-nodes",
									Purpose: "nodes",
								},
							},
						},
						SecurityGroups: []apisazure.SecurityGroup{
							{
								Purpose: "internal",
								Name:    "security-group-name-workers",
							},
						},
						RouteTables: []apisazure.RouteTable{
							{
								Purpose: "nodes",
								Name:    "route-table-name",
							},
						},
						AvailabilitySets: []apisazure.AvailabilitySet{
							{
								Name:    "availability-set-name",
								Purpose: "nodes",
								ID:      "/my/azure/id",
							},
						},
						Zoned: false,
					}),
				},
			},
		}

		cpNoRouteTables = &extensionsv1alpha1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "control-plane",
				Namespace: namespace,
			},
			Spec: extensionsv1alpha1.ControlPlaneSpec{
				Region: "eu-west-1a",
				SecretRef: corev1.SecretReference{
					Name:      v1beta1constants.SecretNameCloudProvider,
					Namespace: namespace,
				},
				ProviderConfig: &runtime.RawExtension{
					Raw: encode(&apisazure.ControlPlaneConfig{
						CloudControllerManager: &apisazure.CloudControllerManagerConfig{
							FeatureGates: map[string]bool{
								"CustomResourceValidation": true,
							},
						},
					}),
				},
				InfrastructureProviderStatus: &runtime.RawExtension{
					Raw: encode(&apisazure.InfrastructureStatus{
						ResourceGroup: apisazure.ResourceGroup{
							Name: "rg-abcd1234",
						},
						Networks: apisazure.NetworkStatus{
							VNet: apisazure.VNetStatus{
								Name: "vnet-abcd1234",
							},
							Subnets: []apisazure.Subnet{
								{
									Name:    "subnet-abcd1234-nodes",
									Purpose: "nodes",
								},
							},
						},
						SecurityGroups: []apisazure.SecurityGroup{
							{
								Purpose: "nodes",
								Name:    "security-group-name-workers",
							},
						},
						RouteTables: []apisazure.RouteTable{
							{
								Purpose: "internal",
								Name:    "route-table-name",
							},
						},
						AvailabilitySets: []apisazure.AvailabilitySet{
							{
								Name:    "availability-set-name",
								Purpose: "nodes",
								ID:      "/my/azure/id",
							},
						},
						Zoned: false,
					}),
				},
			},
		}

		cpZoned = &extensionsv1alpha1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "control-plane",
				Namespace: namespace,
			},
			Spec: extensionsv1alpha1.ControlPlaneSpec{
				Region: "eu-west-1a",
				SecretRef: corev1.SecretReference{
					Name:      v1beta1constants.SecretNameCloudProvider,
					Namespace: namespace,
				},
				ProviderConfig: &runtime.RawExtension{
					Raw: encode(&apisazure.ControlPlaneConfig{
						CloudControllerManager: &apisazure.CloudControllerManagerConfig{
							FeatureGates: map[string]bool{
								"CustomResourceValidation": true,
							},
						},
					}),
				},
				InfrastructureProviderStatus: &runtime.RawExtension{
					Raw: encode(&apisazure.InfrastructureStatus{
						ResourceGroup: apisazure.ResourceGroup{
							Name: "rg-abcd1234",
						},
						Networks: apisazure.NetworkStatus{
							VNet: apisazure.VNetStatus{
								Name: "vnet-abcd1234",
							},
							Subnets: []apisazure.Subnet{
								{
									Name:    "subnet-abcd1234-nodes",
									Purpose: "nodes",
								},
							},
						},
						SecurityGroups: []apisazure.SecurityGroup{
							{
								Purpose: "nodes",
								Name:    "security-group-name-workers",
							},
						},
						RouteTables: []apisazure.RouteTable{
							{
								Purpose: "nodes",
								Name:    "route-table-name",
							},
						},
						Zoned: true,
					}),
				},
			},
		}

		cidr    = "10.250.0.0/19"
		cluster = &extensionscontroller.Cluster{
			Shoot: &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: gardencorev1beta1.Networking{
						Pods: &cidr,
					},
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.13.4",
					},
				},
			},
		}

		cpSecretKey = client.ObjectKey{Namespace: namespace, Name: v1beta1constants.SecretNameCloudProvider}
		cpSecret    = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"clientID":       []byte(`ClientID`),
				"clientSecret":   []byte(`ClientSecret`),
				"subscriptionID": []byte(`SubscriptionID`),
				"tenantID":       []byte(`TenantID`),
			},
		}

		checksums = map[string]string{
			v1beta1constants.SecretNameCloudProvider: "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432",
			azure.CloudProviderConfigName:            "08a7bc7fe8f59b055f173145e211760a83f02cf89635cef26ebb351378635606",
			"cloud-controller-manager":               "3d791b164a808638da9a8df03924be2a41e34cd664e42231c00fe369e3588272",
			"cloud-controller-manager-server":        "6dff2a2e6f14444b66d8e4a351c049f7e89ee24ba3eaab95dbec40ba6bdebb52",
		}

		configNonZonedClusterChartValues = map[string]interface{}{
			"tenantId":            "TenantID",
			"subscriptionId":      "SubscriptionID",
			"aadClientId":         "ClientID",
			"aadClientSecret":     "ClientSecret",
			"resourceGroup":       "rg-abcd1234",
			"vnetName":            "vnet-abcd1234",
			"subnetName":          "subnet-abcd1234-nodes",
			"region":              "eu-west-1a",
			"availabilitySetName": "availability-set-name",
			"routeTableName":      "route-table-name",
			"securityGroupName":   "security-group-name-workers",
			"kubernetesVersion":   "1.13.4",
			"maxNodes":            maxNodes,
		}

		configZonedClusterChartValues = map[string]interface{}{
			"tenantId":          "TenantID",
			"subscriptionId":    "SubscriptionID",
			"aadClientId":       "ClientID",
			"aadClientSecret":   "ClientSecret",
			"resourceGroup":     "rg-abcd1234",
			"vnetName":          "vnet-abcd1234",
			"subnetName":        "subnet-abcd1234-nodes",
			"region":            "eu-west-1a",
			"routeTableName":    "route-table-name",
			"securityGroupName": "security-group-name-workers",
			"kubernetesVersion": "1.13.4",
			"maxNodes":          maxNodes,
		}

		ccmChartValues = map[string]interface{}{
			"replicas":          1,
			"clusterName":       namespace,
			"kubernetesVersion": "1.13.4",
			"podNetwork":        cidr,
			"podAnnotations": map[string]interface{}{
				"checksum/secret-cloud-controller-manager":        "3d791b164a808638da9a8df03924be2a41e34cd664e42231c00fe369e3588272",
				"checksum/secret-cloud-controller-manager-server": "6dff2a2e6f14444b66d8e4a351c049f7e89ee24ba3eaab95dbec40ba6bdebb52",
				"checksum/secret-cloudprovider":                   "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432",
				"checksum/configmap-cloud-provider-config":        "08a7bc7fe8f59b055f173145e211760a83f02cf89635cef26ebb351378635606",
			},
			"featureGates": map[string]bool{
				"CustomResourceValidation": true,
			},
		}

		logger = log.Log.WithName("test")
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#GetConfigChartValues", func() {
		It("should return correct config chart values for non zoned cluster", func() {
			// Create mock client
			client := mockclient.NewMockClient(ctrl)
			client.EXPECT().Get(context.TODO(), cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))

			// Create valuesProvider
			vp := NewValuesProvider(logger)
			err := vp.(inject.Scheme).InjectScheme(scheme)
			Expect(err).NotTo(HaveOccurred())
			err = vp.(inject.Client).InjectClient(client)
			Expect(err).NotTo(HaveOccurred())

			// Call GetConfigChartValues method and check the result
			values, err := vp.GetConfigChartValues(context.TODO(), cp, cluster)

			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(configNonZonedClusterChartValues))
		})
	})

	It("should return correct config chart values for zoned cluster", func() {
		// Create mock client
		client := mockclient.NewMockClient(ctrl)
		client.EXPECT().Get(context.TODO(), cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))

		// Create valuesProvider
		vp := NewValuesProvider(logger)
		err := vp.(inject.Scheme).InjectScheme(scheme)
		Expect(err).NotTo(HaveOccurred())
		err = vp.(inject.Client).InjectClient(client)
		Expect(err).NotTo(HaveOccurred())

		// Call GetConfigChartValues method and check the result
		values, err := vp.GetConfigChartValues(context.TODO(), cpZoned, cluster)

		Expect(err).NotTo(HaveOccurred())
		Expect(values).To(Equal(configZonedClusterChartValues))
	})

	Describe("#GetConfigChartValuesNoSubnet", func() {
		It("should return error, missing subnet", func() {
			// Create mock client
			client := mockclient.NewMockClient(ctrl)
			client.EXPECT().Get(context.TODO(), cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))

			// Create valuesProvider
			vp := NewValuesProvider(logger)
			err := vp.(inject.Scheme).InjectScheme(scheme)
			Expect(err).NotTo(HaveOccurred())
			err = vp.(inject.Client).InjectClient(client)
			Expect(err).NotTo(HaveOccurred())

			// Call GetConfigChartValues method and check the result
			_, err = vp.GetConfigChartValues(context.TODO(), cpNoSubnet, cluster)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not determine subnet for purpose 'nodes'"))
		})
	})

	Describe("#GetConfigChartValuesNoAvailabilitySet", func() {
		It("should return error, missing availability set", func() {
			// Create mock client
			client := mockclient.NewMockClient(ctrl)
			client.EXPECT().Get(context.TODO(), cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))

			// Create valuesProvider
			vp := NewValuesProvider(logger)
			err := vp.(inject.Scheme).InjectScheme(scheme)
			Expect(err).NotTo(HaveOccurred())
			err = vp.(inject.Client).InjectClient(client)
			Expect(err).NotTo(HaveOccurred())

			// Call GetConfigChartValues method and check the result
			_, err = vp.GetConfigChartValues(context.TODO(), cpNoAvailabilitySet, cluster)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not determine availability set for purpose 'nodes'"))
		})
	})

	Describe("#GetConfigChartValuesNoRouteTable", func() {
		It("should return error, missing route tables", func() {
			// Create mock client
			client := mockclient.NewMockClient(ctrl)
			client.EXPECT().Get(context.TODO(), cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))

			// Create valuesProvider
			vp := NewValuesProvider(logger)
			err := vp.(inject.Scheme).InjectScheme(scheme)
			Expect(err).NotTo(HaveOccurred())
			err = vp.(inject.Client).InjectClient(client)
			Expect(err).NotTo(HaveOccurred())

			// Call GetConfigChartValues method and check the result
			_, err = vp.GetConfigChartValues(context.TODO(), cpNoRouteTables, cluster)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not determine route table for purpose 'nodes'"))
		})
	})

	Describe("#GetConfigChartValuesNoSecurityGroups", func() {
		It("should return error, missing security groups", func() {
			// Create mock client
			client := mockclient.NewMockClient(ctrl)
			client.EXPECT().Get(context.TODO(), cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))

			// Create valuesProvider
			vp := NewValuesProvider(logger)
			err := vp.(inject.Scheme).InjectScheme(scheme)
			Expect(err).NotTo(HaveOccurred())
			err = vp.(inject.Client).InjectClient(client)
			Expect(err).NotTo(HaveOccurred())

			// Call GetConfigChartValues method and check the result
			_, err = vp.GetConfigChartValues(context.TODO(), cpNoSecurityGroups, cluster)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not determine security group for purpose 'nodes'"))
		})
	})

	Describe("#GetControlPlaneChartValues", func() {
		It("should return correct control plane chart values", func() {
			// Create valuesProvider
			vp := NewValuesProvider(logger)
			err := vp.(inject.Scheme).InjectScheme(scheme)
			Expect(err).NotTo(HaveOccurred())

			// Call GetControlPlaneChartValues method and check the result
			values, err := vp.GetControlPlaneChartValues(context.TODO(), cp, cluster, checksums, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(ccmChartValues))
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}

func clientGet(result runtime.Object) interface{} {
	return func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
		switch obj.(type) {
		case *corev1.Secret:
			*obj.(*corev1.Secret) = *result.(*corev1.Secret)
		case *corev1.ConfigMap:
			*obj.(*corev1.ConfigMap) = *result.(*corev1.ConfigMap)
		}
		return nil
	}
}
