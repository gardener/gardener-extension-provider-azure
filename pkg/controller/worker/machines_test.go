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
	"fmt"
	"path/filepath"
	"strings"
	"time"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/controller/worker"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	genericworkeractuator "github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/utils"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

var _ = Describe("Machines", func() {
	var (
		ctrl         *gomock.Controller
		c            *mockclient.MockClient
		statusWriter *mockclient.MockStatusWriter
		chartApplier *mockkubernetes.MockChartApplier

		ctx               context.Context
		namespace, region string
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)
		chartApplier = mockkubernetes.NewMockChartApplier(ctrl)
		statusWriter = mockclient.NewMockStatusWriter(ctrl)

		// Let the client always the mocked status writer when Status() is called.
		c.EXPECT().Status().AnyTimes().Return(statusWriter)

		ctx = context.TODO()
		namespace = "shoot--foobar--azure"
		region = "westeurope"
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("workerDelegate", func() {
		Describe("#MachineClassKind", func() {
			It("should return the correct kind of the machine class", func() {
				w := makeWorker(namespace, region, nil, nil)
				workerDelegate := wrapNewWorkerDelegate(c, nil, w, nil, nil)
				Expect(workerDelegate.MachineClassKind()).To(Equal("MachineClass"))
			})
		})

		Describe("#MachineClass", func() {
			It("should return the correct type for the machine class", func() {
				w := makeWorker(namespace, region, nil, nil)
				workerDelegate := wrapNewWorkerDelegate(c, nil, w, nil, nil)
				Expect(workerDelegate.MachineClass()).To(Equal(&machinev1alpha1.MachineClass{}))
			})
		})

		Describe("#MachineClassList", func() {
			It("should return the correct type for the machine class list", func() {
				w := makeWorker(namespace, region, nil, nil)
				workerDelegate := wrapNewWorkerDelegate(c, nil, w, nil, nil)
				Expect(workerDelegate.MachineClassList()).To(Equal(&machinev1alpha1.MachineClassList{}))
			})
		})

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
				machineImageVersion = "1"
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

				nodeTemplateZone1 = machinev1alpha1.NodeTemplate{
					Capacity:     nodeCapacity,
					InstanceType: machineType,
					Region:       region,
					Zone:         "no-zone",
				}

				nodeTemplateZone2 = machinev1alpha1.NodeTemplate{
					Capacity:     nodeCapacity,
					InstanceType: machineType,
					Region:       region,
					Zone:         "no-zone",
				}

				nodeTemplateZone3 = machinev1alpha1.NodeTemplate{
					Capacity:     nodeCapacity,
					InstanceType: machineType,
					Region:       region,
					Zone:         "no-zone",
				}

				nodeTemplateZone4 = machinev1alpha1.NodeTemplate{
					Capacity:     nodeCapacity,
					InstanceType: machineType,
					Region:       region,
					Zone:         "no-zone",
				}

				namePool2 = "pool-zones"
				minPool2 = 30
				maxPool2 = 45
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

				shootVersionMajorMinor = "1.21"
				shootVersion = shootVersionMajorMinor + ".3"

				machineImages = []apiv1alpha1.MachineImages{
					{
						Name: machineImageName,
						Versions: []apiv1alpha1.MachineImageVersion{
							{
								Version:               machineImageVersion,
								URN:                   &machineImageURN,
								AcceleratedNetworking: pointer.BoolPtr(true),
							},
							{
								Version: machineImageVersionID,
								ID:      &machineImageID,
							},
							{
								Version:                 machineImageVersionCommunityID,
								CommunityGalleryImageID: &machineImageCommunityID,
							},
							{
								Version:              machineImageVersionSharedID,
								SharedGalleryImageID: &machineImageSharedID,
							},
						},
					},
				}
				machineTypes = []apiv1alpha1.MachineType{
					{
						Name:                  machineType,
						AcceleratedNetworking: pointer.BoolPtr(true),
					},
				}

				pool1 = extensionsv1alpha1.WorkerPool{
					Name:           namePool1,
					Minimum:        minPool1,
					Maximum:        maxPool1,
					MaxSurge:       maxSurgePool1,
					MaxUnavailable: maxUnavailablePool1,
					MachineType:    machineType,
					NodeTemplate: &extensionsv1alpha1.NodeTemplate{
						Capacity: nodeCapacity,
					},
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
				}

				pool2 = extensionsv1alpha1.WorkerPool{
					Name:           namePool2,
					Minimum:        minPool2,
					Maximum:        maxPool2,
					MaxSurge:       maxSurgePool2,
					MaxUnavailable: maxUnavailablePool2,
					MachineType:    machineType,
					NodeTemplate: &extensionsv1alpha1.NodeTemplate{
						Capacity: nodeCapacity,
					},
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
				}

				pool3 = extensionsv1alpha1.WorkerPool{
					Name:           namePool3,
					Minimum:        minPool3,
					Maximum:        maxPool3,
					MaxSurge:       maxSurgePool3,
					MaxUnavailable: maxUnavailablePool3,
					MachineType:    machineType,
					NodeTemplate: &extensionsv1alpha1.NodeTemplate{
						Capacity: nodeCapacity,
					},
					MachineImage: extensionsv1alpha1.MachineImage{
						Name:    machineImageName,
						Version: machineImageVersionCommunityID,
					},
					UserData: userData,
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
					MaxUnavailable: maxUnavailablePool4,
					MachineType:    machineType,
					NodeTemplate: &extensionsv1alpha1.NodeTemplate{
						Capacity: nodeCapacity,
					},
					MachineImage: extensionsv1alpha1.MachineImage{
						Name:    machineImageName,
						Version: machineImageVersionSharedID,
					},
					UserData: userData,
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

					workerPoolHash1, _ = worker.WorkerPoolHash(w.Spec.Pools[0], cluster, fmt.Sprintf("%dGi", dataVolume2Size), dataVolume2Type, fmt.Sprintf("%dGi", dataVolume1Size), identityID)
					workerPoolHash2, _ = worker.WorkerPoolHash(w.Spec.Pools[1], cluster, identityID)
					workerPoolHash3, _ = worker.WorkerPoolHash(w.Spec.Pools[2], cluster, identityID)
					workerPoolHash4, _ = worker.WorkerPoolHash(w.Spec.Pools[3], cluster, identityID)

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

					machineClasses = map[string]interface{}{"machineClasses": []map[string]interface{}{
						machineClassPool1,
						machineClassPool2,
						machineClassPool3,
						machineClassPool4,
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
							MachineConfiguration: &machinev1alpha1.MachineConfiguration{},
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
							MachineConfiguration: &machinev1alpha1.MachineConfiguration{},
						},
						{
							Name:                 machineClassNamePool3,
							ClassName:            machineClassWithHashPool3,
							SecretName:           machineClassWithHashPool3,
							Minimum:              minPool3,
							Maximum:              maxPool3,
							MaxSurge:             maxSurgePool3,
							MaxUnavailable:       maxUnavailablePool3,
							Labels:               labels,
							MachineConfiguration: &machinev1alpha1.MachineConfiguration{},
						},
						{
							Name:                 machineClassNamePool4,
							ClassName:            machineClassWithHashPool4,
							SecretName:           machineClassWithHashPool4,
							Minimum:              minPool4,
							Maximum:              maxPool4,
							MaxSurge:             maxSurgePool4,
							MaxUnavailable:       maxUnavailablePool4,
							Labels:               labels,
							MachineConfiguration: &machinev1alpha1.MachineConfiguration{},
						},
					}
				})

				It("should return the expected machine deployments for profile image types", func() {
					workerDelegate := wrapNewWorkerDelegate(c, chartApplier, w, cluster, nil)

					chartApplier.
						EXPECT().
						Apply(
							ctx,
							filepath.Join(azure.InternalChartsPath, "machineclass"),
							namespace,
							"machineclass",
							kubernetes.Values(machineClasses),
						)

					// Test workerDelegate.DeployMachineClasses()
					err := workerDelegate.DeployMachineClasses(ctx)
					Expect(err).NotTo(HaveOccurred())

					// Test workerDelegate.UpdateMachineImagesStatus()
					expectWorkerProviderStatusUpdateToSucceed(ctx, c, statusWriter)
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
							UserData: userData,
							Labels:   labels,
							Zones:    []string{zone1, zone2},
						}

						w = makeWorker(namespace, region, &sshKey, infrastructureStatus, poolZones)

						workerPoolHashZ1, _ = worker.WorkerPoolHash(w.Spec.Pools[0], cluster, identityID)
						workerPoolHashZ2, _ = worker.WorkerPoolHash(w.Spec.Pools[0], cluster, identityID, subnet2)

						basename := fmt.Sprintf("%s-%s", namespace, namePoolZones)
						machineClassNamePool1 = fmt.Sprintf("%s-z%s", basename, zone1)
						machineClassNamePool2 = fmt.Sprintf("%s-z%s", basename, zone2)
						machineClassWithHashPool1 = fmt.Sprintf("%s-%s-z%s", basename, workerPoolHashZ1, zone1)
						machineClassWithHashPool2 = fmt.Sprintf("%s-%s-z%s", basename, workerPoolHashZ2, zone2)
						labelsPool1 := utils.MergeStringMaps(labels, map[string]string{azureCSIDiskDriverTopologyKey: region + "-" + zone1})
						labelsPool2 := utils.MergeStringMaps(labels, map[string]string{azureCSIDiskDriverTopologyKey: region + "-" + zone2})

						machineDeployments = worker.MachineDeployments{
							{
								Name:                 machineClassNamePool1,
								ClassName:            machineClassWithHashPool1,
								SecretName:           machineClassWithHashPool1,
								Minimum:              minPoolZones,
								Maximum:              maxPoolZones,
								MaxSurge:             maxSurgePoolZones,
								MaxUnavailable:       maxUnavailablePoolZones,
								Labels:               labelsPool1,
								MachineConfiguration: &machinev1alpha1.MachineConfiguration{},
							},
							{
								Name:                 machineClassNamePool2,
								ClassName:            machineClassWithHashPool2,
								SecretName:           machineClassWithHashPool2,
								Minimum:              minPoolZones,
								Maximum:              maxPoolZones,
								MaxSurge:             maxSurgePoolZones,
								MaxUnavailable:       maxUnavailablePoolZones,
								Labels:               labelsPool2,
								MachineConfiguration: &machinev1alpha1.MachineConfiguration{},
							},
						}
					})

					It("should return the correct machine deployments for zonal setup", func() {
						workerDelegate := wrapNewWorkerDelegate(c, chartApplier, w, cluster, nil)

						// Test workerDelegate.GenerateMachineDeployments()
						result, err := workerDelegate.GenerateMachineDeployments(ctx)
						Expect(err).NotTo(HaveOccurred())
						Expect(result).To(Equal(machineDeployments))
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
				w.Spec.Pools[0].Architecture = pointer.String("arm64")
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
		v1beta1constants.GardenerPurpose: genericworkeractuator.GardenPurposeMachineClass,
	}
}
