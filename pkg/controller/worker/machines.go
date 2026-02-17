// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Masterminds/semver/v3"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	genericworkeractuator "github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/charts"
	azureapi "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	azureapihelper "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

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

	return w.seedChartApplier.ApplyFromEmbeddedFS(ctx, charts.InternalChart, filepath.Join("internal", "machineclass"), w.worker.Namespace, "machineclass", kubernetes.Values(map[string]any{"machineClasses": w.machineClasses}))
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
		machineClasses            []map[string]any
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
		vmoDependency, err := w.determineWorkerPoolVmoDependency(ctx, infrastructureStatus, workerStatus, pool.Name, pool.UpdateStrategy)
		if err != nil {
			return err
		}

		arch := ptr.Deref(pool.Architecture, v1beta1constants.ArchitectureAMD64)

		machineTypeFromCloudProfile := gardencorev1beta1helper.FindMachineTypeByName(w.cluster.CloudProfile.Spec.MachineTypes, pool.MachineType)
		if machineTypeFromCloudProfile == nil {
			return fmt.Errorf("machine type %q not found in cloud profile %q", pool.MachineType, w.cluster.CloudProfile.Name)
		}

		machineImage, err := w.selectMachineImageForWorkerPool(pool.MachineImage.Name, pool.MachineImage.Version, &arch, machineTypeFromCloudProfile.Capabilities)

		if err != nil {
			return err
		}

		machineImages = ensureUniformMachineImages(machineImages, w.cluster.CloudProfile.Spec.MachineCapabilities)
		machineImages = appendMachineImage(machineImages, *machineImage, w.cluster.CloudProfile.Spec.MachineCapabilities)

		image := map[string]any{}
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

		disks, err := computeDisks(pool, workerConfig.DataVolumes, workerConfig.Volume)
		if err != nil {
			return err
		}

		userData, err := worker.FetchUserData(ctx, w.client, w.worker.Namespace, pool)
		if err != nil {
			return err
		}

		generateMachineClassAndDeployment := func(zone *zoneInfo, machineSet *machineSetInfo, subnetName, workerPoolHash string, workerConfig *azureapi.WorkerConfig) (worker.MachineDeployment, map[string]any) {
			var (
				machineDeployment = worker.MachineDeployment{
					PoolName:             pool.Name,
					Minimum:              pool.Minimum,
					Maximum:              pool.Maximum,
					Priority:             pool.Priority,
					Labels:               addTopologyLabel(pool.Labels, w.worker.Spec.Region, zone),
					Annotations:          pool.Annotations,
					Taints:               pool.Taints,
					MachineConfiguration: genericworkeractuator.ReadMachineConfiguration(pool),
				}

				machineClassSpec = utils.MergeMaps(map[string]any{
					"region":        w.worker.Spec.Region,
					"resourceGroup": infrastructureStatus.ResourceGroup.Name,
					"tags":          w.getVMTags(pool),
					"secret": map[string]any{
						"cloudConfig": string(userData),
					},
					"credentialsSecretRef": map[string]any{
						"name":      w.worker.Spec.SecretRef.Name,
						"namespace": w.worker.Spec.SecretRef.Namespace,
					},
					"machineType":  pool.MachineType,
					"image":        image,
					"sshPublicKey": string(w.worker.Spec.SSHPublicKey),
				}, disks)
			)

			networkConfig := map[string]any{
				"vnet":   infrastructureStatus.Networks.VNet.Name,
				"subnet": subnetName,
			}

			cloudConfiguration, err := azureclient.CloudConfiguration(nil, &w.worker.Spec.Region)
			if err == nil {
				machineClassSpec["cloudConfiguration"] = map[string]any{
					"name": cloudConfiguration.Name,
				}
			}

			if infrastructureStatus.Networks.VNet.ResourceGroup != nil {
				networkConfig["vnetResourceGroup"] = *infrastructureStatus.Networks.VNet.ResourceGroup
			}

			if acceleratedNetworkAllowed {
				if len(w.cluster.CloudProfile.Spec.MachineCapabilities) > 0 {
					defaultedMachineTypeCapabilities := gardencorev1beta1.GetCapabilitiesWithAppliedDefaults(machineTypeFromCloudProfile.Capabilities, w.cluster.CloudProfile.Spec.MachineCapabilities)
					defaultedImageCapabilities := gardencorev1beta1.GetCapabilitiesWithAppliedDefaults(machineImage.Capabilities, w.cluster.CloudProfile.Spec.MachineCapabilities)
					machineTypeSupportsAcceleratedNetworking := slices.Contains(defaultedMachineTypeCapabilities[azure.CapabilityNetworkName], azure.CapabilityNetworkAccelerated)
					machineImageSupportsAcceleratedNetworking := slices.Contains(defaultedImageCapabilities[azure.CapabilityNetworkName], azure.CapabilityNetworkAccelerated)
					if machineTypeSupportsAcceleratedNetworking && machineImageSupportsAcceleratedNetworking {
						networkConfig["acceleratedNetworking"] = true
					}
				} else {
					if ptr.Deref(machineImage.AcceleratedNetworking, false) && w.isMachineTypeSupportingAcceleratedNetworking(pool.MachineType) {
						networkConfig["acceleratedNetworking"] = true
					}
				}
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

			if gardencorev1beta1helper.IsUpdateStrategyInPlace(pool.UpdateStrategy) {
				machineDeploymentStrategy = machinev1alpha1.MachineDeploymentStrategy{
					Type: machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType,
					InPlaceUpdate: &machinev1alpha1.InPlaceUpdateMachineDeployment{
						UpdateConfiguration: updateConfiguration,
						OrchestrationType:   machinev1alpha1.OrchestrationTypeAuto,
					},
				}

				if gardencorev1beta1helper.IsUpdateStrategyManualInPlace(pool.UpdateStrategy) {
					machineDeploymentStrategy.InPlaceUpdate.OrchestrationType = machinev1alpha1.OrchestrationTypeManual
				}
			}

			machineDeployment.Strategy = machineDeploymentStrategy

			if workerConfig.DiagnosticsProfile != nil {
				diagnosticProfile := map[string]any{
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

				nodeTemplate := machinev1alpha1.NodeTemplate{
					Capacity:        pool.NodeTemplate.Capacity,
					VirtualCapacity: pool.NodeTemplate.VirtualCapacity,
					InstanceType:    pool.MachineType,
					Region:          w.worker.Spec.Region,
					Zone:            zoneName,
					Architecture:    &arch,
				}
				if workerConfig.NodeTemplate != nil {
					// Support providerConfig extended resources by copying into node template capacity and virtualCapacity
					maps.Copy(nodeTemplate.Capacity, workerConfig.NodeTemplate.Capacity)
					if workerConfig.NodeTemplate.VirtualCapacity != nil {
						if nodeTemplate.VirtualCapacity == nil {
							nodeTemplate.VirtualCapacity = corev1.ResourceList{}
						}
						maps.Copy(nodeTemplate.VirtualCapacity, workerConfig.NodeTemplate.VirtualCapacity)
					}
				}
				machineClassSpec["nodeTemplate"] = nodeTemplate
			}

			if machineSet != nil {
				machineClassSpec["machineSet"] = map[string]any{
					"kind": machineSet.kind,
					"id":   machineSet.id,
				}
			}

			if infrastructureStatus.Identity != nil {
				machineClassSpec["identityID"] = infrastructureStatus.Identity.ID
			}

			var (
				deploymentName = fmt.Sprintf("%s-%s", w.cluster.Shoot.Status.TechnicalID, pool.Name)
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
				machineClassSpec["operatingSystem"] = map[string]any{
					"operatingSystemName":    pool.MachineImage.Name,
					"operatingSystemVersion": strings.ReplaceAll(pool.MachineImage.Version, "+", "_"),
				}
			}

			// special processing of CVMs.
			if isConfidentialVM(pool) {
				machineClassSpec["securityProfile"] = map[string]any{
					"securityType": string(armcompute.SecurityTypesConfidentialVM),
					"uefiSettings": map[string]any{
						"vtpmEnabled": true,
					},
				}
			}

			if capacityReservation := workerConfig.CapacityReservation; capacityReservation != nil {
				capacityReservationConfig := map[string]any{}

				if capacityReservationGroupID := capacityReservation.CapacityReservationGroupID; capacityReservationGroupID != nil {
					capacityReservationConfig["capacityReservationGroupID"] = *capacityReservationGroupID
				}

				machineClassSpec["capacityReservation"] = capacityReservationConfig
			}

			machineDeployment.ClusterAutoscalerAnnotations = extensionsv1alpha1helper.GetMachineDeploymentClusterAutoscalerAnnotations(pool.ClusterAutoscaler)

			return machineDeployment, machineClassSpec
		}

		workerPoolHash, err := w.generateWorkerPoolHash(pool, infrastructureStatus, vmoDependency, nil, &workerConfig)
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

		// Availability Zones
		zoneCount := len(pool.Zones)
		for zoneIndex, zone := range pool.Zones {
			if infrastructureStatus.Networks.Layout == azureapi.NetworkLayoutMultipleSubnet {
				_, nodesSubnet, err = azureapihelper.FindSubnetByPurposeAndZone(infrastructureStatus.Networks.Subnets, azureapi.PurposeNodes, &zone)
				if err != nil {
					return err
				}

				if nodesSubnet.Migrated {
					workerPoolHash, err = w.generateWorkerPoolHash(pool, infrastructureStatus, vmoDependency, nil, &workerConfig)
					if err != nil {
						return err
					}
				} else {
					workerPoolHash, err = w.generateWorkerPoolHash(pool, infrastructureStatus, vmoDependency, &nodesSubnet.Name, &workerConfig)
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
		"Name": w.cluster.Shoot.Status.TechnicalID,
		SanitizeAzureVMTag(fmt.Sprintf("kubernetes.io-cluster-%s", w.cluster.Shoot.Status.TechnicalID)): "1",
		SanitizeAzureVMTag("kubernetes.io-role-node"):                                                   "1",
	}
	for k, v := range pool.Labels {
		vmTags[SanitizeAzureVMTag(k)] = v
	}
	return vmTags
}

func computeDisks(pool extensionsv1alpha1.WorkerPool, dataVolumesConfig []azureapi.DataVolume, osDiskConfig *azureapi.Volume) (map[string]any, error) {
	// handle root disk
	volumeSize, err := worker.DiskSize(pool.Volume.Size)
	if err != nil {
		return nil, err
	}
	osDisk := map[string]any{
		"size": volumeSize,
		// TODO: undo setting cachingTypeNone after MCM supports passing nil as caching type to trigger defaults
		// https://github.com/gardener/machine-controller-manager-provider-azure/issues/214
		"caching": string(armcompute.CachingTypesNone),
	}
	if pool.Volume != nil && pool.Volume.Type != nil {
		osDisk["type"] = *pool.Volume.Type
	}

	if isConfidentialVM(pool) {
		osDisk["securityProfile"] = map[string]any{
			"securityEncryptionType": string(armcompute.SecurityEncryptionTypesVMGuestStateOnly),
		}
	}

	if osDiskConfig != nil && osDiskConfig.Caching != nil {
		osDisk["caching"] = *osDiskConfig.Caching
	}

	disks := map[string]any{
		"osDisk": osDisk,
	}

	// handle data disks
	var dataDisks []map[string]any
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
			disk := map[string]any{
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

func applyWorkerConfig(diskName string, dataDisk map[string]any, dataVolumeConfigs []azureapi.DataVolume) {
	for _, config := range dataVolumeConfigs {
		imageRef := config.ImageRef
		if imageRef != nil && config.Name == diskName {
			if imageRef.URN != nil {
				dataDisk["imageRef"] = map[string]any{"urn": *imageRef.URN}
			} else if imageRef.CommunityGalleryImageID != nil {
				dataDisk["imageRef"] = map[string]any{"communityGalleryImageID": *imageRef.CommunityGalleryImageID}
			} else if imageRef.SharedGalleryImageID != nil {
				dataDisk["imageRef"] = map[string]any{"sharedGalleryImageID": *imageRef.SharedGalleryImageID}
			} else if imageRef.ID != nil {
				dataDisk["imageRef"] = map[string]any{"id": imageRef.ID}
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
		return utils.MergeStringMaps(labels, map[string]string{azure.AzureCSIDiskDriverTopologyKey: region + "-" + zone.name})
	}
	return labels
}

func (w *workerDelegate) generateWorkerPoolHash(pool extensionsv1alpha1.WorkerPool, infrastructureStatus *azureapi.InfrastructureStatus, vmoDependency *azureapi.VmoDependency, subnetName *string, workerConfig *azureapi.WorkerConfig) (string, error) {
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
	hashDataV2, err := WorkerPoolHashDataV2(pool, workerConfig)
	if err != nil {
		return "", err
	}
	additionalHashDataV2 := append(additionalHashData, hashDataV2...)

	return worker.WorkerPoolHash(pool, w.cluster, additionalHashData, additionalHashDataV2, []string{})
}

// workerPoolHashDataV2 adds additional provider-specific data points to consider to the given data.
// Addition or Change in VirtualCapacity should NOT cause existing hash to change to prevent trigger of rollout.
// TODO: once the MCM supports Machine Hot-Update from the WorkerConfig, this hash data logic can be made smarter.
func WorkerPoolHashDataV2(pool extensionsv1alpha1.WorkerPool, workerConfig *azureapi.WorkerConfig) ([]string, error) {
	var useNewHashData bool
	if pool.KubernetesVersion != nil {
		poolK8sVersion, err := semver.NewVersion(*pool.KubernetesVersion)
		if err != nil {
			return nil, err
		}
		useNewHashData = versionutils.ConstraintK8sGreaterEqual134.Check(poolK8sVersion)
	}

	if useNewHashData && workerConfig != nil {
		return appendHashDataForWorkerConfig(nil, workerConfig), nil
	}

	if pool.ProviderConfig == nil || pool.ProviderConfig.Raw == nil {
		return nil, nil
	}

	// Addition or Change in VirtualCapacity should NOT cause existing hash to change to prevent trigger of rollout.
	if workerConfig != nil && workerConfig.NodeTemplate != nil && workerConfig.NodeTemplate.VirtualCapacity != nil {
		modifiedProviderConfig := stripVirtualCapacity(pool.ProviderConfig.Raw)
		return []string{string(modifiedProviderConfig)}, nil
	}

	// preserve legacy behaviour
	return []string{string(pool.ProviderConfig.Raw)}, nil
}

// appendHashDataForWorkerConfig appends individual WorkerConfig fields to hash data.
func appendHashDataForWorkerConfig(hashData []string, workerConfig *azureapi.WorkerConfig) []string {
	if workerConfig.NodeTemplate != nil {
		keys := slices.Sorted(maps.Keys(workerConfig.NodeTemplate.Capacity)) // ensure order
		for _, k := range keys {
			q := workerConfig.NodeTemplate.Capacity[k]
			hashData = append(hashData, fmt.Sprintf("%s=%d", k, q.Value()))
		}
	}
	if workerConfig.Volume != nil {
		if workerConfig.Volume.Caching != nil {
			hashData = append(hashData, *workerConfig.Volume.Caching)
		}
	}
	if workerConfig.DataVolumes != nil {
		for _, dv := range workerConfig.DataVolumes {
			hashData = append(hashData, dv.Name)
			if dv.ImageRef != nil {
				if dv.ImageRef.URN != nil {
					hashData = append(hashData, *dv.ImageRef.URN)
				}
				if dv.ImageRef.ID != nil {
					hashData = append(hashData, *dv.ImageRef.ID)
				}
				if dv.ImageRef.CommunityGalleryImageID != nil {
					hashData = append(hashData, *dv.ImageRef.CommunityGalleryImageID)
				}
				if dv.ImageRef.SharedGalleryImageID != nil {
					hashData = append(hashData, *dv.ImageRef.SharedGalleryImageID)
				}
			}
		}
	}
	if workerConfig.DiagnosticsProfile != nil {
		hashData = append(hashData, strconv.FormatBool(workerConfig.DiagnosticsProfile.Enabled))
		if workerConfig.DiagnosticsProfile.StorageURI != nil {
			hashData = append(hashData, *workerConfig.DiagnosticsProfile.StorageURI)
		}
	}
	if workerConfig.CapacityReservation != nil {
		if workerConfig.CapacityReservation.CapacityReservationGroupID != nil {
			hashData = append(hashData, *workerConfig.CapacityReservation.CapacityReservationGroupID)
		}
	}
	return hashData
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

// ensureUniformMachineImages ensures that all machine images are in the same format, either with or without Capabilities.
func ensureUniformMachineImages(images []azureapi.MachineImage, definitions []gardencorev1beta1.CapabilityDefinition) []azureapi.MachineImage {
	var uniformMachineImages []azureapi.MachineImage

	if len(definitions) == 0 {
		// transform images that were added with Capabilities to the legacy format without Capabilities
		for _, img := range images {
			if len(img.Capabilities) == 0 {
				// image is already legacy format
				uniformMachineImages = appendMachineImage(uniformMachineImages, img, definitions)
				continue
			}
			// transform to legacy format by using the Architecture capability if it exists
			var architecture *string
			if len(img.Capabilities[v1beta1constants.ArchitectureName]) > 0 {
				architecture = &img.Capabilities[v1beta1constants.ArchitectureName][0]
			}
			uniformMachineImages = appendMachineImage(uniformMachineImages, azureapi.MachineImage{
				Name:         img.Name,
				Version:      img.Version,
				Image:        img.Image,
				Architecture: architecture,
			}, definitions)
		}
		return uniformMachineImages
	}

	// transform images that were added without Capabilities to contain a MachineImageFlavor with defaulted Architecture
	for _, img := range images {
		if len(img.Capabilities) > 0 {
			// image is already in the new format with Capabilities
			uniformMachineImages = appendMachineImage(uniformMachineImages, img, definitions)
		} else {
			// add image as a capability set with defaulted Architecture
			architecture := ptr.Deref(img.Architecture, v1beta1constants.ArchitectureAMD64)
			uniformMachineImages = appendMachineImage(uniformMachineImages, azureapi.MachineImage{
				Name:         img.Name,
				Version:      img.Version,
				Image:        img.Image,
				Capabilities: gardencorev1beta1.Capabilities{v1beta1constants.ArchitectureName: []string{architecture}},
			}, definitions)
		}
	}
	return uniformMachineImages
}
