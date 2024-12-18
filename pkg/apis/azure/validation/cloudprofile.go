// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"strings"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
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

	// validate all provider images fields
	for i, machineImage := range cpConfig.MachineImages {
		idxPath := machineImagesPath.Index(i)
		allErrs = append(allErrs, ValidateProviderMachineImage(idxPath, machineImage)...)
	}

	allErrs = append(allErrs, validateProviderImagesMapping(cpConfig, machineImages, machineImagesPath)...)

	return allErrs
}

// ValidateProviderMachineImage validates a CloudProfileConfig MachineImages entry.
func ValidateProviderMachineImage(validationPath *field.Path, machineImage apisazure.MachineImages) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(machineImage.Name) == 0 {
		allErrs = append(allErrs, field.Required(validationPath.Child("name"), "must provide a name"))
	}

	if len(machineImage.Versions) == 0 {
		allErrs = append(allErrs, field.Required(validationPath.Child("versions"), fmt.Sprintf("must provide at least one version for machine image %q", machineImage.Name)))
	}
	for j, version := range machineImage.Versions {
		jdxPath := validationPath.Child("versions").Index(j)

		if len(version.Version) == 0 {
			allErrs = append(allErrs, field.Required(jdxPath.Child("version"), "must provide a version"))
		}

		allErrs = append(allErrs, validateProvidedImageIdCount(version, jdxPath)...)

		if version.URN != nil {
			if len(*version.URN) == 0 {
				allErrs = append(allErrs, field.Required(jdxPath.Child("urn"), "urn cannot be empty when defined"))
			} else if len(strings.Split(*version.URN, ":")) != 4 {
				allErrs = append(allErrs, field.Invalid(jdxPath.Child("urn"), version.URN, "please use the format `Publisher:Offer:Sku:Version` for the urn"))
			}
		}
		if version.ID != nil && len(*version.ID) == 0 {
			allErrs = append(allErrs, field.Required(jdxPath.Child("id"), "id cannot be empty when defined"))
		}
		if version.CommunityGalleryImageID != nil {
			if len(*version.CommunityGalleryImageID) == 0 {
				allErrs = append(allErrs, field.Required(jdxPath.Child("communityGalleryImageID"), "communityGalleryImageID cannot be empty when defined"))
			} else if len(strings.Split(*version.CommunityGalleryImageID, "/")) != 7 {
				allErrs = append(allErrs, field.Invalid(jdxPath.Child("communityGalleryImageID"),
					version.CommunityGalleryImageID, "please use the format `/CommunityGalleries/<gallery id>/Images/<image id>/versions/<version id>` for the communityGalleryImageID"))
			} else if !strings.EqualFold(strings.Split(*version.CommunityGalleryImageID, "/")[1], "CommunityGalleries") {
				allErrs = append(allErrs, field.Invalid(jdxPath.Child("communityGalleryImageID"),
					version.CommunityGalleryImageID, "communityGalleryImageID must start with '/CommunityGalleries/' prefix"))
			}
		}

		if version.SharedGalleryImageID != nil {
			if len(*version.SharedGalleryImageID) == 0 {
				allErrs = append(allErrs, field.Required(jdxPath.Child("sharedGalleryImageID"), "SharedGalleryImageID cannot be empty when defined"))
			} else if len(strings.Split(*version.SharedGalleryImageID, "/")) != 7 {
				allErrs = append(allErrs, field.Invalid(jdxPath.Child("sharedGalleryImageID"),
					version.SharedGalleryImageID, "please use the format `/SharedGalleries/<sharedGalleryName>/Images/<sharedGalleryImageName>/Versions/<sharedGalleryImageVersionName>` for the SharedGalleryImageID"))
			} else if !strings.EqualFold(strings.Split(*version.SharedGalleryImageID, "/")[1], "SharedGalleries") {
				allErrs = append(allErrs, field.Invalid(jdxPath.Child("sharedGalleryImageID"),
					version.SharedGalleryImageID, "SharedGalleryImageID must start with '/SharedGalleries/' prefix"))
			}
		}

		if !slices.Contains(v1beta1constants.ValidArchitectures, *version.Architecture) {
			allErrs = append(allErrs, field.NotSupported(jdxPath.Child("architecture"), *version.Architecture, v1beta1constants.ValidArchitectures))
		}
	}

	return allErrs
}

// verify that for each cp image a provider image exists
func validateProviderImagesMapping(cpConfig *apisazure.CloudProfileConfig, machineImages []core.MachineImage, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	providerImageMap := utils.CreateMapFromSlice(cpConfig.MachineImages, func(mi apisazure.MachineImages) string { return mi.Name })
	for idx, image := range machineImages {
		providerImage, found := providerImageMap[image.Name]
		if found {
			allErrs = append(allErrs, validateVersionsMapping(providerImage.Versions, image.Versions, fldPath.Index(idx).Child("versions"))...)
		} else if len(image.Versions) > 0 {
			allErrs = append(allErrs, field.Required(fldPath, fmt.Sprintf("must provide a provider image mapping for image %q", image.Name)))
		}
	}

	return allErrs
}

// validate that for each image version and architecture a corresponding provider image exists
func validateVersionsMapping(providerVersions []apisazure.MachineImageVersion, versions []core.MachineImageVersion, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	isVersionArchAvailable := func(version string, architecture string) bool {
		for _, providerVersion := range providerVersions {
			if architecture == ptr.Deref(providerVersion.Architecture, v1beta1constants.ArchitectureAMD64) && version == providerVersion.Version {
				return true
			}
		}
		return false
	}

	for _, version := range versions {
		for _, architecture := range version.Architectures {
			if !isVersionArchAvailable(version.Version, architecture) {
				errorMessage := fmt.Sprintf("must provide an image mapping for version %q and architecture: %s", version.Version, architecture)
				allErrs = append(allErrs, field.Required(fldPath, errorMessage))
			}
		}
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
