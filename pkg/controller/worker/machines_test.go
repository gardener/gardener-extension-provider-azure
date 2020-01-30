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

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/controller/worker"
	extensionscontroller "github.com/gardener/gardener-extensions/pkg/controller"
	"github.com/gardener/gardener-extensions/pkg/controller/common"
	"github.com/gardener/gardener-extensions/pkg/controller/worker"
	genericworkeractuator "github.com/gardener/gardener-extensions/pkg/controller/worker/genericactuator"
	mockclient "github.com/gardener/gardener-extensions/pkg/mock/controller-runtime/client"
	mockkubernetes "github.com/gardener/gardener-extensions/pkg/mock/gardener/client/kubernetes"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
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
		chartApplier *mockkubernetes.MockChartApplier
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)
		chartApplier = mockkubernetes.NewMockChartApplier(ctrl)
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

				machineImageName    string
				machineImageVersion string
				machineImageURN     string

				resourceGroupName string
				vnetName          string
				subnetName        string
				availabilitySetID string
				machineType       string
				userData          []byte
				volumeSize        int
				sshKey            string

				namePool1           string
				minPool1            int
				maxPool1            int
				maxSurgePool1       intstr.IntOrString
				maxUnavailablePool1 intstr.IntOrString

				namePool2           string
				minPool2            int
				maxPool2            int
				maxSurgePool2       intstr.IntOrString
				maxUnavailablePool2 intstr.IntOrString

				workerPoolHash1 string
				workerPoolHash2 string

				shootVersionMajorMinor string
				shootVersion           string
				scheme                 *runtime.Scheme
				decoder                runtime.Decoder
				clusterWithoutImages   *extensionscontroller.Cluster
				cluster                *extensionscontroller.Cluster
				w                      *extensionsv1alpha1.Worker
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
				machineImageURN = "bar:baz:foo:123"

				resourceGroupName = "my-rg"
				vnetName = "my-vnet"
				subnetName = "subnet-1234"
				availabilitySetID = "av-1234"
				machineType = "large"
				userData = []byte("some-user-data")
				volumeSize = 20
				sshKey = "public-key"

				namePool1 = "pool-1"
				minPool1 = 5
				maxPool1 = 10
				maxSurgePool1 = intstr.FromInt(3)
				maxUnavailablePool1 = intstr.FromInt(2)

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
						apiv1alpha1.MachineImages{
							Name: machineImageName,
							Versions: []apiv1alpha1.MachineImageVersion{
								apiv1alpha1.MachineImageVersion{
									Version: machineImageVersion,
									URN:     machineImageURN,
								},
							},
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
							ProviderConfig: &gardencorev1beta1.ProviderConfig{
								RawExtension: runtime.RawExtension{
									Raw: cloudProfileConfigJSON,
								},
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
										Name: vnetName,
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
									Version: machineImageVersion,
								},
								UserData: userData,
								Volume: &extensionsv1alpha1.Volume{
									Size: fmt.Sprintf("%dGi", volumeSize),
								},
							},
						},
					},
				}

				scheme = runtime.NewScheme()
				_ = apisazure.AddToScheme(scheme)
				_ = apiv1alpha1.AddToScheme(scheme)
				decoder = serializer.NewCodecFactory(scheme).UniversalDecoder()

				workerPoolHash1, _ = worker.WorkerPoolHash(w.Spec.Pools[0], cluster)
				workerPoolHash2, _ = worker.WorkerPoolHash(w.Spec.Pools[1], cluster)

				workerDelegate, _ = NewWorkerDelegate(common.NewClientContext(c, scheme, decoder), chartApplier, "", w, clusterWithoutImages)
			})

			Describe("machine images", func() {
				var (
					defaultMachineClass map[string]interface{}
					machineDeployments  worker.MachineDeployments
					machineClasses      map[string]interface{}
				)

				BeforeEach(func() {
					defaultMachineClass = map[string]interface{}{
						"region":            region,
						"resourceGroup":     resourceGroupName,
						"vnetName":          vnetName,
						"subnetName":        subnetName,
						"availabilitySetID": availabilitySetID,
						"tags": map[string]interface{}{
							"Name": namespace,
							fmt.Sprintf("kubernetes.io-cluster-%s", namespace): "1",
							"kubernetes.io-role-node":                          "1",
						},
						"secret": map[string]interface{}{
							"cloudConfig": string(userData),
						},
						"machineType": machineType,
						"image": map[string]interface{}{
							"urn": machineImageURN,
						},
						"osDisk": map[string]interface{}{
							"size": volumeSize,
						},
						"sshPublicKey": sshKey,
					}

					var (
						machineClassPool1 = copyMachineClass(defaultMachineClass)
						machineClassPool2 = copyMachineClass(defaultMachineClass)

						machineClassNamePool1 = fmt.Sprintf("%s-%s", namespace, namePool1)
						machineClassNamePool2 = fmt.Sprintf("%s-%s", namespace, namePool2)

						machineClassWithHashPool1 = fmt.Sprintf("%s-%s", machineClassNamePool1, workerPoolHash1)
						machineClassWithHashPool2 = fmt.Sprintf("%s-%s", machineClassNamePool2, workerPoolHash2)
					)

					addNameAndSecretsToMachineClass(machineClassPool1, azureClientID, azureClientSecret, azureSubscriptionID, azureTenantID, machineClassWithHashPool1)
					addNameAndSecretsToMachineClass(machineClassPool2, azureClientID, azureClientSecret, azureSubscriptionID, azureTenantID, machineClassWithHashPool2)

					machineClasses = map[string]interface{}{"machineClasses": []map[string]interface{}{
						machineClassPool1,
						machineClassPool2,
					}}

					machineDeployments = worker.MachineDeployments{
						{
							Name:           machineClassNamePool1,
							ClassName:      machineClassWithHashPool1,
							SecretName:     machineClassWithHashPool1,
							Minimum:        minPool1,
							Maximum:        maxPool1,
							MaxSurge:       maxSurgePool1,
							MaxUnavailable: maxUnavailablePool1,
						},
						{
							Name:           machineClassNamePool2,
							ClassName:      machineClassWithHashPool2,
							SecretName:     machineClassWithHashPool2,
							Minimum:        minPool2,
							Maximum:        maxPool2,
							MaxSurge:       maxSurgePool2,
							MaxUnavailable: maxUnavailablePool2,
						},
					}

				})

				It("should return the expected machine deployments for profile image types", func() {
					workerDelegate, _ = NewWorkerDelegate(common.NewClientContext(c, scheme, decoder), chartApplier, "", w, cluster)

					expectGetSecretCallToWork(c, azureClientID, azureClientSecret, azureSubscriptionID, azureTenantID)

					// Test workerDelegate.DeployMachineClasses()
					chartApplier.
						EXPECT().
						ApplyChart(
							context.TODO(),
							filepath.Join(azure.InternalChartsPath, "machineclass"),
							namespace,
							"machineclass",
							machineClasses,
							nil,
						).
						Return(nil)

					err := workerDelegate.DeployMachineClasses(context.TODO())
					Expect(err).NotTo(HaveOccurred())

					// Test workerDelegate.GetMachineImages()
					machineImages, err := workerDelegate.GetMachineImages(context.TODO())
					Expect(machineImages).To(Equal(&apiv1alpha1.WorkerStatus{
						TypeMeta: metav1.TypeMeta{
							APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
							Kind:       "WorkerStatus",
						},
						MachineImages: []apiv1alpha1.MachineImage{
							{
								Name:    machineImageName,
								Version: machineImageVersion,
								URN:     &machineImageURN,
							},
						},
					}))
					Expect(err).NotTo(HaveOccurred())

					// Test workerDelegate.GenerateMachineDeployments()

					result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(machineDeployments))
				})
			})

			It("should fail because the secret cannot be read", func() {
				c.EXPECT().
					Get(context.TODO(), gomock.Any(), gomock.AssignableToTypeOf(&corev1.Secret{})).
					Return(fmt.Errorf("error"))

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

func addNameAndSecretsToMachineClass(class map[string]interface{}, azureClientID, azureClientSecret, azureSubscriptionID, azureTenantID, name string) {
	class["name"] = name
	class["labels"] = map[string]string{
		v1beta1constants.GardenPurpose: genericworkeractuator.GardenPurposeMachineClass,
	}
	class["secret"].(map[string]interface{})[azure.ClientIDKey] = azureClientID
	class["secret"].(map[string]interface{})[azure.ClientSecretKey] = azureClientSecret
	class["secret"].(map[string]interface{})[azure.SubscriptionIDKey] = azureSubscriptionID
	class["secret"].(map[string]interface{})[azure.TenantIDKey] = azureTenantID
}
