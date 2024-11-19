// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"strings"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"k8s.io/utils/strings/slices"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
)

// ValidateCloudProfileConfig validates a CloudProfileConfig object.
func ValidateCloudProfileConfig(cpConfig *apisazure.CloudProfileConfig, machineImages []core.MachineImage, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateDomainCount(cpConfig.CountFaultDomains, fldPath.Child("countFaultDomains"))...)
	allErrs = append(allErrs, validateDomainCount(cpConfig.CountUpdateDomains, fldPath.Child("countUpdateDomains"))...)

	machineImagesPath := fldPath.Child("machineImages")
	if len(cpConfig.MachineImages) == 0 {
		allErrs = append(allErrs, field.Required(machineImagesPath, "must provide at least one machine image"))
		return allErrs
	}

	for _, image := range machineImages {
		var found bool
		for i, imageConfig := range cpConfig.MachineImages {
			if image.Name == imageConfig.Name {
				allErrs = append(allErrs, validateVersions(imageConfig.Versions, image.Versions, machineImagesPath.Index(i).Child("versions"))...)
				found = true
				break
			}
		}
		if !found && len(image.Versions) > 0 {
			allErrs = append(allErrs, field.Required(machineImagesPath, fmt.Sprintf("must provide an image mapping for image %q", image.Name)))
		}
	}

	return allErrs
}

func validateVersions(versionsConfig []apisazure.MachineImageVersion, versions []core.MachineImageVersion, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for _, version := range versions {
		for _, versionArch := range version.Architectures {
			var found bool
			for j, versionConfig := range versionsConfig {
				jdxPath := fldPath.Index(j)
				if versionArch == ptr.Deref(versionConfig.Architecture, v1beta1constants.ArchitectureAMD64) && version.Version == versionConfig.Version {
					allErrs = append(allErrs, validateCpConfigMachineImageVersion(versionConfig, jdxPath)...)
					found = true
					break
				}
			}
			if !found {
				allErrs = append(allErrs, field.Required(fldPath, fmt.Sprintf("must provide an image mapping for version %q and architecture: %s", version.Version, versionArch)))
			}
		}
	}

	return allErrs
}

func validateCpConfigMachineImageVersion(version apisazure.MachineImageVersion, versionPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(version.Version) == 0 {
		allErrs = append(allErrs, field.Required(versionPath.Child("version"), "must provide a version"))
	}

	allErrs = append(allErrs, validateProvidedImageIdCount(version, versionPath)...)

	if version.URN != nil {
		if len(*version.URN) == 0 {
			allErrs = append(allErrs, field.Required(versionPath.Child("urn"), "urn cannot be empty when defined"))
		} else if len(strings.Split(*version.URN, ":")) != 4 {
			allErrs = append(allErrs, field.Invalid(versionPath.Child("urn"), version.URN, "please use the format `Publisher:Offer:Sku:Version` for the urn"))
		}
	}
	if version.ID != nil && len(*version.ID) == 0 {
		allErrs = append(allErrs, field.Required(versionPath.Child("id"), "id cannot be empty when defined"))
	}
	if version.CommunityGalleryImageID != nil {
		if len(*version.CommunityGalleryImageID) == 0 {
			allErrs = append(allErrs, field.Required(versionPath.Child("communityGalleryImageID"), "communityGalleryImageID cannot be empty when defined"))
		} else if len(strings.Split(*version.CommunityGalleryImageID, "/")) != 7 {
			allErrs = append(allErrs, field.Invalid(versionPath.Child("communityGalleryImageID"),
				version.CommunityGalleryImageID, "please use the format `/CommunityGalleries/<gallery id>/Images/<image id>/versions/<version id>` for the communityGalleryImageID"))
		} else if !strings.EqualFold(strings.Split(*version.CommunityGalleryImageID, "/")[1], "CommunityGalleries") {
			allErrs = append(allErrs, field.Invalid(versionPath.Child("communityGalleryImageID"),
				version.CommunityGalleryImageID, "communityGalleryImageID must start with '/CommunityGalleries/' prefix"))
		}
	}

	if version.SharedGalleryImageID != nil {
		if len(*version.SharedGalleryImageID) == 0 {
			allErrs = append(allErrs, field.Required(versionPath.Child("sharedGalleryImageID"), "SharedGalleryImageID cannot be empty when defined"))
		} else if len(strings.Split(*version.SharedGalleryImageID, "/")) != 7 {
			allErrs = append(allErrs, field.Invalid(versionPath.Child("sharedGalleryImageID"),
				version.SharedGalleryImageID, "please use the format `/SharedGalleries/<sharedGalleryName>/Images/<sharedGalleryImageName>/Versions/<sharedGalleryImageVersionName>` for the SharedGalleryImageID"))
		} else if !strings.EqualFold(strings.Split(*version.SharedGalleryImageID, "/")[1], "SharedGalleries") {
			allErrs = append(allErrs, field.Invalid(versionPath.Child("sharedGalleryImageID"),
				version.SharedGalleryImageID, "SharedGalleryImageID must start with '/SharedGalleries/' prefix"))
		}
	}

	if !slices.Contains(v1beta1constants.ValidArchitectures, *version.Architecture) {
		allErrs = append(allErrs, field.NotSupported(versionPath.Child("architecture"), *version.Architecture, v1beta1constants.ValidArchitectures))
	}
	return allErrs
}

// validateProvidedImageIdCount validates that only one of urn/id/communityGalleryImageID/sharedGalleryImageID is provided
func validateProvidedImageIdCount(version apisazure.MachineImageVersion, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	idCount := 0

	if version.URN != nil {
		idCount++
	}

	if version.ID != nil {
		idCount++
	}

	if version.CommunityGalleryImageID != nil {
		idCount++
	}

	if version.SharedGalleryImageID != nil {
		idCount++
	}

	if idCount != 1 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide either urn, id, sharedGalleryImageID or communityGalleryImageID"))
	}

	return allErrs
}

func validateDomainCount(domainCount []apisazure.DomainCount, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(domainCount) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one domain count"))
	}

	for i, count := range domainCount {
		idxPath := fldPath.Index(i)
		regionPath := idxPath.Child("region")
		countPath := idxPath.Child("count")

		if len(count.Region) == 0 {
			allErrs = append(allErrs, field.Required(regionPath, "must provide a region"))
		}
		if count.Count < 0 {
			allErrs = append(allErrs, field.Invalid(countPath, count.Count, "count must not be negative"))
		}
	}

	return allErrs
}
