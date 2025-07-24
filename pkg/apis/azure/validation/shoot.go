// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"encoding/json"
	"math"

	"github.com/gardener/gardener/pkg/apis/core"
	validationutils "github.com/gardener/gardener/pkg/utils/validation"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
)

const maxDataVolumeCount = 64

// ValidateNetworking validates the network settings of a Shoot.
func ValidateNetworking(networking *core.Networking, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if networking == nil {
		allErrs = append(allErrs, field.Required(fldPath, "networking field can't be empty for Azure shoots"))
		return allErrs
	}

	if networking.Nodes == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("nodes"), "a nodes CIDR must be provided for Azure shoots"))
	}
	if networking.Type != nil && *networking.Type == "calico" && networking.ProviderConfig != nil {
		networkConfig, err := decodeNetworkConfig(networking.ProviderConfig)
		if err != nil {
			allErrs = append(allErrs, field.InternalError(fldPath.Child("providerConfig"), err))
		} else if overlay, ok := networkConfig["overlay"].(map[string]interface{}); ok {
			if overlay["enabled"].(bool) {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("providerConfig").Child("overlay").Child("enabled"), "Calico overlay network is not supported on azure"))
			}
		}
	}

	if core.IsIPv6SingleStack(networking.IPFamilies) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("ipFamilies"), networking.IPFamilies, "IPv6 single-stack networking is not supported"))
	}

	if len(networking.IPFamilies) > 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("ipFamilies"), networking.IPFamilies, "dual-stack networking is not supported"))
	}

	return allErrs
}

func decodeNetworkConfig(network *runtime.RawExtension) (map[string]interface{}, error) {
	var networkConfig map[string]interface{}
	if network == nil || network.Raw == nil {
		return map[string]interface{}{}, nil
	}
	if err := json.Unmarshal(network.Raw, &networkConfig); err != nil {
		return nil, err
	}
	return networkConfig, nil
}

// ValidateWorkers validates the workers of a Shoot.
func ValidateWorkers(workers []core.Worker, infra *api.InfrastructureConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, worker := range workers {
		path := fldPath.Index(i)

		if worker.Volume == nil {
			allErrs = append(allErrs, field.Required(path.Child("volume"), "must not be nil"))
		} else {
			allErrs = append(allErrs, validateVolume(worker.Volume, path.Child("volume"))...)
		}

		if length := len(worker.DataVolumes); length > maxDataVolumeCount {
			allErrs = append(allErrs, field.TooMany(path.Child("dataVolumes"), length, maxDataVolumeCount))
		}
		for j, volume := range worker.DataVolumes {
			dataVolPath := path.Child("dataVolumes").Index(j)
			allErrs = append(allErrs, validateDataVolume(&volume, dataVolPath)...)
		}

		// Zones validation
		if infra.Zoned && len(worker.Zones) == 0 {
			allErrs = append(allErrs, field.Required(path.Child("zones"), "at least one zone must be configured for zoned clusters"))
			continue
		}

		if !infra.Zoned && len(worker.Zones) > 0 {
			allErrs = append(allErrs, field.Required(path.Child("zones"), "zones must not be specified for non zoned clusters"))
			continue
		}

		if len(worker.Zones) > math.MaxInt32 {
			allErrs = append(allErrs, field.Invalid(path.Child("zones"), len(worker.Zones), "too many zones"))
			continue
		}

		zones := sets.New[string]()
		for j, zone := range worker.Zones {
			if zones.Has(zone) {
				allErrs = append(allErrs, field.Invalid(path.Child("zones").Index(j), zone, "must only be specified once per worker group"))
				continue
			}
			zones.Insert(zone)
		}

		if !helper.IsUsingSingleSubnetLayout(infra) {
			infraZones := sets.Set[string]{}
			for _, zone := range infra.Networks.Zones {
				infraZones.Insert(helper.InfrastructureZoneToString(zone.Name))
			}

			for zoneIndex, workerZone := range worker.Zones {
				if !infraZones.Has(workerZone) {
					allErrs = append(allErrs, field.Invalid(path.Child("zones").Index(zoneIndex), workerZone, "zone configuration must be specified in \"infrastructureConfig.networks.zones\""))
				}
			}
		}
	}

	return allErrs
}

// ValidateWorkersUpdate validates updates on `workers`.
func ValidateWorkersUpdate(oldWorkers, newWorkers []core.Worker, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, newWorker := range newWorkers {
		for _, oldWorker := range oldWorkers {
			if newWorker.Name == oldWorker.Name {
				if validationutils.ShouldEnforceImmutability(newWorker.Zones, oldWorker.Zones) {
					allErrs = append(allErrs, apivalidation.ValidateImmutableField(newWorker.Zones, oldWorker.Zones, fldPath.Index(i).Child("zones"))...)
				}
				break
			}
		}
	}
	return allErrs
}

func validateVolume(vol *core.Volume, fldPath *field.Path) field.ErrorList {
	return validateVolumeFunc(vol.Type, vol.VolumeSize, vol.Encrypted, fldPath)
}

func validateDataVolume(vol *core.DataVolume, fldPath *field.Path) field.ErrorList {
	return validateVolumeFunc(vol.Type, vol.VolumeSize, vol.Encrypted, fldPath)
}

func validateVolumeFunc(volumeType *string, volumeSize string, encrypted *bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if volumeType == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "must not be empty"))
	}
	if volumeSize == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("size"), "must not be empty"))
	}
	if encrypted != nil {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("encrypted"), *encrypted, []string{}))
	}
	return allErrs
}
