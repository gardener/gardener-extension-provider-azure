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

package worker

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	azureapi "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	azureapihelper "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"

	genericworkeractuator "github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	tagRegex = regexp.MustCompile(`[<>%\\&?/ ]`)
)

// MachineClassKind yields the name of machine class kind used by Azure provider.
func (w *workerDelegate) MachineClassKind() string {
	return "MachineClass"
}

// MachineClass yields a newly initialized machine class object.
func (w *workerDelegate) MachineClass() client.Object {
	return &machinev1alpha1.MachineClass{}
}

// MachineClassList yields a newly initialized MachineClassList object.
func (w *workerDelegate) MachineClassList() client.ObjectList {
	return &machinev1alpha1.MachineClassList{}
}

// DeployMachineClasses generates and creates the Azure specific machine classes.
func (w *workerDelegate) DeployMachineClasses(ctx context.Context) error {
	if w.machineClasses == nil {
		if err := w.generateMachineConfig(ctx); err != nil {
			return err
		}
	}

	return w.seedChartApplier.Apply(ctx, filepath.Join(azure.InternalChartsPath, "machineclass"), w.worker.Namespace, "machineclass", kubernetes.Values(map[string]interface{}{"machineClasses": w.machineClasses}))
}

// GenerateMachineDeployments generates the configuration for the desired machine deployments.
func (w *workerDelegate) GenerateMachineDeployments(ctx context.Context) (worker.MachineDeployments, error) {
	if w.machineDeployments == nil {
		if err := w.generateMachineConfig(ctx); err != nil {
			return nil, err
		}
	}
	return w.machineDeployments, nil
}

type zoneInfo struct {
	name  string
	index int32
	count int32
}

type machineSetInfo struct {
	id   string
	kind string
}

func (w *workerDelegate) generateMachineConfig(ctx context.Context) error {
	var (
		acceleratedNetworkAllowed = true
		machineDeployments        = worker.MachineDeployments{}
		machineClasses            []map[string]interface{}
		machineImages             []azureapi.MachineImage
	)

	infrastructureStatus, err := w.decodeAzureInfrastructureStatus()
	if err != nil {
		return err
	}

	workerStatus, err := w.decodeWorkerProviderStatus()
	if err != nil {
		return err
	}

	nodesSubnet, err := azureapihelper.FindSubnetByPurpose(infrastructureStatus.Networks.Subnets, azureapi.PurposeNodes)
	if err != nil {
		return err
	}

	for _, pool := range w.worker.Spec.Pools {
		// Get the vmo dependency from the worker status if exists.
		vmoDependency, err := w.determineWorkerPoolVmoDependency(ctx, infrastructureStatus, workerStatus, pool.Name)
		if err != nil {
			return err
		}

		urn, id, imageSupportAcceleratedNetworking, err := w.findMachineImage(pool.MachineImage.Name, pool.MachineImage.Version)
		if err != nil {
			return err
		}
		machineImages = appendMachineImage(machineImages, azureapi.MachineImage{
			Name:                  pool.MachineImage.Name,
			Version:               pool.MachineImage.Version,
			URN:                   urn,
			ID:                    id,
			AcceleratedNetworking: imageSupportAcceleratedNetworking,
		})

		image := map[string]interface{}{}
		if urn != nil {
			image["urn"] = *urn
		} else {
			image["id"] = *id
		}

		disks, err := computeDisks(pool)
		if err != nil {
			return err
		}

		generateMachineClassAndDeployment := func(zone *zoneInfo, machineSet *machineSetInfo, subnetName, workerPoolHash string) (worker.MachineDeployment, map[string]interface{}) {
			var (
				machineDeployment = worker.MachineDeployment{
					Minimum:              pool.Minimum,
					Maximum:              pool.Maximum,
					MaxSurge:             pool.MaxSurge,
					MaxUnavailable:       pool.MaxUnavailable,
					Labels:               pool.Labels,
					Annotations:          pool.Annotations,
					Taints:               pool.Taints,
					MachineConfiguration: genericworkeractuator.ReadMachineConfiguration(pool),
				}

				machineClassSpec = utils.MergeMaps(map[string]interface{}{
					"region":        w.worker.Spec.Region,
					"resourceGroup": infrastructureStatus.ResourceGroup.Name,
					"tags":          w.getVMTags(pool),
					"secret": map[string]interface{}{
						"cloudConfig": string(pool.UserData),
					},
					"credentialsSecretRef": map[string]interface{}{
						"name":      w.worker.Spec.SecretRef.Name,
						"namespace": w.worker.Spec.SecretRef.Namespace,
					},
					"machineType":  pool.MachineType,
					"image":        image,
					"sshPublicKey": string(w.worker.Spec.SSHPublicKey),
				}, disks)
			)

			networkConfig := map[string]interface{}{
				"vnet":   infrastructureStatus.Networks.VNet.Name,
				"subnet": subnetName,
			}
			if infrastructureStatus.Networks.VNet.ResourceGroup != nil {
				networkConfig["vnetResourceGroup"] = *infrastructureStatus.Networks.VNet.ResourceGroup
			}
			if imageSupportAcceleratedNetworking != nil && *imageSupportAcceleratedNetworking && w.isMachineTypeSupportingAcceleratedNetworking(pool.MachineType) && acceleratedNetworkAllowed {
				networkConfig["acceleratedNetworking"] = true
			}
			machineClassSpec["network"] = networkConfig

			if zone != nil {
				machineDeployment.Minimum = worker.DistributeOverZones(zone.index, pool.Minimum, zone.count)
				machineDeployment.Maximum = worker.DistributeOverZones(zone.index, pool.Maximum, zone.count)
				machineDeployment.MaxSurge = worker.DistributePositiveIntOrPercent(zone.index, pool.MaxSurge, zone.count, pool.Maximum)
				machineDeployment.MaxUnavailable = worker.DistributePositiveIntOrPercent(zone.index, pool.MaxUnavailable, zone.count, pool.Minimum)
				machineClassSpec["zone"] = zone.name
			}

			if machineSet != nil {
				machineClassSpec["machineSet"] = map[string]interface{}{
					"kind": machineSet.kind,
					"id":   machineSet.id,
				}
			}

			if infrastructureStatus.Identity != nil {
				machineClassSpec["identityID"] = infrastructureStatus.Identity.ID
			}

			var (
				deploymentName = fmt.Sprintf("%s-%s", w.worker.Namespace, pool.Name)
				className      = fmt.Sprintf("%s-%s", deploymentName, workerPoolHash)
			)

			if zone != nil {
				deploymentName = fmt.Sprintf("%s-z%s", deploymentName, zone.name)
				className = fmt.Sprintf("%s-z%s", className, zone.name)
			}

			machineDeployment.Name = deploymentName
			machineDeployment.ClassName = className
			machineDeployment.SecretName = className

			machineClassSpec["name"] = className
			machineClassSpec["labels"] = map[string]string{v1beta1constants.GardenerPurpose: genericworkeractuator.GardenPurposeMachineClass}

			return machineDeployment, machineClassSpec
		}

		// VMO
		if vmoDependency != nil {
			workerPoolHash, err := w.generateWorkerPoolHash(pool, infrastructureStatus, vmoDependency, nil)
			if err != nil {
				return err
			}
			machineDeployment, machineClassSpec := generateMachineClassAndDeployment(nil, &machineSetInfo{
				id:   vmoDependency.ID,
				kind: "vmo",
			}, nodesSubnet.Name, workerPoolHash)
			machineDeployments = append(machineDeployments, machineDeployment)
			machineClasses = append(machineClasses, machineClassSpec)
			continue
		}

		// AvailabilitySet
		if !infrastructureStatus.Zoned && vmoDependency == nil {
			nodesAvailabilitySet, err := azureapihelper.FindAvailabilitySetByPurpose(infrastructureStatus.AvailabilitySets, azureapi.PurposeNodes)
			if err != nil {
				return err
			}

			// Do not enable accelerated networking for AvSet cluster.
			// This is necessary to avoid `ExistingAvailabilitySetWasNotDeployedOnAcceleratedNetworkingEnabledCluster` error.
			acceleratedNetworkAllowed = false

			workerPoolHash, err := w.generateWorkerPoolHash(pool, infrastructureStatus, vmoDependency, nil)
			if err != nil {
				return err
			}

			machineDeployment, machineClassSpec := generateMachineClassAndDeployment(nil, &machineSetInfo{
				id:   nodesAvailabilitySet.ID,
				kind: "availabilityset",
			}, nodesSubnet.Name, workerPoolHash)
			machineDeployments = append(machineDeployments, machineDeployment)
			machineClasses = append(machineClasses, machineClassSpec)
			continue
		}

		// Availability Zones
		var zoneCount = len(pool.Zones)
		for zoneIndex, zone := range pool.Zones {
			var (
				subnetName     string
				workerPoolHash string
			)

			if infrastructureStatus.Networks.Topology == azureapi.TopologyZonal {
				subnetIndex, nodesSubnet, err := azureapihelper.FindSubnetByPurposeAndZone(infrastructureStatus.Networks.Subnets, azureapi.PurposeNodes, zone)
				if err != nil {
					return err
				}
				subnetName = nodesSubnet.Name
				if subnetIndex == 0 {
					workerPoolHash, err = w.generateWorkerPoolHash(pool, infrastructureStatus, vmoDependency, nil)
					if err != nil {
						return err
					}
				} else {
					workerPoolHash, err = w.generateWorkerPoolHash(pool, infrastructureStatus, vmoDependency, &nodesSubnet.Name)
					if err != nil {
						return err
					}
				}
			} else {
				subnetName = nodesSubnet.Name
				workerPoolHash, err = w.generateWorkerPoolHash(pool, infrastructureStatus, vmoDependency, nil)
				if err != nil {
					return err
				}
			}
			machineDeployment, machineClassSpec := generateMachineClassAndDeployment(&zoneInfo{
				name:  zone,
				index: int32(zoneIndex),
				count: int32(zoneCount),
			}, nil, subnetName, workerPoolHash)
			machineDeployments = append(machineDeployments, machineDeployment)
			machineClasses = append(machineClasses, machineClassSpec)
		}
	}

	w.machineDeployments = machineDeployments
	w.machineClasses = machineClasses
	w.machineImages = machineImages

	return nil
}

// isMachineTypeSupportingAcceleratedNetworking checks if the passed machine type is supporting Azure accelerated networking.
func (w *workerDelegate) isMachineTypeSupportingAcceleratedNetworking(machineTypeName string) bool {
	for _, machType := range w.cloudProfileConfig.MachineTypes {
		if machType.Name == machineTypeName && machType.AcceleratedNetworking != nil && *machType.AcceleratedNetworking {
			return true
		}
	}
	return false
}

// getVMTags returns a map of vm tags
func (w *workerDelegate) getVMTags(pool extensionsv1alpha1.WorkerPool) map[string]string {
	vmTags := map[string]string{
		"Name": w.worker.Namespace,
		SanitizeAzureVMTag(fmt.Sprintf("kubernetes.io-cluster-%s", w.worker.Namespace)): "1",
		SanitizeAzureVMTag("kubernetes.io-role-node"):                                   "1",
	}
	for k, v := range pool.Labels {
		vmTags[SanitizeAzureVMTag(k)] = v
	}
	return vmTags
}

func computeDisks(pool extensionsv1alpha1.WorkerPool) (map[string]interface{}, error) {
	// handle root disk
	volumeSize, err := worker.DiskSize(pool.Volume.Size)
	if err != nil {
		return nil, err
	}
	osDisk := map[string]interface{}{
		"size": volumeSize,
	}
	// In the past the volume type information was not passed to the machineclass.
	// In consequence the Machine controller manager has created machines always
	// with the default volume type of the requested machine type. Existing clusters
	// respectively their worker pools could have an invalid volume configuration
	// which was not applied. To do not damage existing cluster we will set for
	// now the volume type only if it's a valid Azure volume type.
	// Otherwise we will still use the default volume of the machine type.
	if pool.Volume.Type != nil && (*pool.Volume.Type == "Standard_LRS" || *pool.Volume.Type == "StandardSSD_LRS" || *pool.Volume.Type == "Premium_LRS") {
		osDisk["type"] = *pool.Volume.Type
	}

	disks := map[string]interface{}{
		"osDisk": osDisk,
	}

	// handle data disks
	var dataDisks []map[string]interface{}
	if dataVolumes := pool.DataVolumes; len(dataVolumes) > 0 {
		// sort data volumes for consistent device naming
		sort.Slice(dataVolumes, func(i, j int) bool {
			return dataVolumes[i].Name < dataVolumes[j].Name
		})

		for i, volume := range dataVolumes {
			volumeSize, err := worker.DiskSize(volume.Size)
			if err != nil {
				return nil, err
			}
			disk := map[string]interface{}{
				"name":       volume.Name,
				"lun":        int32(i),
				"diskSizeGB": volumeSize,
				"caching":    "None",
			}
			if volume.Type != nil {
				disk["storageAccountType"] = *volume.Type
			}
			dataDisks = append(dataDisks, disk)
		}

		disks["dataDisks"] = dataDisks
	}

	return disks, nil
}

// SanitizeAzureVMTag will sanitize the tag base on the azure tag Restrictions
// refer: https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/tag-resources#limitations
func SanitizeAzureVMTag(label string) string {
	return tagRegex.ReplaceAllString(strings.ToLower(label), "_")
}

func (w *workerDelegate) generateWorkerPoolHash(pool extensionsv1alpha1.WorkerPool, infrastructureStatus *azureapi.InfrastructureStatus, vmoDependency *azureapi.VmoDependency, subnetName *string) (string, error) {
	var additionalHashData = []string{}

	// Integrate data disks/volumes in the hash.
	for _, dv := range pool.DataVolumes {
		additionalHashData = append(additionalHashData, dv.Size)

		if dv.Type != nil {
			additionalHashData = append(additionalHashData, *dv.Type)
		}
	}

	// Incorporate the identity ID in the workerpool hash.
	// Machines need to be rolled when the identity has been exchanged.
	if infrastructureStatus.Identity != nil {
		additionalHashData = append(additionalHashData, infrastructureStatus.Identity.ID)
	}

	// Include the vmo dependency name into the workerpool hash.
	if vmoDependency != nil {
		additionalHashData = append(additionalHashData, vmoDependency.Name)
	}

	if subnetName != nil {
		additionalHashData = append(additionalHashData, *subnetName)
	}

	// Generate the worker pool hash.
	workerPoolHash, err := worker.WorkerPoolHash(pool, w.cluster, additionalHashData...)
	if err != nil {
		return "", err
	}
	return workerPoolHash, nil
}
