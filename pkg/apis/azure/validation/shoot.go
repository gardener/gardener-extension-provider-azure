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
	"fmt"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"

	"github.com/Masterminds/semver"
	"github.com/gardener/gardener/pkg/apis/core"
	validationutils "github.com/gardener/gardener/pkg/utils/validation"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const maxDataVolumeCount = 64

var k8sV121 *semver.Version

func init() {
	k8sV121 = semver.MustParse("v1.21.0")
}

// ValidateNetworking validates the network settings of a Shoot.
func ValidateNetworking(networking core.Networking, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if networking.Nodes == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("nodes"), "a nodes CIDR must be provided for Azure shoots"))
	}

	return allErrs
}

// ValidateWorkers validates the workers of a Shoot.
func ValidateWorkers(workers []core.Worker, infra *api.InfrastructureConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	csiMigrationVersion, err := semver.NewVersion(azure.CSIMigrationKubernetesVersion)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, err))
		return allErrs
	}

	for i, worker := range workers {
		path := fldPath.Index(i)

		// Ensure the kubelet version is not lower than the version in which the extension performs CSI migration.
		if worker.Kubernetes != nil && worker.Kubernetes.Version != nil {
			versionPath := path.Child("kubernetes", "version")

			v, err := semver.NewVersion(*worker.Kubernetes.Version)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(versionPath, *worker.Kubernetes.Version, err.Error()))
				return allErrs
			}

			if v.LessThan(csiMigrationVersion) {
				allErrs = append(allErrs, field.Forbidden(versionPath, fmt.Sprintf("cannot use kubelet version (%s) lower than CSI migration version (%s)", v.String(), csiMigrationVersion.String())))
			}
		}

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

		zones := sets.NewString()
		for j, zone := range worker.Zones {
			if zones.Has(zone) {
				allErrs = append(allErrs, field.Invalid(path.Child("zones").Index(j), zone, "must only be specified once per worker group"))
				continue
			}
			zones.Insert(zone)
		}

		if !helper.IsUsingSingleSubnetLayout(infra) {
			infraZones := sets.String{}
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

// ValidateUpgradeV120ToV121 prevents that Shoots get updated from k8s v1.20 to v1.21.
// TODO(dkistner) remove this once csi-driver async operation issues are resolved.
func ValidateUpgradeV120ToV121(newShoot, oldShoot *core.Shoot, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	oldShootVersion, err := semver.NewVersion(oldShoot.Spec.Kubernetes.Version)
	if err != nil {
		return append(allErrs, field.Invalid(fldPath, oldShoot.Spec.Kubernetes.Version, "could not parse kubernetes version"))
	}

	newShootVersion, err := semver.NewVersion(newShoot.Spec.Kubernetes.Version)
	if err != nil {
		return append(allErrs, field.Invalid(fldPath, newShoot.Spec.Kubernetes.Version, "could not parse kubernetes version"))
	}

	if oldShootVersion.LessThan(k8sV121) && (newShootVersion.Equal(k8sV121) || newShootVersion.GreaterThan(k8sV121)) {
		if value, ok := newShoot.Annotations[azure.ForceK8sVersionUpgrade]; ok && value == "true" {
			return allErrs
		}
		allErrs = append(allErrs, field.Forbidden(fldPath, "upgrade Kubernetes version from < v1.21 to >= v1.21.0 are disabled"))
	}

	return allErrs
}
