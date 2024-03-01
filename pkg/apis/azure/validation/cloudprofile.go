// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"strings"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/strings/slices"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
)

// ValidateCloudProfileConfig validates a CloudProfileConfig object.
func ValidateCloudProfileConfig(cloudProfile *apisazure.CloudProfileConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateDomainCount(cloudProfile.CountFaultDomains, fldPath.Child("countFaultDomains"))...)
	allErrs = append(allErrs, validateDomainCount(cloudProfile.CountUpdateDomains, fldPath.Child("countUpdateDomains"))...)

	machineImagesPath := fldPath.Child("machineImages")
	if len(cloudProfile.MachineImages) == 0 {
		allErrs = append(allErrs, field.Required(machineImagesPath, "must provide at least one machine image"))
	}
	for i, machineImage := range cloudProfile.MachineImages {
		idxPath := machineImagesPath.Index(i)

		if len(machineImage.Name) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), "must provide a name"))
		}

		if len(machineImage.Versions) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("versions"), fmt.Sprintf("must provide at least one version for machine image %q", machineImage.Name)))
		}
		for j, version := range machineImage.Versions {
			jdxPath := idxPath.Child("versions").Index(j)

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
