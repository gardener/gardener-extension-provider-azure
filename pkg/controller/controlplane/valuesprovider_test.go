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

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	appsv1 "k8s.io/api/apps/v1"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
		ctrl   *gomock.Controller
		ctx    = context.TODO()
		logger = log.Log.WithName("test")

		c  *mockclient.MockClient
		vp genericactuator.ValuesProvider

		scheme = runtime.NewScheme()
		_      = apisazure.AddToScheme(scheme)

		infrastructureStatus = &apisazure.InfrastructureStatus{
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
		}

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
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					ProviderConfig: &runtime.RawExtension{
						Raw: encode(&apisazure.ControlPlaneConfig{
							CloudControllerManager: &apisazure.CloudControllerManagerConfig{
								FeatureGates: map[string]bool{
									"CustomResourceValidation": true,
								},
							},
						}),
					},
				},
				InfrastructureProviderStatus: &runtime.RawExtension{
					Raw: encode(infrastructureStatus),
				},
			},
		}

		cidr                  = "10.250.0.0/19"
		clusterK8sLessThan119 = &extensionscontroller.Cluster{
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
		clusterK8sAtLeast119 = &extensionscontroller.Cluster{
			Shoot: &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: gardencorev1beta1.Networking{
						Pods: &cidr,
					},
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.19.4",
					},
				},
			},
		}
		clusterWithRemedyControllerEnabled = &extensionscontroller.Cluster{
			Shoot: &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						enableRemedyControllerAnnotation: "true",
					},
				},
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

		acrConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: azure.CloudProviderAcrConfigName, Namespace: namespace},
		}
		errorAcrConfigMapNotFound = errors.NewNotFound(schema.GroupResource{}, azure.CloudProviderAcrConfigName)

		cloudProviderConfigData = "foo"
		cpConfigSecretKey       = client.ObjectKey{Namespace: namespace, Name: azure.CloudProviderConfigName}
		cpConfigSecret          = &corev1.Secret{
			Data: map[string][]byte{azure.CloudProviderConfigMapKey: []byte(cloudProviderConfigData)},
		}
		cpDiskConfigKey = client.ObjectKey{Namespace: namespace, Name: azure.CloudProviderDiskConfigName}
		cpDiskConfig    = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      azure.CloudProviderDiskConfigName,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				azure.CloudProviderConfigMapKey: []byte(cloudProviderConfigData),
			},
		}

		checksums = map[string]string{
			v1beta1constants.SecretNameCloudProvider:     "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432",
			azure.CloudProviderDiskConfigName:            "77627eb2343b9f2dc2fca3cce35f2f9eec55783aa5f7dac21c473019e5825de2",
			azure.CloudControllerManagerName:             "3d791b164a808638da9a8df03924be2a41e34cd664e42231c00fe369e3588272",
			azure.CloudControllerManagerName + "-server": "6dff2a2e6f14444b66d8e4a351c049f7e89ee24ba3eaab95dbec40ba6bdebb52",
			azure.CSIControllerFileName:                  "d8a928b2043db77e340b523547bf16cb4aa483f0645fe0a290ed1f20aab76257",
			azure.CSIProvisionerName:                     "65b1dac6b50673535cff480564c2e5c71077ed19b1b6e0e2291207225bdf77d4",
			azure.CSIAttacherName:                        "3f22909841cdbb80e5382d689d920309c0a7d995128e52c79773f9608ed7c289",
			azure.CSISnapshotterName:                     "6a5bfc847638c499062f7fb44e31a30a9760bf4179e1dbf85e0ff4b4f162cd68",
			azure.CSIResizerName:                         "a77e663ba1af340fb3dd7f6f8a1be47c7aa9e658198695480641e6b934c0b9ed",
			azure.CSISnapshotControllerName:              "84cba346d2e2cf96c3811b55b01f57bdd9b9bcaed7065760470942d267984eaf",
			azure.RemedyControllerName:                   "84cba346d2e2cf96c3811b55b01f57bdd9b9bcaed7065760470942d267984eaf",
		}

		enabledTrue  = map[string]interface{}{"enabled": true}
		enabledFalse = map[string]interface{}{"enabled": false}
	)

	infrastructureStatusNoSubnet := infrastructureStatus.DeepCopy()
	infrastructureStatusNoSubnet.Networks.Subnets[0].Purpose = "internal"
	cpNoSubnet := cp.DeepCopy()
	cpNoSubnet.Spec.InfrastructureProviderStatus = &runtime.RawExtension{Raw: encode(infrastructureStatusNoSubnet)}

	infrastructureStatusNoAvailabilitySet := infrastructureStatus.DeepCopy()
	infrastructureStatusNoAvailabilitySet.AvailabilitySets = nil
	cpNoAvailabilitySet := cp.DeepCopy()
	cpNoAvailabilitySet.Spec.InfrastructureProviderStatus = &runtime.RawExtension{Raw: encode(infrastructureStatusNoAvailabilitySet)}

	infrastructureStatusNoSecurityGroups := infrastructureStatus.DeepCopy()
	infrastructureStatusNoSecurityGroups.SecurityGroups[0].Purpose = "internal"
	cpNoSecurityGroups := cp.DeepCopy()
	cpNoSecurityGroups.Spec.InfrastructureProviderStatus = &runtime.RawExtension{Raw: encode(infrastructureStatusNoSecurityGroups)}

	infrastructureStatusNoRouteTables := infrastructureStatus.DeepCopy()
	infrastructureStatusNoRouteTables.RouteTables[0].Purpose = "internal"
	cpNoRouteTables := cp.DeepCopy()
	cpNoRouteTables.Spec.InfrastructureProviderStatus = &runtime.RawExtension{Raw: encode(infrastructureStatusNoRouteTables)}

	infrastructureStatusZoned := infrastructureStatus.DeepCopy()
	infrastructureStatusZoned.Zoned = true
	cpZoned := cp.DeepCopy()
	cpZoned.Spec.InfrastructureProviderStatus = &runtime.RawExtension{Raw: encode(infrastructureStatusZoned)}

	infrastructureStatusIdentity := infrastructureStatus.DeepCopy()
	infrastructureStatusIdentity.Zoned = true
	infrastructureStatusIdentity.Identity = &apisazure.IdentityStatus{ClientID: "identity-client-id", ACRAccess: true}
	cpIdentity := cp.DeepCopy()
	cpIdentity.Spec.InfrastructureProviderStatus = &runtime.RawExtension{Raw: encode(infrastructureStatusIdentity)}

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)
		vp = NewValuesProvider(logger)

		err := vp.(inject.Scheme).InjectScheme(scheme)
		Expect(err).NotTo(HaveOccurred())
		err = vp.(inject.Client).InjectClient(c)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#GetConfigChartValues", func() {
		It("should return error, missing subnet", func() {
			c.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))
			c.EXPECT().Delete(ctx, acrConfigMap).Return(errorAcrConfigMapNotFound)

			_, err := vp.GetConfigChartValues(ctx, cpNoSubnet, clusterK8sLessThan119)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not determine subnet for purpose 'nodes'"))
		})

		It("should return error, missing availability set", func() {
			c.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))
			c.EXPECT().Delete(ctx, acrConfigMap).Return(errorAcrConfigMapNotFound)

			_, err := vp.GetConfigChartValues(ctx, cpNoAvailabilitySet, clusterK8sLessThan119)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not determine availability set for purpose 'nodes'"))
		})

		It("should return error, missing route tables", func() {
			c.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))
			c.EXPECT().Delete(ctx, acrConfigMap).Return(errorAcrConfigMapNotFound)

			_, err := vp.GetConfigChartValues(ctx, cpNoRouteTables, clusterK8sLessThan119)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not determine route table for purpose 'nodes'"))
		})

		It("should return error, missing security groups", func() {
			c.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))
			c.EXPECT().Delete(ctx, acrConfigMap).Return(errorAcrConfigMapNotFound)

			_, err := vp.GetConfigChartValues(ctx, cpNoSecurityGroups, clusterK8sLessThan119)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not determine security group for purpose 'nodes'"))
		})

		It("should return correct config chart values for non zoned cluster", func() {
			c.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))
			c.EXPECT().Delete(ctx, acrConfigMap).Return(errorAcrConfigMapNotFound)

			values, err := vp.GetConfigChartValues(ctx, cp, clusterK8sLessThan119)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
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
			}))
		})

		It("should return correct config chart values for zoned cluster", func() {
			c.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))
			c.EXPECT().Delete(ctx, acrConfigMap).Return(errorAcrConfigMapNotFound)

			values, err := vp.GetConfigChartValues(ctx, cpZoned, clusterK8sLessThan119)

			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
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
			}))
		})

		It("should return correct control plane chart values with identity", func() {
			c.EXPECT().Get(ctx, cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))

			values, err := vp.GetConfigChartValues(ctx, cpIdentity, clusterK8sLessThan119)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				"tenantId":            "TenantID",
				"subscriptionId":      "SubscriptionID",
				"aadClientId":         "ClientID",
				"aadClientSecret":     "ClientSecret",
				"resourceGroup":       "rg-abcd1234",
				"vnetName":            "vnet-abcd1234",
				"subnetName":          "subnet-abcd1234-nodes",
				"region":              "eu-west-1a",
				"routeTableName":      "route-table-name",
				"securityGroupName":   "security-group-name-workers",
				"kubernetesVersion":   "1.13.4",
				"acrIdentityClientId": "identity-client-id",
				"maxNodes":            maxNodes,
			}))
		})
	})

	Describe("#GetControlPlaneChartValues", func() {
		ccmChartValues := utils.MergeMaps(enabledTrue, map[string]interface{}{
			"replicas":          1,
			"clusterName":       namespace,
			"kubernetesVersion": "1.13.4",
			"podNetwork":        cidr,
			"podAnnotations": map[string]interface{}{
				"checksum/secret-cloud-controller-manager":        "3d791b164a808638da9a8df03924be2a41e34cd664e42231c00fe369e3588272",
				"checksum/secret-cloud-controller-manager-server": "6dff2a2e6f14444b66d8e4a351c049f7e89ee24ba3eaab95dbec40ba6bdebb52",
				"checksum/secret-cloudprovider":                   "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432",
				"checksum/secret-cloud-provider-config":           "77627eb2343b9f2dc2fca3cce35f2f9eec55783aa5f7dac21c473019e5825de2",
			},
			"podLabels": map[string]interface{}{
				"maintenance.gardener.cloud/restart": "true",
			},
			"featureGates": map[string]bool{
				"CustomResourceValidation": true,
			},
		})

		BeforeEach(func() {
			c.EXPECT().Get(ctx, cpConfigSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpConfigSecret))
			c.EXPECT().Get(ctx, kutil.Key(namespace, v1beta1constants.DeploymentNameKubeAPIServer), &appsv1.Deployment{}).Return(nil)
			c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: azure.CloudProviderConfigName, Namespace: namespace}}).Return(nil)
			c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: azure.CloudProviderDiskConfigName, Namespace: namespace}}).Return(nil)
		})

		It("should return correct control plane chart values (k8s < 1.19)", func() {
			values, err := vp.GetControlPlaneChartValues(ctx, cp, clusterK8sLessThan119, checksums, false)

			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				azure.CloudControllerManagerName: utils.MergeMaps(ccmChartValues, map[string]interface{}{
					"kubernetesVersion": clusterK8sLessThan119.Shoot.Spec.Kubernetes.Version,
				}),
				azure.CSIControllerName:    enabledFalse,
				azure.RemedyControllerName: enabledFalse,
			}))
		})

		It("should return correct control plane chart values (k8s >= 1.19)", func() {
			values, err := vp.GetControlPlaneChartValues(ctx, cp, clusterK8sAtLeast119, checksums, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				azure.CloudControllerManagerName: utils.MergeMaps(ccmChartValues, map[string]interface{}{
					"kubernetesVersion": clusterK8sAtLeast119.Shoot.Spec.Kubernetes.Version,
				}),
				azure.CSIControllerName: utils.MergeMaps(enabledTrue, map[string]interface{}{
					"replicas": 1,
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + azure.CSIControllerFileName:   checksums[azure.CSIControllerFileName],
						"checksum/secret-" + azure.CSIProvisionerName:      checksums[azure.CSIProvisionerName],
						"checksum/secret-" + azure.CSIAttacherName:         checksums[azure.CSIAttacherName],
						"checksum/secret-" + azure.CSISnapshotterName:      checksums[azure.CSISnapshotterName],
						"checksum/secret-" + azure.CSIResizerName:          checksums[azure.CSIResizerName],
						"checksum/secret-" + azure.CloudProviderConfigName: checksums[azure.CloudProviderConfigName],
					},
					"csiSnapshotController": map[string]interface{}{
						"replicas": 1,
						"podAnnotations": map[string]interface{}{
							"checksum/secret-" + azure.CSISnapshotControllerName: checksums[azure.CSISnapshotControllerName],
						},
					},
				}),
				azure.RemedyControllerName: enabledFalse,
			}))
		})

		It("should return correct control plane chart values when remedy controller is enabled", func() {
			values, err := vp.GetControlPlaneChartValues(ctx, cp, clusterWithRemedyControllerEnabled, checksums, false)

			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				azure.CloudControllerManagerName: utils.MergeMaps(ccmChartValues, map[string]interface{}{
					"kubernetesVersion": clusterK8sLessThan119.Shoot.Spec.Kubernetes.Version,
				}),
				azure.CSIControllerName: enabledFalse,
				azure.RemedyControllerName: utils.MergeMaps(enabledTrue, map[string]interface{}{
					"replicas": 1,
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + azure.RemedyControllerName:    checksums[azure.RemedyControllerName],
						"checksum/secret-" + azure.CloudProviderConfigName: checksums[azure.CloudProviderConfigName],
					},
				}),
			}))
		})
	})

	Describe("#GetControlPlaneShootChartValues", func() {
		var (
			csiNodeNotEnabled = utils.MergeMaps(enabledFalse, map[string]interface{}{
				"podAnnotations": map[string]interface{}{
					"checksum/configmap-" + azure.CloudProviderDiskConfigName: "",
				},
				"cloudProviderConfig": "",
			})
			csiNodeEnabled = utils.MergeMaps(enabledTrue, map[string]interface{}{
				"podAnnotations": map[string]interface{}{
					"checksum/configmap-" + azure.CloudProviderDiskConfigName: checksums[azure.CloudProviderDiskConfigName],
				},
				"cloudProviderConfig": cloudProviderConfigData,
			})
		)

		Context("k8s < 1.19", func() {
			It("should return correct control plane shoot chart values for non zoned cluster", func() {
				values, err := vp.GetControlPlaneShootChartValues(ctx, cp, clusterK8sLessThan119, checksums)
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(Equal(map[string]interface{}{
					azure.AllowUDPEgressName:         enabledFalse,
					azure.CloudControllerManagerName: enabledTrue,
					azure.CSINodeName:                csiNodeNotEnabled,
					azure.RemedyControllerName:       enabledFalse,
				}))
			})

			It("should return correct control plane shoot chart values for zoned cluster", func() {
				values, err := vp.GetControlPlaneShootChartValues(ctx, cpZoned, clusterK8sLessThan119, checksums)
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(Equal(map[string]interface{}{
					azure.AllowUDPEgressName:         enabledTrue,
					azure.CloudControllerManagerName: enabledTrue,
					azure.CSINodeName:                csiNodeNotEnabled,
					azure.RemedyControllerName:       enabledFalse,
				}))
			})
		})

		Context("k8s >= 1.19", func() {
			BeforeEach(func() {
				c.EXPECT().Get(ctx, cpDiskConfigKey, &corev1.Secret{}).DoAndReturn(clientGet(cpDiskConfig))
			})

			It("should return correct control plane shoot chart values for non zoned cluster", func() {
				values, err := vp.GetControlPlaneShootChartValues(ctx, cp, clusterK8sAtLeast119, checksums)
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(Equal(map[string]interface{}{
					azure.AllowUDPEgressName:         enabledFalse,
					azure.CloudControllerManagerName: enabledTrue,
					azure.CSINodeName:                csiNodeEnabled,
					azure.RemedyControllerName:       enabledFalse,
				}))
			})

			It("should return correct control plane shoot chart values for zoned cluster", func() {
				values, err := vp.GetControlPlaneShootChartValues(ctx, cpZoned, clusterK8sAtLeast119, checksums)
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(Equal(map[string]interface{}{
					azure.AllowUDPEgressName:         enabledTrue,
					azure.CloudControllerManagerName: enabledTrue,
					azure.CSINodeName:                csiNodeEnabled,
					azure.RemedyControllerName:       enabledFalse,
				}))
			})
		})

		Context("remedy controller is enabled", func() {
			It("should return correct control plane shoot chart values for non zoned cluster", func() {
				values, err := vp.GetControlPlaneShootChartValues(ctx, cp, clusterWithRemedyControllerEnabled, checksums)
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(Equal(map[string]interface{}{
					azure.AllowUDPEgressName:         enabledFalse,
					azure.CloudControllerManagerName: enabledTrue,
					azure.CSINodeName:                csiNodeNotEnabled,
					azure.RemedyControllerName:       enabledTrue,
				}))
			})

			It("should return correct control plane shoot chart values for zoned cluster", func() {
				values, err := vp.GetControlPlaneShootChartValues(ctx, cpZoned, clusterWithRemedyControllerEnabled, checksums)
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(Equal(map[string]interface{}{
					azure.AllowUDPEgressName:         enabledTrue,
					azure.CloudControllerManagerName: enabledTrue,
					azure.CSINodeName:                csiNodeNotEnabled,
					azure.RemedyControllerName:       enabledTrue,
				}))
			})
		})
	})

	Describe("#GetStorageClassesChartValues()", func() {
		It("should return correct storage class chart values (k8s < 1.19)", func() {
			values, err := vp.GetStorageClassesChartValues(ctx, cp, clusterK8sLessThan119)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{"useLegacyProvisioner": true}))
		})

		It("should return correct storage class chart values (k8s >= 1.19)", func() {
			values, err := vp.GetStorageClassesChartValues(ctx, cp, clusterK8sAtLeast119)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{"useLegacyProvisioner": false}))
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
