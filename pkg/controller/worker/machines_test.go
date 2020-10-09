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

package worker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/controller/worker"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/common"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	genericworkeractuator "github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mockkubernetes "github.com/gardener/gardener/pkg/mock/gardener/client/kubernetes"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Machines", func() {
	var (
		ctrl         *gomock.Controller
		c            *mockclient.MockClient
		statusWriter *mockclient.MockStatusWriter
		chartApplier *mockkubernetes.MockChartApplier
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)
		chartApplier = mockkubernetes.NewMockChartApplier(ctrl)
		statusWriter = mockclient.NewMockStatusWriter(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("workerDelegate", func() {
		workerDelegate, _ := NewWorkerDelegate(common.NewClientContext(nil, nil, nil), nil, "", nil, nil)

		Describe("#MachineClassKind", func() {
			It("should return the correct kind of the machine class", func() {
				Expect(workerDelegate.MachineClassKind()).To(Equal("AzureMachineClass"))
			})
		})

		Describe("#MachineClassList", func() {
			It("should return the correct type for the machine class list", func() {
				Expect(workerDelegate.MachineClassList()).To(Equal(&machinev1alpha1.AzureMachineClassList{}))
			})
		})

		Describe("#GenerateMachineDeployments, #DeployMachineClasses", func() {
			var (
				namespace        string
				cloudProfileName string

				azureClientID       string
				azureClientSecret   string
				azureSubscriptionID string
				azureTenantID       string
				region              string

				machineImageName      string
				machineImageVersion   string
				machineImageVersionID string
				machineImageURN       string
				machineImageID        string

				resourceGroupName     string
				vnetResourceGroupName string
				vnetName              string
				subnetName            string
				availabilitySetID     string
				identityID            string
				machineType           string
				userData              []byte
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
				maxSurgePool2       intstr.IntOrString
				maxUnavailablePool2 intstr.IntOrString

				workerPoolHash1 string
				workerPoolHash2 string

				labels map[string]string

				machineConfiguration *machinev1alpha1.MachineConfiguration

				shootVersionMajorMinor string
				shootVersion           string
				scheme                 *runtime.Scheme
				decoder                runtime.Decoder
				clusterWithoutImages   *extensionscontroller.Cluster
				cluster                *extensionscontroller.Cluster
				w                      *extensionsv1alpha1.Worker

				boolTrue = true
			)

			BeforeEach(func() {
				namespace = "shoot--foobar--azure"
				cloudProfileName = "azure"

				region = "westeurope"
				azureClientID = "client-id"
				azureClientSecret = "client-secret"
				azureSubscriptionID = "1234"
				azureTenantID = "1234"

				machineImageName = "my-os"
				machineImageVersion = "1"
				machineImageVersionID = "2"
				machineImageURN = "bar:baz:foo:123"
				machineImageID = "/shared/image/gallery/image/id"

				resourceGroupName = "my-rg"
				vnetResourceGroupName = "my-vnet-rg"
				vnetName = "my-vnet"
				subnetName = "subnet-1234"
				availabilitySetID = "av-1234"
				machineType = "large"
				userData = []byte("some-user-data")
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
				machineConfiguration = &machinev1alpha1.MachineConfiguration{}

				namePool2 = "pool-2"
				minPool2 = 30
				maxPool2 = 45
				maxSurgePool2 = intstr.FromInt(10)
				maxUnavailablePool2 = intstr.FromInt(15)

				shootVersionMajorMinor = "1.2"
				shootVersion = shootVersionMajorMinor + ".3"

				clusterWithoutImages = &extensionscontroller.Cluster{
					Shoot: &gardencorev1beta1.Shoot{
						Spec: gardencorev1beta1.ShootSpec{
							Kubernetes: gardencorev1beta1.Kubernetes{
								Version: shootVersion,
							},
						},
					},
				}
				cloudProfileConfig := &apiv1alpha1.CloudProfileConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
						Kind:       "CloudProfileConfig",
					},
					MachineImages: []apiv1alpha1.MachineImages{
						{
							Name: machineImageName,
							Versions: []apiv1alpha1.MachineImageVersion{
								{
									Version:               machineImageVersion,
									URN:                   &machineImageURN,
									AcceleratedNetworking: &boolTrue,
								},
								{
									Version: machineImageVersionID,
									ID:      &machineImageID,
								},
							},
						},
					},
					MachineTypes: []apiv1alpha1.MachineType{
						{
							Name:                  machineType,
							AcceleratedNetworking: &boolTrue,
						},
					},
				}
				cloudProfileConfigJSON, _ := json.Marshal(cloudProfileConfig)
				cluster = &extensionscontroller.Cluster{
					CloudProfile: &gardencorev1beta1.CloudProfile{
						ObjectMeta: metav1.ObjectMeta{
							Name: cloudProfileName,
						},
						Spec: gardencorev1beta1.CloudProfileSpec{
							ProviderConfig: &runtime.RawExtension{
								Raw: cloudProfileConfigJSON,
							},
						},
					},
					Shoot: clusterWithoutImages.Shoot,
				}

				w = &extensionsv1alpha1.Worker{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
					},
					Spec: extensionsv1alpha1.WorkerSpec{
						SecretRef: corev1.SecretReference{
							Name:      "secret",
							Namespace: namespace,
						},
						Region:       region,
						SSHPublicKey: []byte(sshKey),
						InfrastructureProviderStatus: &runtime.RawExtension{
							Raw: encode(&apisazure.InfrastructureStatus{
								ResourceGroup: apisazure.ResourceGroup{
									Name: resourceGroupName,
								},
								Networks: apisazure.NetworkStatus{
									VNet: apisazure.VNetStatus{
										Name:          vnetName,
										ResourceGroup: &vnetResourceGroupName,
									},
									Subnets: []apisazure.Subnet{
										{
											Purpose: apisazure.PurposeNodes,
											Name:    subnetName,
										},
									},
								},
								AvailabilitySets: []apisazure.AvailabilitySet{
									{
										Purpose: apisazure.PurposeNodes,
										ID:      availabilitySetID,
									},
								},
								Identity: &apisazure.IdentityStatus{
									ID: identityID,
								},
							}),
						},
						Pools: []extensionsv1alpha1.WorkerPool{
							{
								Name:           namePool1,
								Minimum:        minPool1,
								Maximum:        maxPool1,
								MaxSurge:       maxSurgePool1,
								MaxUnavailable: maxUnavailablePool1,
								MachineType:    machineType,
								MachineImage: extensionsv1alpha1.MachineImage{
									Name:    machineImageName,
									Version: machineImageVersion,
								},
								UserData: userData,
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
								Labels: labels,
							},
							{
								Name:           namePool2,
								Minimum:        minPool2,
								Maximum:        maxPool2,
								MaxSurge:       maxSurgePool2,
								MaxUnavailable: maxUnavailablePool2,
								MachineType:    machineType,
								MachineImage: extensionsv1alpha1.MachineImage{
									Name:    machineImageName,
									Version: machineImageVersionID,
								},
								UserData: userData,
								Volume: &extensionsv1alpha1.Volume{
									Size: fmt.Sprintf("%dGi", volumeSize),
									Type: &volumeType,
								},
								Labels: labels,
							},
						},
					},
				}

				scheme = runtime.NewScheme()
				_ = apisazure.AddToScheme(scheme)
				_ = apiv1alpha1.AddToScheme(scheme)
				decoder = serializer.NewCodecFactory(scheme).UniversalDecoder()

				workerPoolHash1, _ = worker.WorkerPoolHash(w.Spec.Pools[0], cluster, identityID, fmt.Sprintf("%dGi", dataVolume1Size), fmt.Sprintf("%dGi", dataVolume2Size), dataVolume2Type)
				workerPoolHash2, _ = worker.WorkerPoolHash(w.Spec.Pools[1], cluster, identityID)

				workerDelegate, _ = NewWorkerDelegate(common.NewClientContext(c, scheme, decoder), chartApplier, "", w, clusterWithoutImages)
			})

			Describe("machine images", func() {
				var (
					urnMachineClass     map[string]interface{}
					imageIdMachineClass map[string]interface{}
					machineDeployments  worker.MachineDeployments
					machineClasses      map[string]interface{}
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
						"availabilitySetID": availabilitySetID,
						"tags":              vmTags,
						"secret": map[string]interface{}{
							"cloudConfig": string(userData),
						},
						"machineType": machineType,
						"osDisk": map[string]interface{}{
							"size": volumeSize,
						},
						"sshPublicKey": sshKey,
						"identityID":   identityID,
					}

					urnMachineClass = copyMachineClass(defaultMachineClass)
					urnMachineClass["image"] = map[string]interface{}{
						"urn": machineImageURN,
					}

					imageIdMachineClass = copyMachineClass(defaultMachineClass)
					imageIdMachineClass["image"] = map[string]interface{}{
						"id": machineImageID,
					}

					var (
						machineClassPool1 = copyMachineClass(urnMachineClass)
						machineClassPool2 = copyMachineClass(imageIdMachineClass)

						machineClassNamePool1 = fmt.Sprintf("%s-%s", namespace, namePool1)
						machineClassNamePool2 = fmt.Sprintf("%s-%s", namespace, namePool2)

						machineClassWithHashPool1 = fmt.Sprintf("%s-%s", machineClassNamePool1, workerPoolHash1)
						machineClassWithHashPool2 = fmt.Sprintf("%s-%s", machineClassNamePool2, workerPoolHash2)
					)

					addNameAndSecretsToMachineClass(machineClassPool1, azureClientID, azureClientSecret, azureSubscriptionID, azureTenantID, machineClassWithHashPool1)
					addNameAndSecretsToMachineClass(machineClassPool2, azureClientID, azureClientSecret, azureSubscriptionID, azureTenantID, machineClassWithHashPool2)

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

					machineClasses = map[string]interface{}{"machineClasses": []map[string]interface{}{
						machineClassPool1,
						machineClassPool2,
					}}

					machineDeployments = worker.MachineDeployments{
						{
							Name:                 machineClassNamePool1,
							ClassName:            machineClassWithHashPool1,
							SecretName:           machineClassWithHashPool1,
							Minimum:              minPool1,
							Maximum:              maxPool1,
							MaxSurge:             maxSurgePool1,
							MaxUnavailable:       maxUnavailablePool1,
							Labels:               labels,
							MachineConfiguration: machineConfiguration,
						},
						{
							Name:                 machineClassNamePool2,
							ClassName:            machineClassWithHashPool2,
							SecretName:           machineClassWithHashPool2,
							Minimum:              minPool2,
							Maximum:              maxPool2,
							MaxSurge:             maxSurgePool2,
							MaxUnavailable:       maxUnavailablePool2,
							Labels:               labels,
							MachineConfiguration: machineConfiguration,
						},
					}

				})

				It("should return the expected machine deployments for profile image types", func() {
					workerDelegate, _ = NewWorkerDelegate(common.NewClientContext(c, scheme, decoder), chartApplier, "", w, cluster)

					expectGetSecretCallToWork(c, azureClientID, azureClientSecret, azureSubscriptionID, azureTenantID)

					// Test workerDelegate.DeployMachineClasses()
					chartApplier.EXPECT().Apply(context.TODO(), filepath.Join(azure.InternalChartsPath, "machineclass"), namespace, "machineclass", kubernetes.Values(machineClasses))

					err := workerDelegate.DeployMachineClasses(context.TODO())
					Expect(err).NotTo(HaveOccurred())

					// Test workerDelegate.UpdateMachineImagesStatus()

					expectStatusContainsMachineImages(c, statusWriter, w, []apiv1alpha1.MachineImage{
						{
							Name:                  machineImageName,
							Version:               machineImageVersion,
							URN:                   &machineImageURN,
							AcceleratedNetworking: &boolTrue,
						},
						{
							Name:    machineImageName,
							Version: machineImageVersionID,
							ID:      &machineImageID,
						},
					})
					err = workerDelegate.UpdateMachineImagesStatus(context.TODO())
					Expect(err).NotTo(HaveOccurred())

					// Test workerDelegate.GenerateMachineDeployments()

					result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(machineDeployments))
				})
			})

			It("should fail because the secret cannot be read", func() {
				c.EXPECT().Get(context.TODO(), gomock.Any(), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fmt.Errorf("error"))

				result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the version is invalid", func() {
				expectGetSecretCallToWork(c, azureClientID, azureClientSecret, azureSubscriptionID, azureTenantID)

				clusterWithoutImages.Shoot.Spec.Kubernetes.Version = "invalid"
				workerDelegate, _ = NewWorkerDelegate(common.NewClientContext(c, scheme, decoder), chartApplier, "", w, cluster)

				result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the infrastructure status cannot be decoded", func() {
				expectGetSecretCallToWork(c, azureClientID, azureClientSecret, azureSubscriptionID, azureTenantID)

				w.Spec.InfrastructureProviderStatus = &runtime.RawExtension{}

				workerDelegate, _ = NewWorkerDelegate(common.NewClientContext(c, scheme, decoder), chartApplier, "", w, cluster)

				result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the nodes subnet cannot be found", func() {
				expectGetSecretCallToWork(c, azureClientID, azureClientSecret, azureSubscriptionID, azureTenantID)

				w.Spec.InfrastructureProviderStatus = &runtime.RawExtension{
					Raw: encode(&apisazure.InfrastructureStatus{}),
				}

				workerDelegate, _ = NewWorkerDelegate(common.NewClientContext(c, scheme, decoder), chartApplier, "", w, cluster)

				result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the nodes availability set cannot be found", func() {
				expectGetSecretCallToWork(c, azureClientID, azureClientSecret, azureSubscriptionID, azureTenantID)

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
					}),
				}

				workerDelegate, _ = NewWorkerDelegate(common.NewClientContext(c, scheme, decoder), chartApplier, "", w, cluster)

				result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the machine image information cannot be found", func() {
				expectGetSecretCallToWork(c, azureClientID, azureClientSecret, azureSubscriptionID, azureTenantID)

				workerDelegate, _ = NewWorkerDelegate(common.NewClientContext(c, scheme, decoder), chartApplier, "", w, clusterWithoutImages)

				result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the volume size cannot be decoded", func() {
				expectGetSecretCallToWork(c, azureClientID, azureClientSecret, azureSubscriptionID, azureTenantID)

				w.Spec.Pools[0].Volume.Size = "not-decodeable"

				workerDelegate, _ = NewWorkerDelegate(common.NewClientContext(c, scheme, decoder), chartApplier, "", w, cluster)

				result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should set expected machineControllerManager settings on machine deployment", func() {
				expectGetSecretCallToWork(c, azureClientID, azureClientSecret, azureSubscriptionID, azureTenantID)

				testDrainTimeout := metav1.Duration{Duration: 10 * time.Minute}
				testHealthTimeout := metav1.Duration{Duration: 20 * time.Minute}
				testCreationTimeout := metav1.Duration{Duration: 30 * time.Minute}
				testMaxEvictRetries := int32(30)
				testNodeConditions := []string{"ReadonlyFilesystem", "KernelDeadlock", "DiskPressure"}
				w.Spec.Pools[0].MachineControllerManagerSettings = &gardencorev1beta1.MachineControllerManagerSettings{
					MachineDrainTimeout:    &testDrainTimeout,
					MachineCreationTimeout: &testCreationTimeout,
					MachineHealthTimeout:   &testHealthTimeout,
					MaxEvictRetries:        &testMaxEvictRetries,
					NodeConditions:         testNodeConditions,
				}

				workerDelegate, _ = NewWorkerDelegate(common.NewClientContext(c, scheme, decoder), chartApplier, "", w, cluster)

				result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
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

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}

func copyMachineClass(def map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(def))

	for k, v := range def {
		out[k] = v
	}

	return out
}

func expectGetSecretCallToWork(c *mockclient.MockClient, azureClientID, azureClientSecret, azureSubscriptionID, azureTenantID string) {
	c.EXPECT().
		Get(context.TODO(), gomock.Any(), gomock.AssignableToTypeOf(&corev1.Secret{})).
		DoAndReturn(func(_ context.Context, _ client.ObjectKey, secret *corev1.Secret) error {
			secret.Data = map[string][]byte{
				azure.ClientIDKey:       []byte(azureClientID),
				azure.ClientSecretKey:   []byte(azureClientSecret),
				azure.SubscriptionIDKey: []byte(azureSubscriptionID),
				azure.TenantIDKey:       []byte(azureTenantID),
			}
			return nil
		})
}

func expectStatusContainsMachineImages(c *mockclient.MockClient, statusWriter *mockclient.MockStatusWriter, worker *extensionsv1alpha1.Worker, images []apiv1alpha1.MachineImage) {
	expectedProviderStatus := &apiv1alpha1.WorkerStatus{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
			Kind:       "WorkerStatus",
		},
		MachineImages: images,
	}
	workerWithExpectedStatus := worker.DeepCopy()
	workerWithExpectedStatus.Status.ProviderStatus = &runtime.RawExtension{
		Object: expectedProviderStatus,
	}

	c.EXPECT().Get(context.TODO(), gomock.Any(), gomock.AssignableToTypeOf(&extensionsv1alpha1.Worker{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, worker *extensionsv1alpha1.Worker) error {
		return nil
	})
	c.EXPECT().Status().Return(statusWriter)
	statusWriter.EXPECT().Update(context.TODO(), workerWithExpectedStatus).Return(nil)
}

func addNameAndSecretsToMachineClass(class map[string]interface{}, azureClientID, azureClientSecret, azureSubscriptionID, azureTenantID, name string) {
	class["name"] = name
	class["labels"] = map[string]string{
		v1beta1constants.GardenerPurpose: genericworkeractuator.GardenPurposeMachineClass,
	}
	class["secret"].(map[string]interface{})[azure.ClientIDKey] = azureClientID
	class["secret"].(map[string]interface{})[azure.ClientSecretKey] = azureClientSecret
	class["secret"].(map[string]interface{})[azure.SubscriptionIDKey] = azureSubscriptionID
	class["secret"].(map[string]interface{})[azure.TenantIDKey] = azureTenantID
}
