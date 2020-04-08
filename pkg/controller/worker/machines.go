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

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	azureapi "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	azureapihelper "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	genericworkeractuator "github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
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
		machineDeployments   = worker.MachineDeployments{}
		machineClasses       []map[string]interface{}
		machineImages        []apisazure.MachineImage
		nodesAvailabilitySet *azureapi.AvailabilitySet
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
	}

	for _, pool := range w.worker.Spec.Pools {
		var additionalData []string
		if infrastructureStatus.Identity != nil {
			additionalData = append(additionalData, infrastructureStatus.Identity.ID)
		}
		workerPoolHash, err := worker.WorkerPoolHash(pool, w.cluster, additionalData...)
		if err != nil {
			return err
		}

		urn, err := w.findMachineImage(pool.MachineImage.Name, pool.MachineImage.Version)
		if err != nil {
			return err
		}
		machineImages = appendMachineImage(machineImages, apisazure.MachineImage{
			Name:    pool.MachineImage.Name,
			Version: pool.MachineImage.Version,
			URN:     urn,
		})

		volumeSize, err := worker.DiskSize(pool.Volume.Size)
		if err != nil {
			return err
		}
		osDisk := map[string]interface{}{
			"size": volumeSize,
		}

		// In the past the volume type information was not passed to the machineclass.
		// In consequence the Machine controller manager has created machines always
		// with the default volume type of the requested machine type. Existing clusters
		// respectively their worker pools could have an invalid volume configuration
		// which was not applied. To do not damage exisiting cluster we will set for
		// now the volume type only if it's a valid Azure volume type.
		// Otherwise we will still use the default volume of the machine type.
		if pool.Volume.Type != nil {
			if *pool.Volume.Type == "Standard_LRS" || *pool.Volume.Type == "StandardSSD_LRS" || *pool.Volume.Type == "Premium_LRS" {
				osDisk["type"] = *pool.Volume.Type
			}
		}

		image := map[string]interface{}{
			"urn": *urn,
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

				machineClassSpec = map[string]interface{}{
					"region":        w.worker.Spec.Region,
					"resourceGroup": infrastructureStatus.ResourceGroup.Name,
					"vnetName":      infrastructureStatus.Networks.VNet.Name,
					"subnetName":    nodesSubnet.Name,
					"tags": map[string]interface{}{
						"Name": w.worker.Namespace,
						fmt.Sprintf("kubernetes.io-cluster-%s", w.worker.Namespace): "1",
						"kubernetes.io-role-node":                                   "1",
					},
					"secret": map[string]interface{}{
						"cloudConfig": string(pool.UserData),
					},
					"machineType":  pool.MachineType,
					"image":        image,
					"osDisk":       osDisk,
					"sshPublicKey": string(w.worker.Spec.SSHPublicKey),
				}
			)
			if infrastructureStatus.Networks.VNet.ResourceGroup != nil {
				machineClassSpec["vnetResourceGroup"] = *infrastructureStatus.Networks.VNet.ResourceGroup
			}

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
			machineClassSpec["labels"] = map[string]string{
				v1beta1constants.GardenerPurpose: genericworkeractuator.GardenPurposeMachineClass,
			}
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
