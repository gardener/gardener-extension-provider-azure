// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	genericworkeractuator "github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/charts"
	azureapi "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	azureapihelper "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

const azureCSIDiskDriverTopologyKey = "topology.disk.csi.azure.com/zone"

var tagRegex = regexp.MustCompile(`[<>%\\&?/ ]`)

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

	return w.seedChartApplier.ApplyFromEmbeddedFS(ctx, charts.InternalChart, filepath.Join("internal", "machineclass"), w.worker.Namespace, "machineclass", kubernetes.Values(map[string]interface{}{"machineClasses": w.machineClasses}))
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

	_, nodesSubnet, err := azureapihelper.FindSubnetByPurposeAndZone(infrastructureStatus.Networks.Subnets, azureapi.PurposeNodes, nil)
	if err != nil {
		return err
	}

	for _, pool := range w.worker.Spec.Pools {
		// Get the vmo dependency from the worker status if exists.
		vmoDependency, err := w.determineWorkerPoolVmoDependency(ctx, infrastructureStatus, workerStatus, pool.Name)
		if err != nil {
			return err
		}

		arch := ptr.Deref(pool.Architecture, v1beta1constants.ArchitectureAMD64)

		machineImage, err := w.findMachineImage(pool.MachineImage.Name, pool.MachineImage.Version, &arch)
		if err != nil {
			return err
		}
		machineImages = appendMachineImage(machineImages, azureapi.MachineImage{
			Name:                     pool.MachineImage.Name,
			Version:                  pool.MachineImage.Version,
			AcceleratedNetworking:    machineImage.AcceleratedNetworking,
			Architecture:             &arch,
			SkipMarketplaceAgreement: machineImage.SkipMarketplaceAgreement,
			Image: azureapi.Image{
				URN:                     machineImage.URN,
				ID:                      machineImage.ID,
				CommunityGalleryImageID: machineImage.CommunityGalleryImageID,
				SharedGalleryImageID:    machineImage.SharedGalleryImageID,
			},
		})

		image := map[string]interface{}{}
		if machineImage.URN != nil {
			image["urn"] = *machineImage.URN
			if ok := ptr.Deref(machineImage.SkipMarketplaceAgreement, false); ok {
				image["skipMarketplaceAgreement"] = ok
			}
		} else if machineImage.CommunityGalleryImageID != nil {
			image["communityGalleryImageID"] = *machineImage.CommunityGalleryImageID
		} else if machineImage.SharedGalleryImageID != nil {
			image["sharedGalleryImageID"] = *machineImage.SharedGalleryImageID
		} else {
			image["id"] = *machineImage.ID
		}

		workerConfig := azureapi.WorkerConfig{}
		if pool.ProviderConfig != nil && pool.ProviderConfig.Raw != nil {
			if _, _, err := w.decoder.Decode(pool.ProviderConfig.Raw, nil, &workerConfig); err != nil {
				return fmt.Errorf("could not decode provider config: %+v", err)
			}
		}

		disks, err := computeDisks(pool, workerConfig.DataVolumes)
		if err != nil {
			return err
		}

		userData, err := worker.FetchUserData(ctx, w.client, w.worker.Namespace, pool)
		if err != nil {
			return err
		}

		generateMachineClassAndDeployment := func(zone *zoneInfo, machineSet *machineSetInfo, subnetName, workerPoolHash string, workerConfig *azureapi.WorkerConfig) (worker.MachineDeployment, map[string]interface{}) {
			var (
				machineDeployment = worker.MachineDeployment{
					Minimum:              pool.Minimum,
					Maximum:              pool.Maximum,
					Priority:             pool.Priority,
					Labels:               addTopologyLabel(pool.Labels, w.worker.Spec.Region, zone),
					Annotations:          pool.Annotations,
					Taints:               pool.Taints,
					MachineConfiguration: genericworkeractuator.ReadMachineConfiguration(pool),
				}

				machineClassSpec = utils.MergeMaps(map[string]interface{}{
					"region":        w.worker.Spec.Region,
					"resourceGroup": infrastructureStatus.ResourceGroup.Name,
					"tags":          w.getVMTags(pool),
					"secret": map[string]interface{}{
						"cloudConfig": string(userData),
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

			cloudConfiguration, err := azureclient.CloudConfiguration(nil, &w.worker.Spec.Region)
			if err == nil {
				machineClassSpec["cloudConfiguration"] = map[string]interface{}{
					"name": cloudConfiguration.Name,
				}
			}

			if infrastructureStatus.Networks.VNet.ResourceGroup != nil {
				networkConfig["vnetResourceGroup"] = *infrastructureStatus.Networks.VNet.ResourceGroup
			}
			if ptr.Deref(machineImage.AcceleratedNetworking, false) && w.isMachineTypeSupportingAcceleratedNetworking(pool.MachineType) && acceleratedNetworkAllowed {
				networkConfig["acceleratedNetworking"] = true
			}
			machineClassSpec["network"] = networkConfig

			updateConfiguration := machinev1alpha1.UpdateConfiguration{
				MaxUnavailable: &pool.MaxUnavailable,
				MaxSurge:       &pool.MaxSurge,
			}

			if zone != nil {
				machineDeployment.Minimum = worker.DistributeOverZones(zone.index, pool.Minimum, zone.count)
				machineDeployment.Maximum = worker.DistributeOverZones(zone.index, pool.Maximum, zone.count)
				updateConfiguration = machinev1alpha1.UpdateConfiguration{
					MaxUnavailable: ptr.To(worker.DistributePositiveIntOrPercent(zone.index, pool.MaxUnavailable, zone.count, pool.Minimum)),
					MaxSurge:       ptr.To(worker.DistributePositiveIntOrPercent(zone.index, pool.MaxSurge, zone.count, pool.Maximum)),
				}
				machineClassSpec["zone"] = zone.name
			}

			machineDeploymentStrategy := machinev1alpha1.MachineDeploymentStrategy{
				Type: machinev1alpha1.RollingUpdateMachineDeploymentStrategyType,
				RollingUpdate: &machinev1alpha1.RollingUpdateMachineDeployment{
					UpdateConfiguration: updateConfiguration,
				},
			}

			machineDeployment.Strategy = machineDeploymentStrategy

			if workerConfig.DiagnosticsProfile != nil {
				diagnosticProfile := map[string]interface{}{
					"enabled": workerConfig.DiagnosticsProfile.Enabled,
				}
				if workerConfig.DiagnosticsProfile.StorageURI != nil {
					diagnosticProfile["storageURI"] = workerConfig.DiagnosticsProfile.StorageURI
				}
				machineClassSpec["diagnosticsProfile"] = diagnosticProfile
			}

			if pool.NodeTemplate != nil {
				//	Currently Zone field is mandatory, and passing it an
				//	empty string turns it to `null` string during marshalling which fails CRD validation
				//	so setting it to a dummy value `no-zone`
				//	TODO: Zone field in nodeTemplate optional and not to pass it in case of non-zonal setup
				zoneName := "no-zone"

				if zone != nil {
					zoneName = w.worker.Spec.Region + "-" + zone.name
				}

				if workerConfig.NodeTemplate != nil {
					machineClassSpec["nodeTemplate"] = machinev1alpha1.NodeTemplate{
						Capacity:     workerConfig.NodeTemplate.Capacity,
						InstanceType: pool.MachineType,
						Region:       w.worker.Spec.Region,
						Zone:         zoneName,
						Architecture: &arch,
					}
				} else if pool.NodeTemplate != nil {
					machineClassSpec["nodeTemplate"] = machinev1alpha1.NodeTemplate{
						Capacity:     pool.NodeTemplate.Capacity,
						InstanceType: pool.MachineType,
						Region:       w.worker.Spec.Region,
						Zone:         zoneName,
						Architecture: &arch,
					}
				}
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
			machineClassSpec["labels"] = map[string]string{v1beta1constants.GardenerPurpose: v1beta1constants.GardenPurposeMachineClass}

			if pool.MachineImage.Name != "" && pool.MachineImage.Version != "" {
				machineClassSpec["operatingSystem"] = map[string]interface{}{
					"operatingSystemName":    pool.MachineImage.Name,
					"operatingSystemVersion": strings.ReplaceAll(pool.MachineImage.Version, "+", "_"),
				}
			}

			// special processing of CVMs.
			if isConfidentialVM(pool) {
				machineClassSpec["securityProfile"] = map[string]interface{}{
					"securityType": string(armcompute.SecurityTypesConfidentialVM),
					"uefiSettings": map[string]interface{}{
						"vtpmEnabled": true,
					},
				}
			}

			machineDeployment.ClusterAutoscalerAnnotations = extensionsv1alpha1helper.GetMachineDeploymentClusterAutoscalerAnnotations(pool.ClusterAutoscaler)

			return machineDeployment, machineClassSpec
		}

		workerPoolHash, err := w.generateWorkerPoolHash(pool, infrastructureStatus, vmoDependency, nil)
		if err != nil {
			return err
		}

		// VMO
		if vmoDependency != nil {
			machineDeployment, machineClassSpec := generateMachineClassAndDeployment(nil, &machineSetInfo{
				id:   vmoDependency.ID,
				kind: "vmo",
			}, nodesSubnet.Name, workerPoolHash, &workerConfig)
			machineDeployments = append(machineDeployments, machineDeployment)
			machineClasses = append(machineClasses, machineClassSpec)
			continue
		}

		// AvailabilitySet
		if !infrastructureStatus.Zoned {
			nodesAvailabilitySet, err := azureapihelper.FindAvailabilitySetByPurpose(infrastructureStatus.AvailabilitySets, azureapi.PurposeNodes)
			if err != nil {
				return err
			}

			// Do not enable accelerated networking for AvSet cluster.
			// This is necessary to avoid `ExistingAvailabilitySetWasNotDeployedOnAcceleratedNetworkingEnabledCluster` error.
			acceleratedNetworkAllowed = false

			machineDeployment, machineClassSpec := generateMachineClassAndDeployment(nil, &machineSetInfo{
				id:   nodesAvailabilitySet.ID,
				kind: "availabilityset",
			}, nodesSubnet.Name, workerPoolHash, &workerConfig)
			machineDeployments = append(machineDeployments, machineDeployment)
			machineClasses = append(machineClasses, machineClassSpec)
			continue
		}

		// Availability Zones
		zoneCount := len(pool.Zones)
		for zoneIndex, zone := range pool.Zones {
			if infrastructureStatus.Networks.Layout == azureapi.NetworkLayoutMultipleSubnet {
				_, nodesSubnet, err = azureapihelper.FindSubnetByPurposeAndZone(infrastructureStatus.Networks.Subnets, azureapi.PurposeNodes, &zone)
				if err != nil {
					return err
				}

				if nodesSubnet.Migrated {
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
			}
			machineDeployment, machineClassSpec := generateMachineClassAndDeployment(&zoneInfo{
				name:  zone,
				index: int32(zoneIndex), // #nosec: G115 - We validate if pool zones exceeds max_int32.
				count: int32(zoneCount), // #nosec: G115 - We validate if pool zones exceeds max_int32.
			}, nil, nodesSubnet.Name, workerPoolHash, &workerConfig)
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

func computeDisks(pool extensionsv1alpha1.WorkerPool, dataVolumesConfig []azureapi.DataVolume) (map[string]interface{}, error) {
	// handle root disk
	volumeSize, err := worker.DiskSize(pool.Volume.Size)
	if err != nil {
		return nil, err
	}
	osDisk := map[string]interface{}{
		"size": volumeSize,
	}
	if pool.Volume != nil && pool.Volume.Type != nil {
		osDisk["type"] = *pool.Volume.Type
	}

	if isConfidentialVM(pool) {
		osDisk["securityProfile"] = map[string]interface{}{
			"securityEncryptionType": string(armcompute.SecurityEncryptionTypesVMGuestStateOnly),
		}
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
				"lun":        int32(i), // #nosec: G115 - There is a disk validation for lun < 0 in mcm.
				"diskSizeGB": volumeSize,
				"caching":    "None",
			}
			if volume.Type != nil {
				disk["storageAccountType"] = *volume.Type
			}
			applyWorkerConfig(volume.Name, disk, dataVolumesConfig)
			dataDisks = append(dataDisks, disk)
		}

		disks["dataDisks"] = dataDisks
	}

	return disks, nil
}

func applyWorkerConfig(diskName string, dataDisk map[string]interface{}, dataVolumeConfigs []azureapi.DataVolume) {
	for _, config := range dataVolumeConfigs {
		imageRef := config.ImageRef
		if imageRef != nil && config.Name == diskName {
			if imageRef.URN != nil {
				dataDisk["imageRef"] = map[string]interface{}{"urn": *imageRef.URN}
			} else if imageRef.CommunityGalleryImageID != nil {
				dataDisk["imageRef"] = map[string]interface{}{"communityGalleryImageID": *imageRef.CommunityGalleryImageID}
			} else if imageRef.SharedGalleryImageID != nil {
				dataDisk["imageRef"] = map[string]interface{}{"sharedGalleryImageID": *imageRef.SharedGalleryImageID}
			} else if imageRef.ID != nil {
				dataDisk["imageRef"] = map[string]interface{}{"id": imageRef.ID}
			}
		}
	}
}

// SanitizeAzureVMTag will sanitize the tag base on the azure tag Restrictions
// refer: https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/tag-resources#limitations
func SanitizeAzureVMTag(label string) string {
	return tagRegex.ReplaceAllString(strings.ToLower(label), "_")
}

func addTopologyLabel(labels map[string]string, region string, zone *zoneInfo) map[string]string {
	if zone != nil {
		return utils.MergeStringMaps(labels, map[string]string{azureCSIDiskDriverTopologyKey: region + "-" + zone.name})
	}
	return labels
}

func (w *workerDelegate) generateWorkerPoolHash(pool extensionsv1alpha1.WorkerPool, infrastructureStatus *azureapi.InfrastructureStatus, vmoDependency *azureapi.VmoDependency, subnetName *string) (string, error) {
	var additionalHashData []string

	// Integrate data disks/volumes in the hash.
	for _, dv := range pool.DataVolumes {
		additionalHashData = append(additionalHashData, dv.Size)
		if dv.Type != nil {
			additionalHashData = append(additionalHashData, *dv.Type)
		}
		// We exclude volume.Encrypted from the hash calculation because Azure disks are encrypted by default,
		// and the field does not influence disk encryption behavior.
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

	// Include additional data for new worker-pool hash generation.
	// See https://github.com/gardener/gardener/issues/9699 for more details
	additionalHashDataV2 := append(additionalHashData, w.workerPoolHashDataV2(pool)...)

	return worker.WorkerPoolHash(pool, w.cluster, additionalHashData, additionalHashDataV2, nil)
}

// workerPoolHashDataV2 adds additional provider-specific data points to consider to the given data.
func (w workerDelegate) workerPoolHashDataV2(pool extensionsv1alpha1.WorkerPool) []string {
	// in the future, we may not calculate a hash for the whole ProviderConfig
	// for example volume field changes could be done in place, but MCM needs to support it
	if pool.ProviderConfig != nil && pool.ProviderConfig.Raw != nil {
		return []string{string(pool.ProviderConfig.Raw)}
	}

	return nil
}

// TODO: Remove when we have support for VM Capabilities
func isConfidentialVM(pool extensionsv1alpha1.WorkerPool) bool {
	for _, v := range azure.ConfidentialVMFamilyPrefixes {
		if strings.HasPrefix(strings.ToLower(pool.MachineType), strings.ToLower(v)) {
			return true
		}
	}
	return false
}
