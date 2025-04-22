// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

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
	. "github.com/gardener/gardener-extension-provider-azure/pkg/controller/worker"
)

var _ = Describe("Machines", func() {
	var (
		ctx = context.Background()

		ctrl         *gomock.Controller
		c            *mockclient.MockClient
		statusWriter *mockclient.MockStatusWriter
		chartApplier *mockkubernetes.MockChartApplier

		namespace, region string
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)
		chartApplier = mockkubernetes.NewMockChartApplier(ctrl)
		statusWriter = mockclient.NewMockStatusWriter(ctrl)

		// Let the seed client always the mocked status writer when Status() is called.
		c.EXPECT().Status().AnyTimes().Return(statusWriter)

		namespace = "shoot--foobar--azure"
		region = "westeurope"
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("workerDelegate", func() {
		Describe("#GenerateMachineDeployments, #DeployMachineClasses", func() {
			const azureCSIDiskDriverTopologyKey = "topology.disk.csi.azure.com/zone"

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
				availabilitySetID     string
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

				namePoolZones           string
				minPoolZones            int32
				maxPoolZones            int32
				maxSurgePoolZones       intstr.IntOrString
				maxUnavailablePoolZones intstr.IntOrString

				labels map[string]string

				nodeCapacity      corev1.ResourceList
				nodeTemplateZone1 machinev1alpha1.NodeTemplate
				nodeTemplateZone2 machinev1alpha1.NodeTemplate
				nodeTemplateZone3 machinev1alpha1.NodeTemplate
				nodeTemplateZone4 machinev1alpha1.NodeTemplate

				archAMD string
				archARM string

				diagnosticProfile apiv1alpha1.DiagnosticsProfile
				providerConfig    *runtime.RawExtension
				workerConfig      apiv1alpha1.WorkerConfig

				shootVersionMajorMinor string
				shootVersion           string

				machineImages []apiv1alpha1.MachineImages
				machineTypes  []apiv1alpha1.MachineType

				pool1, pool2, pool3, pool4, poolZones extensionsv1alpha1.WorkerPool
				infrastructureStatus                  *apisazure.InfrastructureStatus
				w                                     *extensionsv1alpha1.Worker
				cluster                               *extensionscontroller.Cluster
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
				availabilitySetID = "av-1234"
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
				maxUnavailablePool1 = intstr.FromInt(2)

				labels = map[string]string{"component": "TiDB"}

				nodeCapacity = corev1.ResourceList{
					"cpu":    resource.MustParse("8"),
					"gpu":    resource.MustParse("1"),
					"memory": resource.MustParse("128Gi"),
				}

				archAMD = "amd64"
				archARM = "arm64"

				nodeTemplateZone1 = machinev1alpha1.NodeTemplate{
					Capacity:     nodeCapacity,
					InstanceType: machineType,
					Region:       region,
					Zone:         "no-zone",
					Architecture: ptr.To(archAMD),
				}

				nodeTemplateZone2 = machinev1alpha1.NodeTemplate{
					Capacity:     nodeCapacity,
					InstanceType: machineType,
					Region:       region,
					Zone:         "no-zone",
					Architecture: ptr.To(archAMD),
				}

				nodeTemplateZone3 = machinev1alpha1.NodeTemplate{
					Capacity:     nodeCapacity,
					InstanceType: machineType,
					Region:       region,
					Zone:         "no-zone",
					Architecture: ptr.To(archARM),
				}

				nodeTemplateZone4 = machinev1alpha1.NodeTemplate{
					Capacity:     nodeCapacity,
					InstanceType: machineType,
					Region:       region,
					Zone:         "no-zone",
					Architecture: ptr.To(archARM),
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
					DiagnosticsProfile: &diagnosticProfile,
				}

				marshalledWorkerConfig, err := json.Marshal(workerConfig)
				Expect(err).ToNot(HaveOccurred())
				providerConfig = &runtime.RawExtension{
					Raw: marshalledWorkerConfig,
				}

				namePool2 = "pool-zones"
				minPool2 = 30
				maxPool2 = 45
				priorityPool2 = 100
				maxSurgePool2 = intstr.FromInt(10)
				maxUnavailablePool2 = intstr.FromInt(15)

				namePool3 = "pool-3"
				minPool3 = 1
				maxPool3 = 5
				maxSurgePool3 = intstr.FromInt(2)
				maxUnavailablePool3 = intstr.FromInt(2)

				namePool4 = "pool-4"
				minPool4 = 2
				maxPool4 = 6
				maxSurgePool4 = intstr.FromInt(1)
				maxUnavailablePool4 = intstr.FromInt(2)

				shootVersionMajorMinor = "1.32"
				shootVersion = shootVersionMajorMinor + ".0"

				machineImages = []apiv1alpha1.MachineImages{
					{
						Name: machineImageName,
						Versions: []apiv1alpha1.MachineImageVersion{
							{
								Version:               machineImageVersion,
								URN:                   &machineImageURN,
								AcceleratedNetworking: ptr.To(true),
							},
							{
								Version: machineImageVersionID,
								ID:      &machineImageID,
							},
							{
								Version:                 machineImageVersionCommunityID,
								CommunityGalleryImageID: &machineImageCommunityID,
								Architecture:            ptr.To(archARM),
							},
							{
								Version:              machineImageVersionSharedID,
								SharedGalleryImageID: &machineImageSharedID,
								Architecture:         ptr.To(archARM),
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

				pool1 = extensionsv1alpha1.WorkerPool{
					Name:           namePool1,
					Minimum:        minPool1,
					Maximum:        maxPool1,
					MaxSurge:       maxSurgePool1,
					MaxUnavailable: maxUnavailablePool1,
					Architecture:   ptr.To(archAMD),
					MachineType:    machineType,
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
					Labels:         labels,
					ProviderConfig: providerConfig,
				}

				pool2 = extensionsv1alpha1.WorkerPool{
					Name:           namePool2,
					Minimum:        minPool2,
					Maximum:        maxPool2,
					Priority:       ptr.To(priorityPool2),
					MaxSurge:       maxSurgePool2,
					Architecture:   ptr.To(archAMD),
					MaxUnavailable: maxUnavailablePool2,
					MachineType:    machineType,
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
					Labels: labels,
				}

				pool3 = extensionsv1alpha1.WorkerPool{
					Name:           namePool3,
					Minimum:        minPool3,
					Maximum:        maxPool3,
					MaxSurge:       maxSurgePool3,
					Architecture:   ptr.To(archARM),
					MaxUnavailable: maxUnavailablePool3,
					MachineType:    machineType,
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
					Labels: labels,
				}

				pool4 = extensionsv1alpha1.WorkerPool{
					Name:           namePool4,
					Minimum:        minPool4,
					Maximum:        maxPool4,
					MaxSurge:       maxSurgePool4,
					Architecture:   ptr.To(archARM),
					MaxUnavailable: maxUnavailablePool4,
					MachineType:    machineType,
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
					Labels: labels,
				}

				cluster = makeCluster(shootVersion, region, machineTypes, machineImages, 0)
				infrastructureStatus = makeInfrastructureStatus(resourceGroupName, vnetName, subnetName, false, &vnetResourceGroupName, &availabilitySetID, &identityID)
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
					urnMachineClass                     map[string]interface{}
					imageIDMachineClass                 map[string]interface{}
					communityGalleryImageIDMachineClass map[string]interface{}
					sharedGalleryImageIDMachineClass    map[string]interface{}
					machineDeployments                  worker.MachineDeployments
					machineClasses                      map[string]interface{}

					workerPoolHash1, workerPoolHash2, workerPoolHash3, workerPoolHash4 string

					zone1   = "1"
					zone2   = "2"
					subnet1 = "subnet1"
					subnet2 = "subnet2"
				)

				BeforeEach(func() {
					vmTags := map[string]string{
						"Name": namespace,
						SanitizeAzureVMTag(fmt.Sprintf("kubernetes.io-cluster-%s", namespace)): "1",
						SanitizeAzureVMTag("kubernetes.io-role-node"):                          "1",
					}
					for k, v := range labels {
						vmTags[SanitizeAzureVMTag(k)] = v
					}

					defaultMachineClass := map[string]interface{}{
						"region":        region,
						"resourceGroup": resourceGroupName,
						"network": map[string]interface{}{
							"vnet":              vnetName,
							"subnet":            subnetName,
							"vnetResourceGroup": vnetResourceGroupName,
						},
						"machineSet": map[string]interface{}{
							"id":   availabilitySetID,
							"kind": "availabilityset",
						},
						"tags": vmTags,
						"secret": map[string]interface{}{
							"cloudConfig": string(userData),
						},
						"machineType": machineType,
						"osDisk": map[string]interface{}{
							"size": volumeSize,
						},
						"sshPublicKey": sshKey,
						"identityID":   identityID,
						"cloudConfiguration": map[string]interface{}{
							"name": apisazure.AzurePublicCloudName,
						},
					}

					urnMachineClass = copyMachineClass(defaultMachineClass)
					urnMachineClass["image"] = map[string]interface{}{
						"urn": machineImageURN,
					}

					imageIDMachineClass = copyMachineClass(defaultMachineClass)
					imageIDMachineClass["image"] = map[string]interface{}{
						"id": machineImageID,
					}

					communityGalleryImageIDMachineClass = copyMachineClass(defaultMachineClass)
					communityGalleryImageIDMachineClass["image"] = map[string]interface{}{
						"communityGalleryImageID": machineImageCommunityID,
					}

					sharedGalleryImageIDMachineClass = copyMachineClass(defaultMachineClass)
					sharedGalleryImageIDMachineClass["image"] = map[string]interface{}{
						"sharedGalleryImageID": machineImageSharedID,
					}

					workerPoolHash1AdditionalData := []string{fmt.Sprintf("%dGi", dataVolume2Size), dataVolume2Type, fmt.Sprintf("%dGi", dataVolume1Size), identityID}
					additionalData := []string{identityID}

					workerPoolHash1, _ = worker.WorkerPoolHash(w.Spec.Pools[0], cluster, workerPoolHash1AdditionalData, workerPoolHash1AdditionalData)
					workerPoolHash2, _ = worker.WorkerPoolHash(w.Spec.Pools[1], cluster, additionalData, additionalData)
					workerPoolHash3, _ = worker.WorkerPoolHash(w.Spec.Pools[2], cluster, additionalData, additionalData)
					workerPoolHash4, _ = worker.WorkerPoolHash(w.Spec.Pools[3], cluster, additionalData, additionalData)

					var (
						machineClassPool1 = copyMachineClass(urnMachineClass)
						machineClassPool2 = copyMachineClass(imageIDMachineClass)
						machineClassPool3 = copyMachineClass(communityGalleryImageIDMachineClass)
						machineClassPool4 = copyMachineClass(sharedGalleryImageIDMachineClass)

						machineClassNamePool1 = fmt.Sprintf("%s-%s", namespace, namePool1)
						machineClassNamePool2 = fmt.Sprintf("%s-%s", namespace, namePool2)
						machineClassNamePool3 = fmt.Sprintf("%s-%s", namespace, namePool3)
						machineClassNamePool4 = fmt.Sprintf("%s-%s", namespace, namePool4)

						machineClassWithHashPool1 = fmt.Sprintf("%s-%s", machineClassNamePool1, workerPoolHash1)
						machineClassWithHashPool2 = fmt.Sprintf("%s-%s", machineClassNamePool2, workerPoolHash2)
						machineClassWithHashPool3 = fmt.Sprintf("%s-%s", machineClassNamePool3, workerPoolHash3)
						machineClassWithHashPool4 = fmt.Sprintf("%s-%s", machineClassNamePool4, workerPoolHash4)
					)

					addNameAndSecretsToMachineClass(machineClassPool1, machineClassWithHashPool1, w.Spec.SecretRef)
					addNameAndSecretsToMachineClass(machineClassPool2, machineClassWithHashPool2, w.Spec.SecretRef)
					addNameAndSecretsToMachineClass(machineClassPool3, machineClassWithHashPool3, w.Spec.SecretRef)
					addNameAndSecretsToMachineClass(machineClassPool4, machineClassWithHashPool4, w.Spec.SecretRef)

					machineClassPool1["nodeTemplate"] = nodeTemplateZone1
					machineClassPool2["nodeTemplate"] = nodeTemplateZone2
					machineClassPool3["nodeTemplate"] = nodeTemplateZone3
					machineClassPool4["nodeTemplate"] = nodeTemplateZone4

					machineClassPool1["diagnosticsProfile"] = map[string]interface{}{
						"enabled":    diagnosticProfile.Enabled,
						"storageURI": diagnosticProfile.StorageURI,
					}

					machineClassPool1["dataDisks"] = []map[string]interface{}{
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
					machineClassPool2["osDisk"] = map[string]interface{}{
						"size": volumeSize,
						"type": volumeType,
					}
					machineClassPool3["osDisk"] = map[string]interface{}{
						"size": volumeSize,
						"type": volumeType,
					}
					machineClassPool4["osDisk"] = map[string]interface{}{
						"size": volumeSize,
						"type": volumeType,
					}

					machineClassPool1["operatingSystem"] = map[string]interface{}{
						"operatingSystemName":    machineImageName,
						"operatingSystemVersion": strings.ReplaceAll(machineImageVersion, "+", "_"),
					}
					machineClassPool2["operatingSystem"] = map[string]interface{}{
						"operatingSystemName":    machineImageName,
						"operatingSystemVersion": strings.ReplaceAll(machineImageVersionID, "+", "_"),
					}
					machineClassPool3["operatingSystem"] = map[string]interface{}{
						"operatingSystemName":    machineImageName,
						"operatingSystemVersion": strings.ReplaceAll(machineImageVersionCommunityID, "+", "_"),
					}
					machineClassPool4["operatingSystem"] = map[string]interface{}{
						"operatingSystemName":    machineImageName,
						"operatingSystemVersion": strings.ReplaceAll(machineImageVersionSharedID, "+", "_"),
					}

					machineClasses = map[string]interface{}{"machineClasses": []map[string]interface{}{
						machineClassPool1,
						machineClassPool2,
						machineClassPool3,
						machineClassPool4,
					}}

					machineDeployments = worker.MachineDeployments{
						{
							Name:       machineClassNamePool1,
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
							Labels:               labels,
							MachineConfiguration: &machinev1alpha1.MachineConfiguration{},
						},
						{
							Name:       machineClassNamePool2,
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
							Labels:               labels,
							MachineConfiguration: &machinev1alpha1.MachineConfiguration{},
						},
						{
							Name:       machineClassNamePool3,
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
							Labels:               labels,
							MachineConfiguration: &machinev1alpha1.MachineConfiguration{},
						},
						{
							Name:       machineClassNamePool4,
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
							Labels:               labels,
							MachineConfiguration: &machinev1alpha1.MachineConfiguration{},
						},
					}
				})

				It("should return the expected machine deployments for profile image types", func() {
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

					// Test workerDelegate.UpdateMachineImagesStatus()
					expectWorkerProviderStatusUpdateToSucceed(ctx, statusWriter)
					err = workerDelegate.UpdateMachineImagesStatus(ctx)
					Expect(err).NotTo(HaveOccurred())

					// Test workerDelegate.GenerateMachineDeployments()
					result, err := workerDelegate.GenerateMachineDeployments(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(machineDeployments))
				})

				Describe("#Zonal setup", func() {
					var (
						w                *extensionsv1alpha1.Worker
						workerPoolHashZ1 string
						workerPoolHashZ2 string

						machineClassNamePool1 string
						machineClassNamePool2 string

						machineClassWithHashPool1 string
						machineClassWithHashPool2 string
						volumeSize                int
					)

					BeforeEach(func() {
						volumeSize = 20

						infrastructureStatus = makeInfrastructureStatus(resourceGroupName, vnetName, subnetName, true, &vnetResourceGroupName, &availabilitySetID, &identityID)
						infrastructureStatus.Networks = apisazure.NetworkStatus{
							Layout: apisazure.NetworkLayoutMultipleSubnet,
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

						poolZones = extensionsv1alpha1.WorkerPool{
							Name:           namePoolZones,
							Minimum:        minPoolZones,
							Maximum:        maxPoolZones,
							MaxSurge:       maxSurgePoolZones,
							MaxUnavailable: maxUnavailablePoolZones,
							MachineType:    machineType,
							MachineImage: extensionsv1alpha1.MachineImage{
								Name:    machineImageName,
								Version: machineImageVersionID,
							},
							Volume: &extensionsv1alpha1.Volume{
								Size: fmt.Sprintf("%dGi", volumeSize),
							},
							UserDataSecretRef: corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: userDataSecretName},
								Key:                  userDataSecretDataKey,
							},
							Labels: labels,
							Zones:  []string{zone1, zone2},
						}

						w = makeWorker(namespace, region, &sshKey, infrastructureStatus, poolZones)

						additionalHashDataZ1 := []string{identityID}
						additionalHashDataZ2 := []string{identityID, subnet2}
						workerPoolHashZ1, _ = worker.WorkerPoolHash(w.Spec.Pools[0], cluster, additionalHashDataZ1, additionalHashDataZ1)
						workerPoolHashZ2, _ = worker.WorkerPoolHash(w.Spec.Pools[0], cluster, additionalHashDataZ2, additionalHashDataZ2)

						basename := fmt.Sprintf("%s-%s", namespace, namePoolZones)
						machineClassNamePool1 = fmt.Sprintf("%s-z%s", basename, zone1)
						machineClassNamePool2 = fmt.Sprintf("%s-z%s", basename, zone2)
						machineClassWithHashPool1 = fmt.Sprintf("%s-%s-z%s", basename, workerPoolHashZ1, zone1)
						machineClassWithHashPool2 = fmt.Sprintf("%s-%s-z%s", basename, workerPoolHashZ2, zone2)
						labelsPool1 := utils.MergeStringMaps(labels, map[string]string{azureCSIDiskDriverTopologyKey: region + "-" + zone1})
						labelsPool2 := utils.MergeStringMaps(labels, map[string]string{azureCSIDiskDriverTopologyKey: region + "-" + zone2})

						machineDeployments = worker.MachineDeployments{
							{
								Name:       machineClassNamePool1,
								ClassName:  machineClassWithHashPool1,
								SecretName: machineClassWithHashPool1,
								Minimum:    minPoolZones,
								Maximum:    maxPoolZones,
								Strategy: machinev1alpha1.MachineDeploymentStrategy{
									Type: machinev1alpha1.RollingUpdateMachineDeploymentStrategyType,
									RollingUpdate: &machinev1alpha1.RollingUpdateMachineDeployment{
										UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
											MaxSurge:       &maxSurgePoolZones,
											MaxUnavailable: &maxUnavailablePoolZones,
										},
									},
								},
								Labels:               labelsPool1,
								MachineConfiguration: &machinev1alpha1.MachineConfiguration{},
							},
							{
								Name:       machineClassNamePool2,
								ClassName:  machineClassWithHashPool2,
								SecretName: machineClassWithHashPool2,
								Minimum:    minPoolZones,
								Maximum:    maxPoolZones,
								Strategy: machinev1alpha1.MachineDeploymentStrategy{
									Type: machinev1alpha1.RollingUpdateMachineDeploymentStrategyType,
									RollingUpdate: &machinev1alpha1.RollingUpdateMachineDeployment{
										UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
											MaxSurge:       &maxSurgePoolZones,
											MaxUnavailable: &maxUnavailablePoolZones,
										},
									},
								},
								Labels:               labelsPool2,
								MachineConfiguration: &machinev1alpha1.MachineConfiguration{},
							},
						}
					})

					It("should return the correct machine deployments for zonal setup", func() {
						workerDelegate := wrapNewWorkerDelegate(c, chartApplier, w, cluster, nil)

						expectedUserDataSecretRefRead()

						// Test workerDelegate.GenerateMachineDeployments()
						result, err := workerDelegate.GenerateMachineDeployments(ctx)
						Expect(err).NotTo(HaveOccurred())
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
						Expect(result[2].ClusterAutoscalerAnnotations).To(BeNil())
						Expect(result[3].ClusterAutoscalerAnnotations).To(BeNil())

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
			})

			It("should fail because the version is invalid", func() {
				cluster = makeCluster("invalid", region, nil, nil, 0)
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

			It("should fail because the nodes availability set cannot be found", func() {
				w.Spec.InfrastructureProviderStatus = &runtime.RawExtension{
					Raw: encode(&apisazure.InfrastructureStatus{
						Networks: apisazure.NetworkStatus{
							Subnets: []apisazure.Subnet{
								{
									Purpose: apisazure.PurposeNodes,
									Name:    subnetName,
								},
							},
						},
						AvailabilitySets: []apisazure.AvailabilitySet{
							{Purpose: "not-nodes"},
						},
					}),
				}
				workerDelegate := wrapNewWorkerDelegate(c, chartApplier, w, cluster, nil)

				expectedUserDataSecretRefRead()

				result, err := workerDelegate.GenerateMachineDeployments(ctx)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the machine image information cannot be found", func() {
				cluster = makeCluster(shootVersion, region, nil, nil, 0)
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
				resultSettings := result[0].MachineConfiguration
				resultNodeConditions := strings.Join(testNodeConditions, ",")

				Expect(err).NotTo(HaveOccurred())
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

func copyMachineClass(def map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(def))

	for k, v := range def {
		out[k] = v
	}

	return out
}

func addNameAndSecretsToMachineClass(class map[string]interface{}, name string, credentialsSecretRef corev1.SecretReference) {
	class["name"] = name
	class["credentialsSecretRef"] = map[string]interface{}{
		"name":      credentialsSecretRef.Name,
		"namespace": credentialsSecretRef.Namespace,
	}
	class["labels"] = map[string]string{
		v1beta1constants.GardenerPurpose: v1beta1constants.GardenPurposeMachineClass,
	}
}
