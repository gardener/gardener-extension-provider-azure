// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"strings"

	"github.com/gardener/gardener/extensions/pkg/util"
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

	allErrs = append(allErrs, validateProviderImagesMapping(cpConfig.MachineImages, machineImages, machineImagesPath)...)

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
func validateProviderImagesMapping(cpConfigImages []apisazure.MachineImages, machineImages []core.MachineImage, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	images := util.NewCoreImagesContext(machineImages)
	providerImages := NewProviderImagesContext(cpConfigImages)

	// for each image in the CloudProfile, check if it exists in the CloudProfileConfig
	for _, machineImage := range images.Images {
		if _, existsInParent := providerImages.GetImage(machineImage.Name); !existsInParent {
			allErrs = append(allErrs, field.Required(fldPath, fmt.Sprintf("must provide a provider image mapping for image %q", machineImage.Name)))
			continue
		}

		// validate that for each version and architecture of an image in the cloud profile a
		// corresponding provider specific image in the cloud profile config exists
		for _, version := range machineImage.Versions {
			for _, expectedArchitecture := range version.Architectures {
				if _, exists := providerImages.GetImageVersion(machineImage.Name, VersionArchitectureKey(version.Version, expectedArchitecture)); !exists {
					allErrs = append(allErrs, field.Required(fldPath.Child("versions"),
						fmt.Sprintf("must provide an image mapping for version %q and architecture: %s", version.Version, expectedArchitecture)))
				}
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

func providerMachineImageKey(v apisazure.MachineImageVersion) string {
	return VersionArchitectureKey(v.Version, ptr.Deref(v.Architecture, v1beta1constants.ArchitectureAMD64))
}

// VersionArchitectureKey returns a key for a version and architecture.
func VersionArchitectureKey(version, architecture string) string {
	return version + "-" + architecture
}

// NewProviderImagesContext creates a new images context for provider images.
func NewProviderImagesContext(providerImages []apisazure.MachineImages) *util.ImagesContext[apisazure.MachineImages, apisazure.MachineImageVersion] {
	return util.NewImagesContext(
		utils.CreateMapFromSlice(providerImages, func(mi apisazure.MachineImages) string { return mi.Name }),
		func(mi apisazure.MachineImages) map[string]apisazure.MachineImageVersion {
			return utils.CreateMapFromSlice(mi.Versions, func(v apisazure.MachineImageVersion) string { return providerMachineImageKey(v) })
		},
	)
}
