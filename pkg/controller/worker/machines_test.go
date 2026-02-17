// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/utils"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/charts"
	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	azuretypes "github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/controller/worker"
)

var _ = Describe("Machines", func() {
	var (
		ctx = context.Background()

		ctrl         *gomock.Controller
		c            *mockclient.MockClient
		statusWriter *mockclient.MockStatusWriter
		chartApplier *mockkubernetes.MockChartApplier

		namespace, technicalID, region string
		zone1, zone2                   string
		regionAndZone1, regionAndZone2 string
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)
		chartApplier = mockkubernetes.NewMockChartApplier(ctrl)
		statusWriter = mockclient.NewMockStatusWriter(ctrl)

		// Let the seed client always the mocked status writer when Status() is called.
		c.EXPECT().Status().AnyTimes().Return(statusWriter)

		namespace = "control-plane-namespace"
		technicalID = "shoot--foobar--azure"
		region = "westeurope"
		zone1 = "1"
		zone2 = "2"
		regionAndZone1 = region + "-" + zone1
		regionAndZone2 = region + "-" + zone2
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("workerDelegate", func() {
		Describe("#GenerateMachineDeployments, #DeployMachineClasses", func() {
			var (
				machineImageName               string
				machineImageVersion            string
				machineImageVersionID          string
				machineImageVersionCommunityID string
				machineImageVersionSharedID    string
				machineImageURN                string
				machineImageID                 string
				machineImageCommunityID        string
				machineImageSharedID           string

				resourceGroupName     string
				vnetResourceGroupName string
				vnetName              string
				subnetName            string
				identityID            string
				machineType           string
				userData              []byte
				userDataSecretName    string
				userDataSecretDataKey string
				sshKey                string

				volumeSize      int
				volumeType      string
				dataVolume1Name string
				dataVolume1Size int
				dataVolume2Name string
				dataVolume2Size int
				dataVolume2Type string

				namePool1           string
				minPool1            int32
				maxPool1            int32
				maxSurgePool1       intstr.IntOrString
				maxUnavailablePool1 intstr.IntOrString

				namePool2           string
				minPool2            int32
				maxPool2            int32
				priorityPool2       int32
				maxSurgePool2       intstr.IntOrString
				maxUnavailablePool2 intstr.IntOrString

				namePool3           string
				minPool3            int32
				maxPool3            int32
				maxSurgePool3       intstr.IntOrString
				maxUnavailablePool3 intstr.IntOrString

				namePool4           string
				minPool4            int32
				maxPool4            int32
				maxSurgePool4       intstr.IntOrString
				maxUnavailablePool4 intstr.IntOrString

				labels, zone1Labels, zone2Labels map[string]string

				nodeCapacity      corev1.ResourceList
				nodeTemplateZone1 machinev1alpha1.NodeTemplate
				nodeTemplateZone2 machinev1alpha1.NodeTemplate
				nodeTemplateZone3 machinev1alpha1.NodeTemplate
				nodeTemplateZone4 machinev1alpha1.NodeTemplate

				archAMD string
				archARM string

				capacityReservationConfig apiv1alpha1.CapacityReservation
				osDiskConfig              apiv1alpha1.Volume
				diagnosticProfile         apiv1alpha1.DiagnosticsProfile
				providerConfig            *runtime.RawExtension
				workerConfig              apiv1alpha1.WorkerConfig

				shootVersionMajorMinor string
				shootVersion           string

				emptyClusterAutoscalerAnnotations map[string]string

				machineImages       []apiv1alpha1.MachineImages
				machineTypes        []apiv1alpha1.MachineType
				defaultMachineClass map[string]any

				pool1, pool2, pool3, pool4 extensionsv1alpha1.WorkerPool
				infrastructureStatus       *apisazure.InfrastructureStatus
				w                          *extensionsv1alpha1.Worker
				cluster                    *extensionscontroller.Cluster
			)

			BeforeEach(func() {
				machineImageName = "my-os"
				machineImageVersion = "123.4.5-foo+bar"
				machineImageVersionID = "2"
				machineImageVersionCommunityID = "3"
				machineImageVersionSharedID = "4"
				machineImageURN = "bar:baz:foo:123"
				machineImageID = "/shared/image/gallery/image/id"
				machineImageCommunityID = "/CommunityGalleries/gallery/Images/image/Versions/123"
				machineImageSharedID = "/SharedGalleries/gallery/Images/image/Versions/123"

				resourceGroupName = "my-rg"
				vnetResourceGroupName = "my-vnet-rg"
				vnetName = "my-vnet"
				subnetName = "subnet-1234"

				machineType = "large"
				userData = []byte("some-user-data")
				userDataSecretName = "userdata-secret-name"
				userDataSecretDataKey = "userdata-secret-key"
				sshKey = "public-key"
				identityID = "identity-id"

				volumeSize = 20
				volumeType = "Standard_LRS"
				dataVolume1Name = "foo"
				dataVolume1Size = 25
				dataVolume2Name = "bar"
				dataVolume2Size = 30
				dataVolume2Type = "Premium_LRS"

				namePool1 = "pool-1"
				minPool1 = 5
				maxPool1 = 10
				maxSurgePool1 = intstr.FromInt(3)
				maxUnavailablePool1 = intstr.FromInt32(2)

				labels = map[string]string{"component": "TiDB"}
				zone1Labels = utils.MergeStringMaps(labels, map[string]string{azuretypes.AzureCSIDiskDriverTopologyKey: regionAndZone1})
				zone2Labels = utils.MergeStringMaps(labels, map[string]string{azuretypes.AzureCSIDiskDriverTopologyKey: regionAndZone2})

				nodeCapacity = corev1.ResourceList{
					"cpu":    resource.MustParse("8"),
					"gpu":    resource.MustParse("1"),
					"memory": resource.MustParse("128Gi"),
				}

				archAMD = "amd64"
				archARM = "arm64"

				capacityReservationConfig = apiv1alpha1.CapacityReservation{
					CapacityReservationGroupID: ptr.To("/foo/bar/test-1234"),
				}

				osDiskConfig = apiv1alpha1.Volume{
					Caching: ptr.To(string(armcompute.CachingTypesReadOnly)),
				}

				diagnosticProfile = apiv1alpha1.DiagnosticsProfile{
					Enabled:    true,
					StorageURI: ptr.To("azure-storage-uri"),
				}

				workerConfig = apiv1alpha1.WorkerConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
						Kind:       "WorkerConfig",
					},
					DiagnosticsProfile:  &diagnosticProfile,
					Volume:              &osDiskConfig,
					CapacityReservation: &capacityReservationConfig,
				}

				marshalledWorkerConfig, err := json.Marshal(workerConfig)
				Expect(err).ToNot(HaveOccurred())
				providerConfig = &runtime.RawExtension{
					Raw: marshalledWorkerConfig,
				}

				namePool2 = "pool-2"
				minPool2 = 30
				maxPool2 = 45
				priorityPool2 = 100
				maxSurgePool2 = intstr.FromInt32(10)
				maxUnavailablePool2 = intstr.FromInt32(15)

				namePool3 = "pool-3"
				minPool3 = 1
				maxPool3 = 5
				maxSurgePool3 = intstr.FromInt32(2)
				maxUnavailablePool3 = intstr.FromInt32(2)

				namePool4 = "pool-4"
				minPool4 = 2
				maxPool4 = 6
				maxSurgePool4 = intstr.FromInt32(1)
				maxUnavailablePool4 = intstr.FromInt32(2)

				nodeTemplateZone1 = machinev1alpha1.NodeTemplate{
					Capacity:     nodeCapacity,
					InstanceType: machineType,
					Region:       region,
					Zone:         regionAndZone1,
					Architecture: ptr.To(archAMD),
				}

				nodeTemplateZone2 = machinev1alpha1.NodeTemplate{
					Capacity:     nodeCapacity,
					InstanceType: machineType,
					Region:       region,
					Zone:         regionAndZone1,
					Architecture: ptr.To(archAMD),
				}

				nodeTemplateZone3 = machinev1alpha1.NodeTemplate{
					Capacity:     nodeCapacity,
					InstanceType: machineType,
					Region:       region,
					Zone:         regionAndZone1,
					Architecture: ptr.To(archARM),
				}

				nodeTemplateZone4 = machinev1alpha1.NodeTemplate{
					Capacity:     nodeCapacity,
					InstanceType: machineType,
					Region:       region,
					Zone:         regionAndZone1,
					Architecture: ptr.To(archARM),
				}

				shootVersionMajorMinor = "1.32"
				shootVersion = shootVersionMajorMinor + ".0"

				machineImages = []apiv1alpha1.MachineImages{
					{
						Name: machineImageName,
						Versions: []apiv1alpha1.MachineImageVersion{
							{
								Version: machineImageVersion,
								Image: apiv1alpha1.Image{
									URN: &machineImageURN,
								},
								AcceleratedNetworking: ptr.To(true),
							},
							{
								Version: machineImageVersionID,
								Image: apiv1alpha1.Image{
									ID: &machineImageID,
								},
							},
							{
								Version:      machineImageVersionCommunityID,
								Architecture: ptr.To(archARM),
								Image: apiv1alpha1.Image{
									CommunityGalleryImageID: &machineImageCommunityID,
								},
							},
							{
								Version:      machineImageVersionSharedID,
								Architecture: ptr.To(archARM),
								Image: apiv1alpha1.Image{
									SharedGalleryImageID: &machineImageSharedID,
								},
							},
						},
					},
				}
				machineTypes = []apiv1alpha1.MachineType{
					{
						Name:                  machineType,
						AcceleratedNetworking: ptr.To(true),
					},
				}

				emptyClusterAutoscalerAnnotations = map[string]string{
					"autoscaler.gardener.cloud/max-node-provision-time":              "",
					"autoscaler.gardener.cloud/scale-down-gpu-utilization-threshold": "",
					"autoscaler.gardener.cloud/scale-down-unneeded-time":             "",
					"autoscaler.gardener.cloud/scale-down-unready-time":              "",
					"autoscaler.gardener.cloud/scale-down-utilization-threshold":     "",
				}

				pool1 = extensionsv1alpha1.WorkerPool{
					Name:              namePool1,
					Minimum:           minPool1,
					Maximum:           maxPool1,
					MaxSurge:          maxSurgePool1,
					MaxUnavailable:    maxUnavailablePool1,
					Architecture:      ptr.To(archAMD),
					MachineType:       machineType,
					KubernetesVersion: ptr.To(shootVersion),
					NodeTemplate: &extensionsv1alpha1.NodeTemplate{
						Capacity: nodeCapacity,
					},
					MachineImage: extensionsv1alpha1.MachineImage{
						Name:    machineImageName,
						Version: machineImageVersion,
					},
					UserDataSecretRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: userDataSecretName},
						Key:                  userDataSecretDataKey,
					},
					Volume: &extensionsv1alpha1.Volume{
						Size: fmt.Sprintf("%dGi", volumeSize),
					},
					DataVolumes: []extensionsv1alpha1.DataVolume{
						{
							Name: dataVolume1Name,
							Size: fmt.Sprintf("%dGi", dataVolume1Size),
						},
						{
							Name: dataVolume2Name,
							Size: fmt.Sprintf("%dGi", dataVolume2Size),
							Type: &dataVolume2Type,
						},
					},
					Labels:         zone1Labels,
					ProviderConfig: providerConfig,
					Zones:          []string{zone1},
				}

				pool2 = extensionsv1alpha1.WorkerPool{
					Name:              namePool2,
					Minimum:           minPool2,
					Maximum:           maxPool2,
					Priority:          ptr.To(priorityPool2),
					MaxSurge:          maxSurgePool2,
					Architecture:      ptr.To(archAMD),
					MaxUnavailable:    maxUnavailablePool2,
					MachineType:       machineType,
					KubernetesVersion: ptr.To(shootVersion),
					NodeTemplate: &extensionsv1alpha1.NodeTemplate{
						Capacity: nodeCapacity,
					},
					MachineImage: extensionsv1alpha1.MachineImage{
						Name:    machineImageName,
						Version: machineImageVersionID,
					},
					UserDataSecretRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: userDataSecretName},
						Key:                  userDataSecretDataKey,
					},
					Volume: &extensionsv1alpha1.Volume{
						Size: fmt.Sprintf("%dGi", volumeSize),
						Type: &volumeType,
					},
					Labels: zone1Labels,
					Zones:  []string{zone1},
				}

				pool3 = extensionsv1alpha1.WorkerPool{
					Name:              namePool3,
					Minimum:           minPool3,
					Maximum:           maxPool3,
					MaxSurge:          maxSurgePool3,
					Architecture:      ptr.To(archARM),
					MaxUnavailable:    maxUnavailablePool3,
					MachineType:       machineType,
					KubernetesVersion: ptr.To(shootVersion),
					NodeTemplate: &extensionsv1alpha1.NodeTemplate{
						Capacity: nodeCapacity,
					},
					MachineImage: extensionsv1alpha1.MachineImage{
						Name:    machineImageName,
						Version: machineImageVersionCommunityID,
					},
					UserDataSecretRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: userDataSecretName},
						Key:                  userDataSecretDataKey,
					},
					Volume: &extensionsv1alpha1.Volume{
						Size: fmt.Sprintf("%dGi", volumeSize),
						Type: &volumeType,
					},
					Labels: zone1Labels,
					Zones:  []string{zone1},
				}

				pool4 = extensionsv1alpha1.WorkerPool{
					Name:              namePool4,
					Minimum:           minPool4,
					Maximum:           maxPool4,
					MaxSurge:          maxSurgePool4,
					Architecture:      ptr.To(archARM),
					MaxUnavailable:    maxUnavailablePool4,
					MachineType:       machineType,
					KubernetesVersion: ptr.To(shootVersion),
					NodeTemplate: &extensionsv1alpha1.NodeTemplate{
						Capacity: nodeCapacity,
					},
					MachineImage: extensionsv1alpha1.MachineImage{
						Name:    machineImageName,
						Version: machineImageVersionSharedID,
					},
					UserDataSecretRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: userDataSecretName},
						Key:                  userDataSecretDataKey,
					},
					Volume: &extensionsv1alpha1.Volume{
						Size: fmt.Sprintf("%dGi", volumeSize),
						Type: &volumeType,
					},
					Labels: zone1Labels,
					Zones:  []string{zone1},
				}

				vmTags := map[string]string{
					"Name": technicalID,
					SanitizeAzureVMTag(fmt.Sprintf("kubernetes.io-cluster-%s", technicalID)): "1",
					SanitizeAzureVMTag("kubernetes.io-role-node"):                            "1",
				}
				for k, v := range labels {
					vmTags[SanitizeAzureVMTag(k)] = v
				}
				defaultMachineClass = map[string]any{
					"region":        region,
					"resourceGroup": resourceGroupName,
					"network": map[string]any{
						"vnet":              vnetName,
						"subnet":            subnetName,
						"vnetResourceGroup": vnetResourceGroupName,
					},
					"tags": vmTags,
					"secret": map[string]any{
						"cloudConfig": string(userData),
					},
					"machineType": machineType,
					"osDisk": map[string]any{
						"caching": "None",
						"size":    volumeSize,
					},
					"sshPublicKey": sshKey,
					"identityID":   identityID,
					"cloudConfiguration": map[string]any{
						"name": apisazure.AzurePublicCloudName,
					},
					"zone": zone1,
					"image": map[string]any{
						"sharedGalleryImageID": machineImageSharedID,
					},
				}

				cluster = makeCluster(technicalID, shootVersion, region, machineTypes, machineImages, 0)
				infrastructureStatus = makeInfrastructureStatus(resourceGroupName, vnetName, subnetName, true, &vnetResourceGroupName, &identityID)
				w = makeWorker(namespace, region, &sshKey, infrastructureStatus, pool1, pool2, pool3, pool4)
			})

			expectedUserDataSecretRefRead := func() {
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: userDataSecretName}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, secret *corev1.Secret, _ ...client.GetOption) error {
						secret.Data = map[string][]byte{userDataSecretDataKey: userData}
						return nil
					},
				).AnyTimes()
			}

			Describe("machine images", func() {
				var (
					urnMachineClass                     map[string]any
					imageIDMachineClass                 map[string]any
					communityGalleryImageIDMachineClass map[string]any
					sharedGalleryImageIDMachineClass    map[string]any
					machineDeployments                  worker.MachineDeployments
					machineClasses                      map[string]any

					workerPoolHash1, workerPoolHash2, workerPoolHash3, workerPoolHash4 string
					volumeSize                                                         int
				)

				BeforeEach(func() {
					urnMachineClass = copyMachineClass(defaultMachineClass)
					urnMachineClass["image"] = map[string]any{
						"urn": machineImageURN,
					}

					imageIDMachineClass = copyMachineClass(defaultMachineClass)
					imageIDMachineClass["image"] = map[string]any{
						"id": machineImageID,
					}

					communityGalleryImageIDMachineClass = copyMachineClass(defaultMachineClass)
					communityGalleryImageIDMachineClass["image"] = map[string]any{
						"communityGalleryImageID": machineImageCommunityID,
					}

					sharedGalleryImageIDMachineClass = copyMachineClass(defaultMachineClass)

					workerPoolHash1AdditionalData := []string{fmt.Sprintf("%dGi", dataVolume2Size), dataVolume2Type, fmt.Sprintf("%dGi", dataVolume1Size), identityID}
					additionalData := []string{identityID}

					workerPoolHash1, _ = worker.WorkerPoolHash(w.Spec.Pools[0], cluster, workerPoolHash1AdditionalData, workerPoolHash1AdditionalData, nil)
					workerPoolHash2, _ = worker.WorkerPoolHash(w.Spec.Pools[1], cluster, additionalData, additionalData, nil)
					workerPoolHash3, _ = worker.WorkerPoolHash(w.Spec.Pools[2], cluster, additionalData, additionalData, nil)
					workerPoolHash4, _ = worker.WorkerPoolHash(w.Spec.Pools[3], cluster, additionalData, additionalData, nil)

					var (
						machineClassPool1 = copyMachineClass(urnMachineClass)
						machineClassPool2 = copyMachineClass(imageIDMachineClass)
						machineClassPool3 = copyMachineClass(communityGalleryImageIDMachineClass)
						machineClassPool4 = copyMachineClass(sharedGalleryImageIDMachineClass)

						machineClassNamePool1 = fmt.Sprintf("%s-%s", technicalID, namePool1)
						machineClassNamePool2 = fmt.Sprintf("%s-%s", technicalID, namePool2)
						machineClassNamePool3 = fmt.Sprintf("%s-%s", technicalID, namePool3)
						machineClassNamePool4 = fmt.Sprintf("%s-%s", technicalID, namePool4)

						machineDeploymentNamePool1 = fmt.Sprintf("%s-%s-z%s", technicalID, namePool1, zone1)
						machineDeploymentNamePool2 = fmt.Sprintf("%s-%s-z%s", technicalID, namePool2, zone1)
						machineDeploymentNamePool3 = fmt.Sprintf("%s-%s-z%s", technicalID, namePool3, zone1)
						machineDeploymentNamePool4 = fmt.Sprintf("%s-%s-z%s", technicalID, namePool4, zone1)

						machineClassWithHashPool1 = fmt.Sprintf("%s-%s-z%s", machineClassNamePool1, workerPoolHash1, zone1)
						machineClassWithHashPool2 = fmt.Sprintf("%s-%s-z%s", machineClassNamePool2, workerPoolHash2, zone1)
						machineClassWithHashPool3 = fmt.Sprintf("%s-%s-z%s", machineClassNamePool3, workerPoolHash3, zone1)
						machineClassWithHashPool4 = fmt.Sprintf("%s-%s-z%s", machineClassNamePool4, workerPoolHash4, zone1)
					)

					addNameAndSecretsToMachineClass(machineClassPool1, machineClassWithHashPool1, w.Spec.SecretRef)
					addNameAndSecretsToMachineClass(machineClassPool2, machineClassWithHashPool2, w.Spec.SecretRef)
					addNameAndSecretsToMachineClass(machineClassPool3, machineClassWithHashPool3, w.Spec.SecretRef)
					addNameAndSecretsToMachineClass(machineClassPool4, machineClassWithHashPool4, w.Spec.SecretRef)
					machineClassPool1["nodeTemplate"] = nodeTemplateZone1
					machineClassPool2["nodeTemplate"] = nodeTemplateZone2
					machineClassPool3["nodeTemplate"] = nodeTemplateZone3
					machineClassPool4["nodeTemplate"] = nodeTemplateZone4

					machineClassPool1["diagnosticsProfile"] = map[string]any{
						"enabled":    diagnosticProfile.Enabled,
						"storageURI": diagnosticProfile.StorageURI,
					}

					machineClassPool1["dataDisks"] = []map[string]any{
						{
							"name":               dataVolume2Name,
							"lun":                int32(0),
							"diskSizeGB":         dataVolume2Size,
							"storageAccountType": dataVolume2Type,
							"caching":            "None",
						},
						{
							"name":       dataVolume1Name,
							"lun":        int32(1),
							"diskSizeGB": dataVolume1Size,
							"caching":    "None",
						},
					}
					machineClassPool1["osDisk"] = map[string]any{
						"size":    volumeSize,
						"caching": *osDiskConfig.Caching,
					}
					machineClassPool2["osDisk"] = map[string]any{
						"size":    volumeSize,
						"type":    volumeType,
						"caching": "None",
					}
					machineClassPool3["osDisk"] = map[string]any{
						"size":    volumeSize,
						"type":    volumeType,
						"caching": "None",
					}
					machineClassPool4["osDisk"] = map[string]any{
						"size":    volumeSize,
						"type":    volumeType,
						"caching": "None",
					}

					machineClassPool1["operatingSystem"] = map[string]any{
						"operatingSystemName":    machineImageName,
						"operatingSystemVersion": strings.ReplaceAll(machineImageVersion, "+", "_"),
					}
					machineClassPool2["operatingSystem"] = map[string]any{
						"operatingSystemName":    machineImageName,
						"operatingSystemVersion": strings.ReplaceAll(machineImageVersionID, "+", "_"),
					}
					machineClassPool3["operatingSystem"] = map[string]any{
						"operatingSystemName":    machineImageName,
						"operatingSystemVersion": strings.ReplaceAll(machineImageVersionCommunityID, "+", "_"),
					}
					machineClassPool4["operatingSystem"] = map[string]any{
						"operatingSystemName":    machineImageName,
						"operatingSystemVersion": strings.ReplaceAll(machineImageVersionSharedID, "+", "_"),
					}

					machineClassPool1["capacityReservation"] = map[string]any{
						"capacityReservationGroupID": capacityReservationConfig.CapacityReservationGroupID,
					}

					machineClasses = map[string]any{"machineClasses": []map[string]any{
						machineClassPool1,
						machineClassPool2,
						machineClassPool3,
						machineClassPool4,
					}}

					machineDeployments = worker.MachineDeployments{
						{
							Name:       machineDeploymentNamePool1,
							PoolName:   namePool1,
							ClassName:  machineClassWithHashPool1,
							SecretName: machineClassWithHashPool1,
							Minimum:    minPool1,
							Maximum:    maxPool1,
							Strategy: machinev1alpha1.MachineDeploymentStrategy{
								Type: machinev1alpha1.RollingUpdateMachineDeploymentStrategyType,
								RollingUpdate: &machinev1alpha1.RollingUpdateMachineDeployment{
									UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
										MaxSurge:       &maxSurgePool1,
										MaxUnavailable: &maxUnavailablePool1,
									},
								},
							},
							Labels:                       zone1Labels,
							MachineConfiguration:         &machinev1alpha1.MachineConfiguration{},
							ClusterAutoscalerAnnotations: emptyClusterAutoscalerAnnotations,
						},
						{
							Name:       machineDeploymentNamePool2,
							PoolName:   namePool2,
							ClassName:  machineClassWithHashPool2,
							SecretName: machineClassWithHashPool2,
							Minimum:    minPool2,
							Maximum:    maxPool2,
							Priority:   ptr.To(priorityPool2),
							Strategy: machinev1alpha1.MachineDeploymentStrategy{
								Type: machinev1alpha1.RollingUpdateMachineDeploymentStrategyType,
								RollingUpdate: &machinev1alpha1.RollingUpdateMachineDeployment{
									UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
										MaxSurge:       &maxSurgePool2,
										MaxUnavailable: &maxUnavailablePool2,
									},
								},
							},
							Labels:                       zone1Labels,
							MachineConfiguration:         &machinev1alpha1.MachineConfiguration{},
							ClusterAutoscalerAnnotations: emptyClusterAutoscalerAnnotations,
						},
						{
							Name:       machineDeploymentNamePool3,
							PoolName:   namePool3,
							ClassName:  machineClassWithHashPool3,
							SecretName: machineClassWithHashPool3,
							Minimum:    minPool3,
							Maximum:    maxPool3,
							Strategy: machinev1alpha1.MachineDeploymentStrategy{
								Type: machinev1alpha1.RollingUpdateMachineDeploymentStrategyType,
								RollingUpdate: &machinev1alpha1.RollingUpdateMachineDeployment{
									UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
										MaxSurge:       &maxSurgePool3,
										MaxUnavailable: &maxUnavailablePool3,
									},
								},
							},
							Labels:                       zone1Labels,
							MachineConfiguration:         &machinev1alpha1.MachineConfiguration{},
							ClusterAutoscalerAnnotations: emptyClusterAutoscalerAnnotations,
						},
						{
							Name:       machineDeploymentNamePool4,
							PoolName:   namePool4,
							ClassName:  machineClassWithHashPool4,
							SecretName: machineClassWithHashPool4,
							Minimum:    minPool4,
							Maximum:    maxPool4,
							Strategy: machinev1alpha1.MachineDeploymentStrategy{
								Type: machinev1alpha1.RollingUpdateMachineDeploymentStrategyType,
								RollingUpdate: &machinev1alpha1.RollingUpdateMachineDeployment{
									UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
										MaxSurge:       &maxSurgePool4,
										MaxUnavailable: &maxUnavailablePool4,
									},
								},
							},
							Labels:                       zone1Labels,
							MachineConfiguration:         &machinev1alpha1.MachineConfiguration{},
							ClusterAutoscalerAnnotations: emptyClusterAutoscalerAnnotations,
						},
					}

					volumeSize = 20
					infrastructureStatus = makeInfrastructureStatus(resourceGroupName, vnetName, subnetName, true, &vnetResourceGroupName, &identityID)
					infrastructureStatus.Networks = apisazure.NetworkStatus{
						Layout: apisazure.NetworkLayoutMultipleSubnet,
						Subnets: []apisazure.Subnet{
							{
								Name:    "subnet",
								Purpose: apisazure.PurposeNodes,
								Zone:    &zone1,
							},
						},
					}

					It("should return the expected machine deployments for profile image types", func() {
						workerDelegate := wrapNewWorkerDelegate(c, chartApplier, w, cluster, nil)

						expectedUserDataSecretRefRead()

						_ = kubernetes.Values(machineClasses)
						chartApplier.
							EXPECT().
							ApplyFromEmbeddedFS(
								ctx,
								charts.InternalChart,
								filepath.Join("internal", "machineclass"),
								namespace,
								"machineclass",
								kubernetes.Values(machineClasses),
							)

						// Test workerDelegate.DeployMachineClasses()
						err := workerDelegate.DeployMachineClasses(ctx)
						Expect(err).NotTo(HaveOccurred())

						// Test workerDelegate.UpdateMachineImagesStatus()
						expectWorkerProviderStatusUpdateToSucceed(ctx, statusWriter)
						err = workerDelegate.UpdateMachineImagesStatus(ctx)
						Expect(err).NotTo(HaveOccurred())

						// Test workerDelegate.GenerateMachineDeployments()
						result, err := workerDelegate.GenerateMachineDeployments(ctx)
						Expect(err).NotTo(HaveOccurred())

						// Expect the whole struct to be equal (just to be sure)
						Expect(result).To(Equal(machineDeployments))
					})
				})
			})

			Describe("worker with multiple zones", func() {
				var (
					namePoolZones           string
					minPoolZones            int32
					maxPoolZones            int32
					maxSurgePoolZones       intstr.IntOrString
					maxUnavailablePoolZones intstr.IntOrString

					machineDeployments worker.MachineDeployments
					machineClasses     map[string]any

					subnet1, subnet2 string
					poolZones        extensionsv1alpha1.WorkerPool
				)

				BeforeEach(func() {
					namePoolZones = "pool-zones"
					subnet1 = "subnet1"
					subnet2 = "subnet2"
					minPoolZones = 2
					maxPoolZones = 4
					poolZones = extensionsv1alpha1.WorkerPool{
						Name:              namePoolZones,
						Minimum:           minPoolZones,
						Maximum:           maxPoolZones,
						MaxSurge:          maxSurgePoolZones,
						Architecture:      ptr.To(archARM),
						MaxUnavailable:    maxUnavailablePoolZones,
						MachineType:       machineType,
						KubernetesVersion: ptr.To(shootVersion),
						NodeTemplate: &extensionsv1alpha1.NodeTemplate{
							Capacity: nodeCapacity,
						},
						MachineImage: extensionsv1alpha1.MachineImage{
							Name:    machineImageName,
							Version: machineImageVersionSharedID,
						},
						UserDataSecretRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: userDataSecretName},
							Key:                  userDataSecretDataKey,
						},
						Volume: &extensionsv1alpha1.Volume{
							Size: fmt.Sprintf("%dGi", volumeSize),
						},
						Labels: labels,
						Zones:  []string{zone1, zone2},
					}

					infrastructureStatus = makeInfrastructureStatus(resourceGroupName, vnetName, subnetName, true, &vnetResourceGroupName, &identityID)
					infrastructureStatus.Networks = apisazure.NetworkStatus{
						Layout: apisazure.NetworkLayoutMultipleSubnet,
						VNet: apisazure.VNetStatus{
							Name:          vnetName,
							ResourceGroup: ptr.To(vnetResourceGroupName),
						},
						Subnets: []apisazure.Subnet{
							{
								Name:     subnet1,
								Purpose:  apisazure.PurposeNodes,
								Zone:     &zone1,
								Migrated: true,
							},
							{
								Name:    subnet2,
								Purpose: apisazure.PurposeNodes,
								Zone:    &zone2,
							},
						},
					}
					w = makeWorker(namespace, region, &sshKey, infrastructureStatus, poolZones)

					machineClassPool1 := copyMachineClass(defaultMachineClass)
					machineClassPool1["operatingSystem"] = map[string]any{
						"operatingSystemName":    machineImageName,
						"operatingSystemVersion": strings.ReplaceAll(machineImageVersionSharedID, "+", "_"),
					}
					machineClassPool1["image"] = map[string]any{
						"sharedGalleryImageID": machineImageSharedID,
					}
					machineClassPool1["nodeTemplate"] = nodeTemplateZone4
					machineClassPool1["network"].(map[string]any)["subnet"] = subnet1

					machineClassPool2 := copyMachineClass(defaultMachineClass)
					machineClassPool2["zone"] = zone2
					machineClassPool2["operatingSystem"] = map[string]any{
						"operatingSystemName":    machineImageName,
						"operatingSystemVersion": strings.ReplaceAll(machineImageVersionSharedID, "+", "_"),
					}
					machineClassPool2["image"] = map[string]any{
						"sharedGalleryImageID": machineImageSharedID,
					}

					nodeTemplateZone4.Zone = regionAndZone2
					machineClassPool2["nodeTemplate"] = nodeTemplateZone4
					machineClassPool2["network"] = maps.Clone(machineClassPool1["network"].(map[string]any))
					machineClassPool2["network"].(map[string]any)["subnet"] = subnet2
					machineClasses = map[string]any{"machineClasses": []map[string]any{
						machineClassPool1,
						machineClassPool2,
					}}
					additionalData := []string{identityID}
					workerPoolHash1, _ := worker.WorkerPoolHash(w.Spec.Pools[0], cluster, additionalData, additionalData, nil)
					workerPoolHash2, _ := worker.WorkerPoolHash(w.Spec.Pools[0], cluster, append(additionalData, subnet2), append(additionalData, subnet2), nil)
					machineClassNamePool1 := fmt.Sprintf("%s-%s", technicalID, poolZones.Name)
					machineClassNamePool2 := fmt.Sprintf("%s-%s", technicalID, poolZones.Name)
					machineDeploymentNamePool1 := fmt.Sprintf("%s-z%s", machineClassNamePool1, zone1)
					machineDeploymentNamePool2 := fmt.Sprintf("%s-z%s", machineClassNamePool2, zone2)
					machineClassWithHashPool1 := fmt.Sprintf("%s-%s-z%s", machineClassNamePool1, workerPoolHash1, zone1)
					machineClassWithHashPool2 := fmt.Sprintf("%s-%s-z%s", machineClassNamePool2, workerPoolHash2, zone2)
					addNameAndSecretsToMachineClass(machineClassPool1, machineClassWithHashPool1, w.Spec.SecretRef)
					addNameAndSecretsToMachineClass(machineClassPool2, machineClassWithHashPool2, w.Spec.SecretRef)
					machineDeployments = worker.MachineDeployments{
						{
							Name:       machineDeploymentNamePool1,
							PoolName:   namePoolZones,
							ClassName:  machineClassWithHashPool1,
							SecretName: machineClassWithHashPool1,
							Minimum:    minPoolZones / 2,
							Maximum:    maxPoolZones / 2,
							Strategy: machinev1alpha1.MachineDeploymentStrategy{
								Type: machinev1alpha1.RollingUpdateMachineDeploymentStrategyType,
								RollingUpdate: &machinev1alpha1.RollingUpdateMachineDeployment{
									UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
										MaxSurge:       &maxSurgePoolZones,
										MaxUnavailable: &maxUnavailablePoolZones,
									},
								},
							},
							Labels:                       zone1Labels,
							MachineConfiguration:         &machinev1alpha1.MachineConfiguration{},
							ClusterAutoscalerAnnotations: emptyClusterAutoscalerAnnotations,
						},
						{
							Name:       machineDeploymentNamePool2,
							PoolName:   namePoolZones,
							ClassName:  machineClassWithHashPool2,
							SecretName: machineClassWithHashPool2,
							Minimum:    minPoolZones / 2,
							Maximum:    maxPoolZones / 2,
							Strategy: machinev1alpha1.MachineDeploymentStrategy{
								Type: machinev1alpha1.RollingUpdateMachineDeploymentStrategyType,
								RollingUpdate: &machinev1alpha1.RollingUpdateMachineDeployment{
									UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
										MaxSurge:       &maxSurgePoolZones,
										MaxUnavailable: &maxUnavailablePoolZones,
									},
								},
							},
							Labels:                       zone2Labels,
							MachineConfiguration:         &machinev1alpha1.MachineConfiguration{},
							ClusterAutoscalerAnnotations: emptyClusterAutoscalerAnnotations,
						},
					}
				})

				It("should return the correct machine deployments for zonal setup", func() {
					workerDelegate := wrapNewWorkerDelegate(c, chartApplier, w, cluster, nil)

					expectedUserDataSecretRefRead()

					chartApplier.
						EXPECT().
						ApplyFromEmbeddedFS(
							ctx,
							charts.InternalChart,
							filepath.Join("internal", "machineclass"),
							namespace,
							"machineclass",
							kubernetes.Values(machineClasses),
						)

					// Test workerDelegate.DeployMachineClasses()
					err := workerDelegate.DeployMachineClasses(ctx)
					Expect(err).NotTo(HaveOccurred())

					// Test workerDelegate.GenerateMachineDeployments()
					result, err := workerDelegate.GenerateMachineDeployments(ctx)
					Expect(err).NotTo(HaveOccurred())

					// Expect the whole struct to be equal (just to be sure)
					Expect(result).To(Equal(machineDeployments))
				})

				It("should set expected cluster-autoscaler annotations on the machine deployment", func() {
					w.Spec.Pools[0].ClusterAutoscaler = &extensionsv1alpha1.ClusterAutoscalerOptions{
						MaxNodeProvisionTime:             ptr.To(metav1.Duration{Duration: time.Minute}),
						ScaleDownGpuUtilizationThreshold: ptr.To("0.4"),
						ScaleDownUnneededTime:            ptr.To(metav1.Duration{Duration: 2 * time.Minute}),
						ScaleDownUnreadyTime:             ptr.To(metav1.Duration{Duration: 3 * time.Minute}),
						ScaleDownUtilizationThreshold:    ptr.To("0.5"),
					}

					w.Spec.Pools = append(w.Spec.Pools, pool2)
					w.Spec.Pools[1].Zones = []string{zone1, zone2}

					workerDelegate := wrapNewWorkerDelegate(c, chartApplier, w, cluster, nil)

					expectedUserDataSecretRefRead()

					result, err := workerDelegate.GenerateMachineDeployments(ctx)

					Expect(err).NotTo(HaveOccurred())
					Expect(result).NotTo(BeNil())

					Expect(result[0].ClusterAutoscalerAnnotations).NotTo(BeNil())
					Expect(result[1].ClusterAutoscalerAnnotations).NotTo(BeNil())
					for k, v := range result[2].ClusterAutoscalerAnnotations {
						Expect(v).To(BeEmpty(), "entry for key %v is not empty", k)
					}
					for k, v := range result[3].ClusterAutoscalerAnnotations {
						Expect(v).To(BeEmpty(), "entry for key %v is not empty", k)
					}

					Expect(result[0].ClusterAutoscalerAnnotations[extensionsv1alpha1.MaxNodeProvisionTimeAnnotation]).To(Equal("1m0s"))
					Expect(result[0].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownGpuUtilizationThresholdAnnotation]).To(Equal("0.4"))
					Expect(result[0].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownUnneededTimeAnnotation]).To(Equal("2m0s"))
					Expect(result[0].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownUnreadyTimeAnnotation]).To(Equal("3m0s"))
					Expect(result[0].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownUtilizationThresholdAnnotation]).To(Equal("0.5"))

					Expect(result[1].ClusterAutoscalerAnnotations[extensionsv1alpha1.MaxNodeProvisionTimeAnnotation]).To(Equal("1m0s"))
					Expect(result[1].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownGpuUtilizationThresholdAnnotation]).To(Equal("0.4"))
					Expect(result[1].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownUnneededTimeAnnotation]).To(Equal("2m0s"))
					Expect(result[1].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownUnreadyTimeAnnotation]).To(Equal("3m0s"))
					Expect(result[1].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownUtilizationThresholdAnnotation]).To(Equal("0.5"))
				})
			})

			Describe("workers with in-place updates strategy", func() {
				var (
					namePoolInPlace           string
					minPoolInPlace            int32
					maxPoolInPlace            int32
					maxSurgePoolInPlace       intstr.IntOrString
					maxUnavailablePoolInPlace intstr.IntOrString

					machineDeployments worker.MachineDeployments
					poolInPlace        extensionsv1alpha1.WorkerPool
				)
				BeforeEach(func() {
					poolInPlace = extensionsv1alpha1.WorkerPool{
						Name:              namePoolInPlace,
						Minimum:           minPoolInPlace,
						Maximum:           maxPoolInPlace,
						MaxSurge:          maxSurgePoolInPlace,
						Architecture:      ptr.To(archARM),
						MaxUnavailable:    maxUnavailablePoolInPlace,
						MachineType:       machineType,
						KubernetesVersion: ptr.To(shootVersion),
						UpdateStrategy:    ptr.To(gardencorev1beta1.AutoInPlaceUpdate),
						NodeTemplate: &extensionsv1alpha1.NodeTemplate{
							Capacity: nodeCapacity,
						},
						MachineImage: extensionsv1alpha1.MachineImage{
							Name:    machineImageName,
							Version: machineImageVersionSharedID,
						},
						UserDataSecretRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: userDataSecretName},
							Key:                  userDataSecretDataKey,
						},
						Volume: &extensionsv1alpha1.Volume{
							Size: fmt.Sprintf("%dGi", volumeSize),
							Type: &volumeType,
						},
						Labels: zone1Labels,
						Zones:  []string{zone1},
					}
					w = makeWorker(namespace, region, &sshKey, infrastructureStatus, poolInPlace)

					machineClassPool := copyMachineClass(defaultMachineClass)
					machineClassPool["operatingSystem"] = map[string]any{
						"operatingSystemName":    machineImageName,
						"operatingSystemVersion": strings.ReplaceAll(machineImageVersionSharedID, "+", "_"),
					}
					machineClassPool["image"] = map[string]any{
						"sharedGalleryImageID": machineImageSharedID,
					}
					machineClassPool["nodeTemplate"] = nodeTemplateZone4
					additionalData := []string{identityID}
					workerPoolHash, _ := worker.WorkerPoolHash(w.Spec.Pools[0], cluster, additionalData, additionalData, nil)
					machineClassNamePool := fmt.Sprintf("%s-%s", technicalID, poolInPlace.Name)
					machineClassWithHashPool := fmt.Sprintf("%s-%s-z%s", machineClassNamePool, workerPoolHash, zone1)
					machineDeploymentNamePool := fmt.Sprintf("%s-z%s", machineClassNamePool, zone1)
					machineDeployments = worker.MachineDeployments{
						{
							Name:       machineDeploymentNamePool,
							PoolName:   namePoolInPlace,
							ClassName:  machineClassWithHashPool,
							SecretName: machineClassWithHashPool,
							Minimum:    minPoolInPlace,
							Maximum:    maxPoolInPlace,
							Strategy: machinev1alpha1.MachineDeploymentStrategy{
								Type: machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType,
								InPlaceUpdate: &machinev1alpha1.InPlaceUpdateMachineDeployment{
									UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
										MaxSurge:       &maxSurgePoolInPlace,
										MaxUnavailable: &maxSurgePoolInPlace,
									},
									OrchestrationType: machinev1alpha1.OrchestrationTypeAuto,
								},
							},
							Labels:                       zone1Labels,
							MachineConfiguration:         &machinev1alpha1.MachineConfiguration{},
							ClusterAutoscalerAnnotations: emptyClusterAutoscalerAnnotations,
						},
					}
				})

				It("should return the correct machine deployments for zonal setup", func() {
					workerDelegate := wrapNewWorkerDelegate(c, chartApplier, w, cluster, nil)

					expectedUserDataSecretRefRead()

					// Test workerDelegate.GenerateMachineDeployments()
					result, err := workerDelegate.GenerateMachineDeployments(ctx)
					Expect(err).NotTo(HaveOccurred())

					// Expect the whole struct to be equal (just to be sure)
					Expect(result).To(Equal(machineDeployments))
				})

				It("should return error if there is worker pool with inplace update strategy and shoot uses VMSS Flex", func() {
					infrastructureStatus = makeInfrastructureStatus(resourceGroupName, vnetName, subnetName, false, &vnetResourceGroupName, &identityID)
					pool1.UpdateStrategy = ptr.To(gardencorev1beta1.AutoInPlaceUpdate)
					pool2.UpdateStrategy = ptr.To(gardencorev1beta1.ManualInPlaceUpdate)
					w = makeWorker(namespace, region, &sshKey, infrastructureStatus, pool1, pool2)

					workerDelegate := wrapNewWorkerDelegate(c, chartApplier, w, cluster, nil)

					expectedUserDataSecretRefRead()

					// Test workerDelegate.DeployMachineClasses()
					err := workerDelegate.DeployMachineClasses(ctx)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("worker pools with in-place update strategy is not supported when VMSS Flex are used"))
				})
			})

			It("should generate machine classes with same name even when virtualCapacity is newly added or changed", Label("virtualCapacity"), func() {
				capacityResources := corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("16Gi"),
				}
				virtualCapacityResources1 := corev1.ResourceList{
					"subdomain.domain.com/virtual-resource": resource.MustParse("1024"),
				}

				// Step 1: ProviderConfig with NodeTemplate.Capacity only (no VirtualCapacity)
				wc1 := apiv1alpha1.WorkerConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
						Kind:       "WorkerConfig",
					},
					NodeTemplate: &extensionsv1alpha1.NodeTemplate{
						Capacity: capacityResources,
					},
				}

				w1 := w.DeepCopy()
				w1.Spec.Pools = []extensionsv1alpha1.WorkerPool{w1.Spec.Pools[0]}
				w1.Spec.Pools[0].NodeAgentSecretName = ptr.To("dummy") // ensure WorkerPoolHashV2 is used
				w1.Spec.Pools[0].ProviderConfig = &runtime.RawExtension{Raw: encode(&wc1)}

				expectedUserDataSecretRefRead()

				wd1 := wrapNewWorkerDelegate(c, chartApplier, w1, cluster, nil)
				result1, err := wd1.GenerateMachineDeployments(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result1).NotTo(BeEmpty())
				className1 := result1[0].ClassName

				// verify nodeTemplate capacity is merged: pool (cpu:8,gpu:1,memory:128Gi) + providerConfig (cpu:4,memory:16Gi) = cpu:4,gpu:1,memory:16Gi
				expectedMergedCapacity := nodeCapacity.DeepCopy()
				maps.Copy(expectedMergedCapacity, capacityResources)

				// Step 2: Add VirtualCapacity to the same ProviderConfig
				wc2 := apiv1alpha1.WorkerConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
						Kind:       "WorkerConfig",
					},
					NodeTemplate: &extensionsv1alpha1.NodeTemplate{
						Capacity:        capacityResources,
						VirtualCapacity: virtualCapacityResources1,
					},
				}

				w2 := w.DeepCopy()
				w2.Spec.Pools = []extensionsv1alpha1.WorkerPool{w2.Spec.Pools[0]}
				w2.Spec.Pools[0].NodeAgentSecretName = ptr.To("dummy") // ensure WorkerPoolHashV2 is used
				w2.Spec.Pools[0].ProviderConfig = &runtime.RawExtension{Raw: encode(&wc2)}

				wd2 := wrapNewWorkerDelegate(c, chartApplier, w2, cluster, nil)
				result2, err := wd2.GenerateMachineDeployments(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result2).NotTo(BeEmpty())
				className2 := result2[0].ClassName

				// Hash should remain the same when VirtualCapacity is added
				Expect(className2).To(Equal(className1),
					fmt.Sprintf("hash should be stable after adding VirtualCapacity: className1=%q, className2=%q", className1, className2))

				// Step 3: Change VirtualCapacity value
				virtualCapacityResources2 := corev1.ResourceList{
					"subdomain.domain.com/virtual-resource": resource.MustParse("2048"),
				}
				wc3 := apiv1alpha1.WorkerConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
						Kind:       "WorkerConfig",
					},
					NodeTemplate: &extensionsv1alpha1.NodeTemplate{
						Capacity:        capacityResources,
						VirtualCapacity: virtualCapacityResources2,
					},
				}

				w3 := w.DeepCopy()
				w3.Spec.Pools = []extensionsv1alpha1.WorkerPool{w3.Spec.Pools[0]}
				w3.Spec.Pools[0].NodeAgentSecretName = ptr.To("dummy") // ensure WorkerPoolHashV2 is used
				w3.Spec.Pools[0].ProviderConfig = &runtime.RawExtension{Raw: encode(&wc3)}

				wd3 := wrapNewWorkerDelegate(c, chartApplier, w3, cluster, nil)
				result3, err := wd3.GenerateMachineDeployments(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result3).NotTo(BeEmpty())
				className3 := result3[0].ClassName

				// Hash should remain the same after changing VirtualCapacity
				Expect(className3).To(Equal(className1),
					fmt.Sprintf("hash should be stable after changing VirtualCapacity: className1=%q, className3=%q", className1, className3))
			})

			It("should generate machine classes with same name even when virtualCapacity is newly added or changed (k8s >= 1.34)", Label("virtualCapacity"), func() {
				capacityResources := corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("16Gi"),
				}

				// Step 1: ProviderConfig with NodeTemplate.Capacity only (no VirtualCapacity), k8s 1.34
				wc1 := apiv1alpha1.WorkerConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
						Kind:       "WorkerConfig",
					},
					NodeTemplate: &extensionsv1alpha1.NodeTemplate{
						Capacity: capacityResources,
					},
				}

				w1 := w.DeepCopy()
				w1.Spec.Pools = []extensionsv1alpha1.WorkerPool{w1.Spec.Pools[0]}
				w1.Spec.Pools[0].NodeAgentSecretName = ptr.To("dummy")
				w1.Spec.Pools[0].KubernetesVersion = ptr.To("1.34.0") // new hash data strategy
				w1.Spec.Pools[0].ProviderConfig = &runtime.RawExtension{Raw: encode(&wc1)}

				expectedUserDataSecretRefRead()

				wd1 := wrapNewWorkerDelegate(c, chartApplier, w1, cluster, nil)
				result1, err := wd1.GenerateMachineDeployments(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result1).NotTo(BeEmpty())
				className1 := result1[0].ClassName

				// Step 2: Add VirtualCapacity
				wc2 := apiv1alpha1.WorkerConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
						Kind:       "WorkerConfig",
					},
					NodeTemplate: &extensionsv1alpha1.NodeTemplate{
						Capacity: capacityResources,
						VirtualCapacity: corev1.ResourceList{
							"subdomain.domain.com/virtual-resource": resource.MustParse("1024"),
						},
					},
				}

				w2 := w.DeepCopy()
				w2.Spec.Pools = []extensionsv1alpha1.WorkerPool{w2.Spec.Pools[0]}
				w2.Spec.Pools[0].NodeAgentSecretName = ptr.To("dummy")
				w2.Spec.Pools[0].KubernetesVersion = ptr.To("1.34.0")
				w2.Spec.Pools[0].ProviderConfig = &runtime.RawExtension{Raw: encode(&wc2)}

				wd2 := wrapNewWorkerDelegate(c, chartApplier, w2, cluster, nil)
				result2, err := wd2.GenerateMachineDeployments(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result2).NotTo(BeEmpty())
				className2 := result2[0].ClassName

				Expect(className2).To(Equal(className1),
					fmt.Sprintf("k8s>=1.34: hash should be stable after adding VirtualCapacity: className1=%q, className2=%q", className1, className2))

				// Step 3: Change VirtualCapacity
				wc3 := apiv1alpha1.WorkerConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
						Kind:       "WorkerConfig",
					},
					NodeTemplate: &extensionsv1alpha1.NodeTemplate{
						Capacity: capacityResources,
						VirtualCapacity: corev1.ResourceList{
							"subdomain.domain.com/virtual-resource": resource.MustParse("2048"),
						},
					},
				}

				w3 := w.DeepCopy()
				w3.Spec.Pools = []extensionsv1alpha1.WorkerPool{w3.Spec.Pools[0]}
				w3.Spec.Pools[0].NodeAgentSecretName = ptr.To("dummy")
				w3.Spec.Pools[0].KubernetesVersion = ptr.To("1.34.0")
				w3.Spec.Pools[0].ProviderConfig = &runtime.RawExtension{Raw: encode(&wc3)}

				wd3 := wrapNewWorkerDelegate(c, chartApplier, w3, cluster, nil)
				result3, err := wd3.GenerateMachineDeployments(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result3).NotTo(BeEmpty())
				className3 := result3[0].ClassName

				Expect(className3).To(Equal(className1),
					fmt.Sprintf("k8s>=1.34: hash should be stable after changing VirtualCapacity: className1=%q, className3=%q", className1, className3))
			})

			It("should fail because the version is invalid", func() {
				cluster = makeCluster(technicalID, "invalid", region, nil, machineImages, 0)
				workerDelegate := wrapNewWorkerDelegate(c, chartApplier, w, cluster, nil)

				result, err := workerDelegate.GenerateMachineDeployments(ctx)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the machine image for given architecture cannot be found", func() {
				w.Spec.Pools[0].Architecture = ptr.To("arm64")
				workerDelegate := wrapNewWorkerDelegate(c, chartApplier, w, cluster, nil)

				result, err := workerDelegate.GenerateMachineDeployments(ctx)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the infrastructure status cannot be decoded", func() {
				w.Spec.InfrastructureProviderStatus = &runtime.RawExtension{Raw: []byte("definitely not correct")}
				workerDelegate := wrapNewWorkerDelegate(c, chartApplier, w, cluster, nil)

				result, err := workerDelegate.GenerateMachineDeployments(ctx)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the nodes subnet cannot be found", func() {
				w.Spec.InfrastructureProviderStatus = &runtime.RawExtension{
					Raw: encode(&apisazure.InfrastructureStatus{}),
				}
				workerDelegate := wrapNewWorkerDelegate(c, chartApplier, w, cluster, nil)

				result, err := workerDelegate.GenerateMachineDeployments(ctx)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the machine image information cannot be found", func() {
				cluster = makeCluster(technicalID, shootVersion, region, machineTypes, nil, 0)
				workerDelegate := wrapNewWorkerDelegate(c, chartApplier, w, cluster, nil)

				result, err := workerDelegate.GenerateMachineDeployments(ctx)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the volume size cannot be decoded", func() {
				w.Spec.Pools[0].Volume.Size = "not-decodeable"
				workerDelegate := wrapNewWorkerDelegate(c, chartApplier, w, cluster, nil)

				result, err := workerDelegate.GenerateMachineDeployments(ctx)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should set expected machineControllerManager settings on machine deployment", func() {
				var (
					testDrainTimeout    = metav1.Duration{Duration: 10 * time.Minute}
					testHealthTimeout   = metav1.Duration{Duration: 20 * time.Minute}
					testCreationTimeout = metav1.Duration{Duration: 30 * time.Minute}
					testMaxEvictRetries = int32(30)
					testNodeConditions  = []string{"ReadonlyFilesystem", "KernelDeadlock", "DiskPressure"}
				)
				w.Spec.Pools[0].MachineControllerManagerSettings = &gardencorev1beta1.MachineControllerManagerSettings{
					MachineDrainTimeout:    &testDrainTimeout,
					MachineCreationTimeout: &testCreationTimeout,
					MachineHealthTimeout:   &testHealthTimeout,
					MaxEvictRetries:        &testMaxEvictRetries,
					NodeConditions:         testNodeConditions,
				}
				workerDelegate := wrapNewWorkerDelegate(c, chartApplier, w, cluster, nil)

				expectedUserDataSecretRefRead()

				result, err := workerDelegate.GenerateMachineDeployments(ctx)
				Expect(err).NotTo(HaveOccurred())

				Expect(result).NotTo(BeNil())
				resultSettings := result[0].MachineConfiguration
				resultNodeConditions := strings.Join(testNodeConditions, ",")

				Expect(resultSettings.MachineDrainTimeout).To(Equal(&testDrainTimeout))
				Expect(resultSettings.MachineCreationTimeout).To(Equal(&testCreationTimeout))
				Expect(resultSettings.MachineHealthTimeout).To(Equal(&testHealthTimeout))
				Expect(resultSettings.MaxEvictRetries).To(Equal(&testMaxEvictRetries))
				Expect(resultSettings.NodeConditions).To(Equal(&resultNodeConditions))
			})
		})
	})

	Describe("sanitize azure vm tag", func() {
		It("not include restricted characters", func() {
			Expect(SanitizeAzureVMTag("<>%\\&?/a ")).To(Equal("_______a_"))
		})
	})
})

func copyMachineClass(def map[string]any) map[string]any {
	out := make(map[string]any, len(def))

	for k, v := range def {
		out[k] = v
	}

	return out
}

func addNameAndSecretsToMachineClass(class map[string]any, name string, credentialsSecretRef corev1.SecretReference) {
	class["name"] = name
	class["credentialsSecretRef"] = map[string]any{
		"name":      credentialsSecretRef.Name,
		"namespace": credentialsSecretRef.Namespace,
	}
	class["labels"] = map[string]string{
		v1beta1constants.GardenerPurpose: v1beta1constants.GardenPurposeMachineClass,
	}
}
