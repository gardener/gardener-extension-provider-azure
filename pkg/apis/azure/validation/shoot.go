// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validation

import (
	"github.com/gardener/gardener-extension-networking-calico/pkg/apis/calico"
	calicopkg "github.com/gardener/gardener-extension-networking-calico/pkg/calico"
	"github.com/gardener/gardener/extensions/pkg/util"
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
func ValidateNetworking(decoder runtime.Decoder, networking *core.Networking, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if networking == nil {
		allErrs = append(allErrs, field.Required(fldPath, "networking field can't be empty for Azure shoots"))
		return allErrs
	}

	if networking.Nodes == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("nodes"), "a nodes CIDR must be provided for Azure shoots"))
	}
	if networking.Type != nil && *networking.Type == calicopkg.ReleaseName {
		networkConfig, err := decodeCalicoNetworkingConfig(decoder, networking.ProviderConfig)
		if err != nil {
			allErrs = append(allErrs, field.InternalError(fldPath.Child("providerConfig"), err))
		} else if networkConfig.Overlay != nil && networkConfig.Overlay.Enabled {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("providerConfig").Child("overlay").Child("enabled"), "Calico overlay network is not supported on azure"))
		}
	}

	return allErrs
}

func decodeCalicoNetworkingConfig(decoder runtime.Decoder, network *runtime.RawExtension) (*calico.NetworkConfig, error) {
	networkConfig := &calico.NetworkConfig{}
	if network != nil && network.Raw != nil {
		if err := util.Decode(decoder, network.Raw, networkConfig); err != nil {
			return nil, err
		}
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
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("encrypted"), *encrypted, nil))
	}
	return allErrs
}
