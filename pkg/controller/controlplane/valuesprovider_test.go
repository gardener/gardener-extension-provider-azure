// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"encoding/json"
	"maps"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	testutils "github.com/gardener/gardener/pkg/utils/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

const (
	namespace                              = "test"
	maxNodes                         int32 = 0
	genericTokenKubeconfigSecretName       = "generic-token-kubeconfig-92e9ae14"
)

var _ = Describe("ValuesProvider", func() {
	var (
		ctx = context.TODO()

		fakeSecretsManager secretsmanager.Interface

		vp  genericactuator.ValuesProvider
		mgr testutils.FakeManager

		scheme = runtime.NewScheme()
		_      = apisazure.AddToScheme(scheme)
		_      = v1alpha1.AddToScheme(scheme)
		_      = corev1.AddToScheme(scheme)
		_      = appsv1.AddToScheme(scheme)
		_      = vpaautoscalingv1.AddToScheme(scheme)
		_      = policyv1.AddToScheme(scheme)

		infrastructureStatus *v1alpha1.InfrastructureStatus
		controlPlaneConfig   *v1alpha1.ControlPlaneConfig
		cluster              *extensionscontroller.Cluster

		ControlPlaneChartValues map[string]interface{}

		defaultInfrastructureStatus = &v1alpha1.InfrastructureStatus{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Kind:       "InfrastructureStatus",
			},
			ResourceGroup: v1alpha1.ResourceGroup{
				Name: "rg-abcd1234",
			},
			Networks: v1alpha1.NetworkStatus{
				VNet: v1alpha1.VNetStatus{
					Name: "vnet-abcd1234",
				},
				Subnets: []v1alpha1.Subnet{
					{
						Name:    "subnet-abcd1234-nodes",
						Purpose: "nodes",
					},
				},
			},
			SecurityGroups: []v1alpha1.SecurityGroup{
				{
					Purpose: "nodes",
					Name:    "security-group-name-workers",
				},
			},
			RouteTables: []v1alpha1.RouteTable{
				{
					Purpose: "nodes",
					Name:    "route-table-name",
				},
			},
			Zoned: true,
		}

		defaultControlPlaneConfig = &v1alpha1.ControlPlaneConfig{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ControlPlaneConfig",
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
			},
			CloudControllerManager: &v1alpha1.CloudControllerManagerConfig{
				FeatureGates: map[string]bool{
					"SomeKubernetesFeature": true,
				},
			},
		}

		cidr                    = "10.250.0.0/19"
		cloudProviderConfigData = "foo"

		k8sVersion = "1.32.0"

		enabledTrue    = map[string]interface{}{"enabled": true}
		enabledFalse   = map[string]interface{}{"enabled": false}
		remedyDisabled = map[string]interface{}{"enabled": true, "replicas": 0}

		checksums = map[string]string{
			v1beta1constants.SecretNameCloudProvider: "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432",
			azure.CloudProviderDiskConfigName:        "77627eb2343b9f2dc2fca3cce35f2f9eec55783aa5f7dac21c473019e5825de2",
		}

		// controlPlaneSecret is the cloud-provider secret used in GetConfigChartValues and GetControlPlaneChartValues.
		controlPlaneSecret = &corev1.Secret{
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

		// controlPlaneConfigSecret is the azure-cloudprovider-config secret used in GetControlPlaneChartValues.
		controlPlaneConfigSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      azure.CloudProviderConfigName,
				Namespace: namespace,
			},
			Data: map[string][]byte{azure.CloudProviderConfigMapKey: []byte(cloudProviderConfigData)},
		}
	)

	// buildMgr creates a fake manager with the given objects pre-populated.
	buildMgr := func(objs ...corev1.Secret) testutils.FakeManager {
		builder := fakeclient.NewClientBuilder().WithScheme(scheme)
		for i := range objs {
			builder = builder.WithObjects(&objs[i])
		}
		return testutils.FakeManager{Client: builder.Build(), Scheme: scheme}
	}

	BeforeEach(func() {
		mgr = buildMgr(*controlPlaneSecret, *controlPlaneConfigSecret)

		fakeSecretsManagerClient := fakeclient.NewClientBuilder().WithScheme(scheme).Build()
		fakeSecretsManager = fakesecretsmanager.New(fakeSecretsManagerClient, namespace)

		vp = NewValuesProvider(mgr)

		infrastructureStatus = defaultInfrastructureStatus.DeepCopy()
		controlPlaneConfig = defaultControlPlaneConfig.DeepCopy()
		cluster = generateCluster(cidr, k8sVersion, false, nil, nil, nil)

		ControlPlaneChartValues = map[string]interface{}{
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
			"vmType":              "standard",
			"cloud":               "AZUREPUBLICCLOUD",
			"useWorkloadIdentity": false,
		}
	})

	Describe("#GetConfigChartValues", func() {
		Context("Error due to missing resources in the infrastructure status", func() {
			It("should return error, missing subnet", func() {
				infrastructureStatus.Networks.Subnets[0].Purpose = "internal"
				cp := generateControlPlane(controlPlaneConfig, infrastructureStatus)

				_, err := vp.GetConfigChartValues(ctx, cp, cluster)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not determine subnet for purpose 'nodes'"))
			})

			It("should return error, missing route tables", func() {
				infrastructureStatus.RouteTables[0].Purpose = "internal"
				cp := generateControlPlane(controlPlaneConfig, infrastructureStatus)

				_, err := vp.GetConfigChartValues(ctx, cp, cluster)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not determine route table for purpose 'nodes'"))
			})

			It("should return error, missing security groups", func() {
				infrastructureStatus.SecurityGroups[0].Purpose = "internal"
				cp := generateControlPlane(controlPlaneConfig, infrastructureStatus)

				_, err := vp.GetConfigChartValues(ctx, cp, cluster)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not determine security group for purpose 'nodes'"))
			})
		})

		Context("Generate config chart values", func() {
			It("should return correct config chart valued for cluser with vmo (non-zoned)", func() {
				infrastructureStatus.Zoned = false
				cp := generateControlPlane(controlPlaneConfig, infrastructureStatus)

				values, err := vp.GetConfigChartValues(ctx, cp, cluster)
				Expect(err).NotTo(HaveOccurred())
				maps.Copy(ControlPlaneChartValues, map[string]interface{}{
					"maxNodes": maxNodes,
					"vmType":   "vmss",
				})
				Expect(values).To(Equal(ControlPlaneChartValues))
			})

			It("should return correct config chart values for zoned cluster", func() {
				cp := generateControlPlane(controlPlaneConfig, infrastructureStatus)

				values, err := vp.GetConfigChartValues(ctx, cp, cluster)
				Expect(err).NotTo(HaveOccurred())
				maps.Copy(ControlPlaneChartValues, map[string]interface{}{
					"maxNodes": maxNodes,
				})
				Expect(values).To(Equal(ControlPlaneChartValues))
			})

			It("should return correct control plane chart values with identity", func() {
				identityName := "identity-client-id"
				infrastructureStatus.Identity = &v1alpha1.IdentityStatus{
					ClientID:  identityName,
					ACRAccess: true,
				}

				cp := generateControlPlane(controlPlaneConfig, infrastructureStatus)

				values, err := vp.GetConfigChartValues(ctx, cp, cluster)
				Expect(err).NotTo(HaveOccurred())
				maps.Copy(ControlPlaneChartValues, map[string]interface{}{
					"maxNodes":            maxNodes,
					"acrIdentityClientId": identityName,
				})
				Expect(values).To(Equal(ControlPlaneChartValues))
			})
		})
	})

	Describe("#GetControlPlaneChartValues", func() {
		var (
			ccmChartValues = utils.MergeMaps(enabledTrue, map[string]interface{}{
				"replicas":    1,
				"clusterName": namespace,
				"podNetwork":  cidr,
				"podAnnotations": map[string]interface{}{
					"checksum/secret-cloudprovider":         "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432",
					"checksum/secret-cloud-provider-config": "77627eb2343b9f2dc2fca3cce35f2f9eec55783aa5f7dac21c473019e5825de2",
				},
				"podLabels": map[string]interface{}{
					"maintenance.gardener.cloud/restart": "true",
				},
				"featureGates": map[string]bool{
					"SomeKubernetesFeature": true,
				},
				"tlsCipherSuites": []string{
					"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
					"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
					"TLS_AES_128_GCM_SHA256",
					"TLS_AES_256_GCM_SHA384",
					"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
					"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
					"TLS_CHACHA20_POLY1305_SHA256",
					"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
					"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
				},
				"secrets": map[string]interface{}{
					"server": "cloud-controller-manager-server",
				},
			})
		)

		BeforeEach(func() {
			By("creating secrets managed outside of this package for whose secretsmanager.Get() will be called")
			fakeSmClient := fakeclient.NewClientBuilder().WithScheme(scheme).Build()
			fakeSecretsManager = fakesecretsmanager.New(fakeSmClient, namespace)
			Expect(fakeSmClient.Create(context.TODO(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-provider-azure-controlplane", Namespace: namespace}})).To(Succeed())
			Expect(fakeSmClient.Create(context.TODO(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "cloud-controller-manager-server", Namespace: namespace}})).To(Succeed())

			// controlPlaneSecret (cloudprovider) is already included via buildMgr base objects
			mgr = buildMgr(*controlPlaneSecret, *controlPlaneConfigSecret)
			vp = NewValuesProvider(mgr)
		})

		It("should return correct control plane chart values without zoned infrastructure", func() {
			cluster = generateCluster(cidr, k8sVersion, true, nil, nil, &gardencorev1beta1.Seed{})
			infrastructureStatus.Zoned = false
			cp := generateControlPlane(controlPlaneConfig, infrastructureStatus)

			values, err := vp.GetControlPlaneChartValues(ctx, cp, cluster, fakeSecretsManager, checksums, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				"global": map[string]interface{}{
					"genericTokenKubeconfigSecretName": genericTokenKubeconfigSecretName,
				},
				azure.CloudControllerManagerName: utils.MergeMaps(ccmChartValues, map[string]interface{}{
					"useWorkloadIdentity": false,
				}),
				azure.CSIControllerName: utils.MergeMaps(enabledTrue, map[string]interface{}{
					"replicas": 1,
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + azure.CloudProviderConfigName: checksums[azure.CloudProviderConfigName],
					},
					"csiSnapshotController": map[string]interface{}{
						"replicas": 1,
					},
					"vmType":              "vmss",
					"useWorkloadIdentity": false,
				}),
				azure.RemedyControllerName: utils.MergeMaps(enabledTrue, map[string]interface{}{
					"replicas": 1,
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + azure.CloudProviderConfigName: checksums[azure.CloudProviderConfigName],
					},
					"useWorkloadIdentity": false,
				}),
			}))
		})

		It("should return correct control plane chart values with zoned infrastructure", func() {
			cluster = generateCluster(cidr, k8sVersion, true, nil, nil, &gardencorev1beta1.Seed{})
			infrastructureStatus.Zoned = true
			cp := generateControlPlane(controlPlaneConfig, infrastructureStatus)

			values, err := vp.GetControlPlaneChartValues(ctx, cp, cluster, fakeSecretsManager, checksums, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				"global": map[string]interface{}{
					"genericTokenKubeconfigSecretName": genericTokenKubeconfigSecretName,
				},
				azure.CloudControllerManagerName: utils.MergeMaps(ccmChartValues, map[string]interface{}{
					"useWorkloadIdentity": false,
				}),
				azure.CSIControllerName: utils.MergeMaps(enabledTrue, map[string]interface{}{
					"replicas": 1,
					"vmType":   "standard",
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + azure.CloudProviderConfigName: checksums[azure.CloudProviderConfigName],
					},
					"csiSnapshotController": map[string]interface{}{
						"replicas": 1,
					},
					"useWorkloadIdentity": false,
				}),
				azure.RemedyControllerName: utils.MergeMaps(enabledTrue, map[string]interface{}{
					"replicas": 1,
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + azure.CloudProviderConfigName: checksums[azure.CloudProviderConfigName],
					},
					"useWorkloadIdentity": false,
				}),
			}))
		})

		It("should return correct control plane chart values when remedy controller is disabled", func() {
			shootAnnotations := map[string]string{
				azure.DisableRemedyControllerAnnotation: "true",
			}
			cluster = generateCluster(cidr, k8sVersion, false, shootAnnotations, nil, &gardencorev1beta1.Seed{})

			cp := generateControlPlane(controlPlaneConfig, infrastructureStatus)
			values, err := vp.GetControlPlaneChartValues(ctx, cp, cluster, fakeSecretsManager, checksums, false)

			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				"global": map[string]interface{}{
					"genericTokenKubeconfigSecretName": genericTokenKubeconfigSecretName,
				},
				azure.CloudControllerManagerName: utils.MergeMaps(ccmChartValues, map[string]interface{}{
					"useWorkloadIdentity": false,
				}),
				azure.CSIControllerName: utils.MergeMaps(enabledTrue, map[string]interface{}{
					"replicas": 1,
					"vmType":   "standard",
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + azure.CloudProviderConfigName: checksums[azure.CloudProviderConfigName],
					},
					"csiSnapshotController": map[string]interface{}{
						"replicas": 1,
					},
					"useWorkloadIdentity": false,
				}),
				azure.RemedyControllerName: remedyDisabled,
			}))
		})

		It("should return correct control plane chart values when forcing read cache for in-tree PVs", func() {
			shootAnnotations := map[string]string{
				azure.ShootDiskConvertRWCachingModeAnnotation: "true",
			}
			cluster = generateCluster(cidr, k8sVersion, false, shootAnnotations, nil, &gardencorev1beta1.Seed{})

			cp := generateControlPlane(controlPlaneConfig, infrastructureStatus)
			values, err := vp.GetControlPlaneChartValues(ctx, cp, cluster, fakeSecretsManager, checksums, false)

			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(HaveKey(azure.CSIControllerName))
			Expect(values[azure.CSIControllerName]).To(HaveKeyWithValue("diskConfig", map[string]any{
				"convertRWCachingModeForInTreePV": true,
			}))
		})

		DescribeTable("topologyAwareRoutingEnabled value",
			func(seedSettings *gardencorev1beta1.SeedSettings, shootControlPlane *gardencorev1beta1.ControlPlane) {
				seed := &gardencorev1beta1.Seed{
					Spec: gardencorev1beta1.SeedSpec{
						Settings: seedSettings,
					},
				}
				cluster := generateCluster(cidr, k8sVersion, true, nil, shootControlPlane, seed)

				infrastructureStatus.Zoned = false
				cp := generateControlPlane(controlPlaneConfig, infrastructureStatus)

				values, err := vp.GetControlPlaneChartValues(ctx, cp, cluster, fakeSecretsManager, checksums, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(HaveKey(azure.CSIControllerName))
			},

			Entry("seed setting is nil, shoot control plane is not HA",
				nil,
				&gardencorev1beta1.ControlPlane{HighAvailability: nil},
			),
			Entry("seed setting is disabled, shoot control plane is not HA",
				&gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: false}},
				&gardencorev1beta1.ControlPlane{HighAvailability: nil},
			),
			Entry("seed setting is enabled, shoot control plane is not HA",
				&gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: true}},
				&gardencorev1beta1.ControlPlane{HighAvailability: nil},
			),
			Entry("seed setting is nil, shoot control plane is HA with failure tolerance type 'zone'",
				nil,
				&gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}},
			),
			Entry("seed setting is disabled, shoot control plane is HA with failure tolerance type 'zone'",
				&gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: false}},
				&gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}},
			),
			Entry("seed setting is enabled, shoot control plane is HA with failure tolerance type 'zone'",
				&gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: true}},
				&gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}},
			),
		)
	})

	Describe("#GetControlPlaneShootChartValues", func() {
		var (
			csiNodeEnabled = utils.MergeMaps(enabledTrue, map[string]interface{}{
				"podAnnotations": map[string]interface{}{
					"checksum/configmap-" + azure.CloudProviderDiskConfigName: checksums[azure.CloudProviderDiskConfigName],
				},
				"cloudProviderConfig": cloudProviderConfigData,
			})
			cloudControllerManager = map[string]interface{}{
				"enabled":    true,
				"vpaEnabled": true,
			}
			cloudControllerManagerWithVPADisabled = map[string]interface{}{
				"enabled":    true,
				"vpaEnabled": false,
			}

			cpDiskConfig = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      azure.CloudProviderDiskConfigName,
					Namespace: namespace,
				},
				Data: map[string][]byte{
					azure.CloudProviderConfigMapKey: []byte(cloudProviderConfigData),
				},
			}
		)

		BeforeEach(func() {
			By("creating secrets managed outside of this package for whose secretsmanager.Get() will be called")
			fakeSmClient := fakeclient.NewClientBuilder().WithScheme(scheme).Build()
			fakeSecretsManager = fakesecretsmanager.New(fakeSmClient, namespace)
			Expect(fakeSmClient.Create(context.TODO(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-provider-azure-controlplane", Namespace: namespace}})).To(Succeed())
			Expect(fakeSmClient.Create(context.TODO(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "cloud-controller-manager-server", Namespace: namespace}})).To(Succeed())

			mgr = buildMgr(*controlPlaneSecret, *controlPlaneConfigSecret, *cpDiskConfig)
			vp = NewValuesProvider(mgr)
		})

		It("should return correct control plane shoot chart values for zoned cluster", func() {
			cluster = generateCluster(cidr, k8sVersion, true, nil, nil, nil)
			cp := generateControlPlane(controlPlaneConfig, infrastructureStatus)
			csiNode := csiNodeEnabled

			values, err := vp.GetControlPlaneShootChartValues(ctx, cp, cluster, fakeSecretsManager, checksums)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				azure.AllowEgressName:            enabledTrue,
				azure.CloudControllerManagerName: cloudControllerManager,
				azure.CSINodeName:                csiNode,
				azure.RemedyControllerName:       enabledTrue,
			}))
		})

		It("should return correct control plane chart values when configuring CSI-Driver flags", func() {
			shootAnnotations := map[string]string{
				azure.VolumeAttachLimit:         "42",
				azure.ReservedVolumeAttachments: "2",
			}
			cluster = generateCluster(cidr, k8sVersion, false, shootAnnotations, nil, &gardencorev1beta1.Seed{})

			cp := generateControlPlane(controlPlaneConfig, infrastructureStatus)
			values, err := vp.GetControlPlaneShootChartValues(ctx, cp, cluster, nil, checksums)

			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(HaveKey(azure.CSINodeName))
			Expect(values[azure.CSINodeName]).To(HaveKey("driver"))
			csiConfig := values[azure.CSINodeName].(map[string]any)
			driverConfig := csiConfig["driver"].(map[string]any)
			Expect(driverConfig).To(HaveKeyWithValue("volumeAttachLimit", "42"))
			Expect(driverConfig).To(HaveKeyWithValue("reservedDataDiskSlotNum", "2"))
		})

		It("should return false if allowEgress behavior is overwritten by user", func() {
			cluster = generateCluster(cidr, k8sVersion, true, nil, nil, nil)
			infrastructureStatus.Zoned = true
			cluster.Shoot.GetAnnotations()[azure.ShootSkipAllowEgressDeployment] = "true"
			cp := generateControlPlane(controlPlaneConfig, infrastructureStatus)
			csiNode := csiNodeEnabled

			values, err := vp.GetControlPlaneShootChartValues(ctx, cp, cluster, fakeSecretsManager, checksums)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				azure.AllowEgressName:            enabledFalse,
				azure.CloudControllerManagerName: cloudControllerManager,
				azure.CSINodeName:                csiNode,
				azure.RemedyControllerName:       enabledTrue,
			}))
		})

		Context("remedy controller is disabled", func() {
			It("should return correct control plane shoot chart values for zoned cluster", func() {
				shootAnnotations := map[string]string{
					azure.DisableRemedyControllerAnnotation: "true",
				}
				cluster = generateCluster(cidr, k8sVersion, false, shootAnnotations, nil, nil)
				cp := generateControlPlane(controlPlaneConfig, infrastructureStatus)
				csiNode := csiNodeEnabled

				values, err := vp.GetControlPlaneShootChartValues(ctx, cp, cluster, fakeSecretsManager, checksums)
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(Equal(map[string]interface{}{
					azure.AllowEgressName:            enabledTrue,
					azure.CloudControllerManagerName: cloudControllerManagerWithVPADisabled,
					azure.CSINodeName:                csiNode,
					azure.RemedyControllerName:       enabledFalse,
				}))
			})
		})
	})

	Describe("#GetControlPlaneShootCRDsChartValues", func() {
		It("should return correct control plane shoot CRDs chart values", func() {
			cluster = generateCluster(cidr, k8sVersion, true, nil, nil, nil)
			cp := generateControlPlane(controlPlaneConfig, infrastructureStatus)
			values, err := vp.GetControlPlaneShootCRDsChartValues(ctx, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{"volumesnapshots": map[string]interface{}{"enabled": true}}))
		})
	})

	Describe("#GetStorageClassesChartValues()", func() {
		It("should return correct storage class chart values when not using managed classes", func() {
			controlPlaneConfig.Storage = &v1alpha1.Storage{
				ManagedDefaultStorageClass:        ptr.To(false),
				ManagedDefaultVolumeSnapshotClass: ptr.To(false),
			}
			cluster = generateCluster(cidr, k8sVersion, true, nil, nil, nil)
			cp := generateControlPlane(controlPlaneConfig, infrastructureStatus)
			values, err := vp.GetStorageClassesChartValues(ctx, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				"managedDefaultStorageClass":        false,
				"managedDefaultVolumeSnapshotClass": false,
			}))
		})

		It("should return correct storage class chart values when not using managed StorageClass", func() {
			controlPlaneConfig.Storage = &v1alpha1.Storage{
				ManagedDefaultStorageClass:        ptr.To(false),
				ManagedDefaultVolumeSnapshotClass: ptr.To(true),
			}
			cluster = generateCluster(cidr, k8sVersion, true, nil, nil, nil)
			cp := generateControlPlane(controlPlaneConfig, infrastructureStatus)
			values, err := vp.GetStorageClassesChartValues(ctx, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				"managedDefaultStorageClass":        false,
				"managedDefaultVolumeSnapshotClass": true,
			}))
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}

func generateControlPlane(controlPlaneConfig *v1alpha1.ControlPlaneConfig, infrastructureStatus *v1alpha1.InfrastructureStatus) *extensionsv1alpha1.ControlPlane {
	return &extensionsv1alpha1.ControlPlane{
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
					Raw: encode(controlPlaneConfig),
				},
			},
			InfrastructureProviderStatus: &runtime.RawExtension{
				Raw: encode(infrastructureStatus),
			},
		},
	}
}

func generateCluster(cidr, k8sVersion string, vpaEnabled bool, shootAnnotations map[string]string, shootControlPlane *gardencorev1beta1.ControlPlane, seed *gardencorev1beta1.Seed) *extensionscontroller.Cluster {
	shoot := &gardencorev1beta1.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: make(map[string]string),
		},
		Spec: gardencorev1beta1.ShootSpec{
			Provider: gardencorev1beta1.Provider{
				Workers: []gardencorev1beta1.Worker{
					{
						Name: "worker",
					},
				},
			},
			Region: "eu-west-1a",
			Networking: &gardencorev1beta1.Networking{
				Pods: &cidr,
			},
			Kubernetes: gardencorev1beta1.Kubernetes{
				Version: k8sVersion,
				VerticalPodAutoscaler: &gardencorev1beta1.VerticalPodAutoscaler{
					Enabled: vpaEnabled,
				},
			},
			ControlPlane: shootControlPlane,
		},
		Status: gardencorev1beta1.ShootStatus{},
	}
	if shootAnnotations != nil {
		shoot.Annotations = shootAnnotations
	}

	return &extensionscontroller.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"generic-token-kubeconfig.secret.gardener.cloud/name": genericTokenKubeconfigSecretName,
			},
		},
		Seed:  seed,
		Shoot: shoot,
	}
}
