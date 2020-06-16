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
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	genericworkeractuator "github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	tagRegex = regexp.MustCompile(`[<>%\\&?/ ]`)
)

// MachineClassKind yields the name of the Azure machine class.
func (w *workerDelegate) MachineClassKind() string {
	return "AzureMachineClass"
}

// MachineClassList yields a newly initialized AzureMachineClassList object.
func (w *workerDelegate) MachineClassList() runtime.Object {
	return &machinev1alpha1.AzureMachineClassList{}
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

func (w *workerDelegate) generateMachineClassSecretData(ctx context.Context) (map[string][]byte, error) {
	credentials, err := internal.GetClientAuthData(ctx, w.Client(), w.worker.Spec.SecretRef)
	if err != nil {
		return nil, err
	}

	return map[string][]byte{
		machinev1alpha1.AzureClientID:       []byte(credentials.ClientID),
		machinev1alpha1.AzureClientSecret:   []byte(credentials.ClientSecret),
		machinev1alpha1.AzureSubscriptionID: []byte(credentials.SubscriptionID),
		machinev1alpha1.AzureTenantID:       []byte(credentials.TenantID),
	}, nil
}

type zoneInfo struct {
	name  string
	index int32
	count int32
}

func (w *workerDelegate) generateMachineConfig(ctx context.Context) error {
	var (
		acceleratedNetworkAllowed = true
		machineDeployments        = worker.MachineDeployments{}
		machineClasses            []map[string]interface{}
		machineImages             []azureapi.MachineImage
		nodesAvailabilitySet      *azureapi.AvailabilitySet
	)

	machineClassSecretData, err := w.generateMachineClassSecretData(ctx)
	if err != nil {
		return err
	}

	infrastructureStatus := &azureapi.InfrastructureStatus{}
	if _, _, err := w.Decoder().Decode(w.worker.Spec.InfrastructureProviderStatus.Raw, nil, infrastructureStatus); err != nil {
		return err
	}

	nodesSubnet, err := azureapihelper.FindSubnetByPurpose(infrastructureStatus.Networks.Subnets, azureapi.PurposeNodes)
	if err != nil {
		return err
	}

	// The AvailabilitySet will be only used for non zoned Shoots.
	if !infrastructureStatus.Zoned {
		nodesAvailabilitySet, err = azureapihelper.FindAvailabilitySetByPurpose(infrastructureStatus.AvailabilitySets, azureapi.PurposeNodes)
		if err != nil {
			return err
		}

		// Do not enable accelerated networking for AvSet cluster.
		// This is necessary to avoid `ExistingAvailabilitySetWasNotDeployedOnAcceleratedNetworkingEnabledCluster` error.
		acceleratedNetworkAllowed = false
	}

	for _, pool := range w.worker.Spec.Pools {
		var additionalHashData []string
		if infrastructureStatus.Identity != nil {
			additionalHashData = append(additionalHashData, infrastructureStatus.Identity.ID)
		}
		additionalHashData = append(additionalHashData, computeAdditionalHashData(pool)...)

		workerPoolHash, err := worker.WorkerPoolHash(pool, w.cluster, additionalHashData...)
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

		generateMachineClassAndDeployment := func(zone *zoneInfo, availabilitySetID *string) (worker.MachineDeployment, map[string]interface{}) {
			var (
				machineDeployment = worker.MachineDeployment{
					Minimum:        pool.Minimum,
					Maximum:        pool.Maximum,
					MaxSurge:       pool.MaxSurge,
					MaxUnavailable: pool.MaxUnavailable,
					Labels:         pool.Labels,
					Annotations:    pool.Annotations,
					Taints:         pool.Taints,
				}

				machineClassSpec = utils.MergeMaps(map[string]interface{}{
					"region":        w.worker.Spec.Region,
					"resourceGroup": infrastructureStatus.ResourceGroup.Name,
					"tags":          w.getVmTags(pool),
					"secret": map[string]interface{}{
						"cloudConfig": string(pool.UserData),
					},
					"machineType":  pool.MachineType,
					"image":        image,
					"sshPublicKey": string(w.worker.Spec.SSHPublicKey),
				}, disks)
			)

			networkConfig := map[string]interface{}{
				"vnet":   infrastructureStatus.Networks.VNet.Name,
				"subnet": nodesSubnet.Name,
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

			if availabilitySetID != nil {
				machineClassSpec["availabilitySetID"] = *availabilitySetID
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
			machineClassSpec["secret"].(map[string]interface{})[azure.ClientIDKey] = string(machineClassSecretData[machinev1alpha1.AzureClientID])
			machineClassSpec["secret"].(map[string]interface{})[azure.ClientSecretKey] = string(machineClassSecretData[machinev1alpha1.AzureClientSecret])
			machineClassSpec["secret"].(map[string]interface{})[azure.SubscriptionIDKey] = string(machineClassSecretData[machinev1alpha1.AzureSubscriptionID])
			machineClassSpec["secret"].(map[string]interface{})[azure.TenantIDKey] = string(machineClassSecretData[machinev1alpha1.AzureTenantID])

			return machineDeployment, machineClassSpec
		}

		// Availability Set
		if !infrastructureStatus.Zoned {
			machineDeployment, machineClassSpec := generateMachineClassAndDeployment(nil, &nodesAvailabilitySet.ID)
			machineDeployments = append(machineDeployments, machineDeployment)
			machineClasses = append(machineClasses, machineClassSpec)
			continue
		}

		// Availability Zones
		zoneCount := len(pool.Zones)
		for zoneIndex, zone := range pool.Zones {
			info := &zoneInfo{
				name:  zone,
				index: int32(zoneIndex),
				count: int32(zoneCount),
			}

			machineDeployment, machineClassSpec := generateMachineClassAndDeployment(info, nil)
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

// getVmTags returns a map of vm tags
func (w *workerDelegate) getVmTags(pool extensionsv1alpha1.WorkerPool) map[string]string {
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
			return *dataVolumes[i].Name < *dataVolumes[j].Name
		})

		for i, volume := range dataVolumes {
			volumeSize, err := worker.DiskSize(volume.Size)
			if err != nil {
				return nil, err
			}
			disk := map[string]interface{}{
				"name":       *volume.Name,
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

func computeAdditionalHashData(pool extensionsv1alpha1.WorkerPool) []string {
	var additionalData []string

	for _, dv := range pool.DataVolumes {
		additionalData = append(additionalData, dv.Size)

		if dv.Type != nil {
			additionalData = append(additionalData, *dv.Type)
		}
	}

	return additionalData
}
