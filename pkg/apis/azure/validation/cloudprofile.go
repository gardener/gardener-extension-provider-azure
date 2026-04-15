// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	gardencoreapi "github.com/gardener/gardener/pkg/api"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
)

// ValidateCloudProfileConfig validates a CloudProfileConfig object.
func ValidateCloudProfileConfig(cpConfig *apisazure.CloudProfileConfig, machineImages []core.MachineImage, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateDomainCount(cpConfig.CountFaultDomains, fldPath.Child("countFaultDomains"))...)
	allErrs = append(allErrs, validateDomainCount(cpConfig.CountUpdateDomains, fldPath.Child("countUpdateDomains"))...)

	machineImagesPath := fldPath.Child("machineImages")
	// Validate machine images section
	allErrs = append(allErrs, validateMachineImages(cpConfig.MachineImages, capabilityDefinitions, machineImagesPath)...)
	if len(allErrs) > 0 {
		return allErrs
	}

	allErrs = append(allErrs, validateProviderImagesMapping(cpConfig.MachineImages, machineImages, capabilityDefinitions, field.NewPath("spec").Child("machineImages"))...)

	return allErrs
}

// validateMachineImages validates the machine images section of CloudProfileConfig
func validateMachineImages(machineImages []apisazure.MachineImages, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure at least one machine image is provided
	if len(machineImages) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one machine image"))
		return allErrs
	}

	// Validate each machine image
	for i, machineImage := range machineImages {
		idxPath := fldPath.Index(i)
		allErrs = append(allErrs, ValidateProviderMachineImage(machineImage, capabilityDefinitions, idxPath)...)
	}

	return allErrs
}

// ValidateProviderMachineImage validates a CloudProfileConfig MachineImages entry.
func ValidateProviderMachineImage(providerImage apisazure.MachineImages, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, validationPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(providerImage.Name) == 0 {
		allErrs = append(allErrs, field.Required(validationPath.Child("name"), "must provide a name"))
	}

	if len(providerImage.Versions) == 0 {
		allErrs = append(allErrs, field.Required(validationPath.Child("versions"), fmt.Sprintf("must provide at least one version for machine image %q", providerImage.Name)))
	}

	// Validate each version
	for j, version := range providerImage.Versions {
		jdxPath := validationPath.Child("versions").Index(j)
		allErrs = append(allErrs, validateMachineImageVersion(providerImage, version, capabilityDefinitions, jdxPath)...)
	}

	return allErrs
}

// validateMachineImageVersion validates a specific machine image version
func validateMachineImageVersion(providerImage apisazure.MachineImages, version apisazure.MachineImageVersion, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, jdxPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(version.Version) == 0 {
		allErrs = append(allErrs, field.Required(jdxPath.Child("version"), "must provide a version"))
	}

	hasCapabilityFlavors := len(version.CapabilityFlavors) > 0
	hasLegacyImage := version.Image != (apisazure.Image{})
	hasLegacyFields := version.Architecture != nil || version.AcceleratedNetworking != nil

	if len(capabilityDefinitions) > 0 {
		// When CloudProfile defines capabilities, allow either old format (image) or new format (capabilityFlavors) per version
		if hasCapabilityFlavors && (hasLegacyImage || hasLegacyFields) {
			if hasLegacyImage {
				allErrs = append(allErrs, field.Forbidden(jdxPath.Child("image"), "must not be set together with capabilityFlavors. Use one format per version."))
			}
			if version.Architecture != nil {
				allErrs = append(allErrs, field.Forbidden(jdxPath.Child("architecture"), "must not be set together with capabilityFlavors. Use one format per version."))
			}
			if version.AcceleratedNetworking != nil {
				allErrs = append(allErrs, field.Forbidden(jdxPath.Child("acceleratedNetworking"), "must not be set together with capabilityFlavors. Use one format per version."))
			}
		} else if hasCapabilityFlavors {
			// New format: validate capabilityFlavors
			allErrs = append(allErrs, validateCapabilityFlavors(version, capabilityDefinitions, jdxPath)...)
		} else if hasLegacyImage {
			// Old format: validate image with architecture (mixed format support)
			allErrs = append(allErrs, validateLegacyImageWithCapabilities(version, jdxPath)...)
		} else {
			// Neither format specified
			allErrs = append(allErrs, field.Required(jdxPath.Child("image"),
				fmt.Sprintf("must provide either image or capabilityFlavors for machine image %s@%s", providerImage.Name, version.Version)))
		}
	} else {
		// Without capabilities, only old format with image is supported
		if hasCapabilityFlavors {
			allErrs = append(allErrs, field.Forbidden(jdxPath.Child("capabilityFlavors"), "capabilityFlavors must not be set when cloudprofile is not using capabilities"))
		}
		allErrs = append(allErrs, validateLegacyProvidedImage(version, jdxPath)...)
	}
	return allErrs
}

// validateLegacyProvidedImage validates a machine image version when no capability definitions are provided in the cloud profile
func validateLegacyProvidedImage(version apisazure.MachineImageVersion, jdxPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	versionArch := ptr.Deref(version.Architecture, v1beta1constants.ArchitectureAMD64)

	allErrs = append(allErrs, validateProvidedImageIdCount(version.Image, jdxPath)...)
	allErrs = append(allErrs, validateProviderImageId(version.Image, jdxPath)...)

	// Validate architecture field
	if !slices.Contains(v1beta1constants.ValidArchitectures, versionArch) {
		allErrs = append(allErrs, field.NotSupported(jdxPath.Child("architecture"), versionArch, v1beta1constants.ValidArchitectures))
	}
	return allErrs
}

// validateLegacyImageWithCapabilities validates old format (image with architecture) when CloudProfile uses capabilities.
// This allows architecture field since it will be used for capability mapping.
func validateLegacyImageWithCapabilities(version apisazure.MachineImageVersion, jdxPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	versionArch := ptr.Deref(version.Architecture, v1beta1constants.ArchitectureAMD64)

	// Validate architecture is valid since it will be used for capability mapping
	if !slices.Contains(v1beta1constants.ValidArchitectures, versionArch) {
		allErrs = append(allErrs, field.NotSupported(jdxPath.Child("architecture"), versionArch, v1beta1constants.ValidArchitectures))
	}

	// Validate image fields
	allErrs = append(allErrs, validateProvidedImageIdCount(version.Image, jdxPath)...)
	allErrs = append(allErrs, validateProviderImageId(version.Image, jdxPath)...)

	return allErrs
}

// validateProviderImageId validates the image id fields (urn/id/communityGalleryImageID/sharedGalleryImageID) of a machine image version
func validateProviderImageId(version apisazure.Image, jdxPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

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

	return allErrs
}

// validateCapabilityFlavors validates the capability flavors defined in a machine image version.
func validateCapabilityFlavors(version apisazure.MachineImageVersion, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, idxPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Architecture must not be set when using capabilityFlavors
	if ptr.Deref(version.Architecture, "") != "" {
		allErrs = append(allErrs, field.Forbidden(idxPath.Child("architecture"), "architecture must not be set when cloudprofile capabilities are defined. Use capabilityFlavors instead"))
	}

	// Validate each flavor's capabilities and regions
	for k, capabilityFlavor := range version.CapabilityFlavors {
		kdxPath := idxPath.Child("capabilityFlavors").Index(k)
		allErrs = append(allErrs, gutil.ValidateCapabilities(capabilityFlavor.Capabilities, capabilityDefinitions, kdxPath.Child("capabilities"))...)
		allErrs = append(allErrs, validateProvidedImageIdCount(capabilityFlavor.Image, kdxPath)...)
		allErrs = append(allErrs, validateProviderImageId(capabilityFlavor.Image, kdxPath)...)
	}

	return allErrs
}

// verify that for each cp image a provider image exists
func validateProviderImagesMapping(cpConfigImages []apisazure.MachineImages, machineImages []core.MachineImage, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	providerImages := NewProviderImagesContext(cpConfigImages)

	// for each image in the CloudProfile, check if it exists in the CloudProfileConfig
	for idxImage, machineImage := range machineImages {
		machineImagePath := fldPath.Index(idxImage)
		providerImage, imageExists := providerImages.GetImage(machineImage.Name)
		if !imageExists {
			allErrs = append(allErrs, field.Required(machineImagePath, fmt.Sprintf("must provide a provider image mapping for image %q", machineImage.Name)))
			continue
		}

		for versionIdx, version := range machineImage.Versions {
			imageVersionPath := machineImagePath.Child("versions").Index(versionIdx)
			// check that each MachineImageFlavor in version.CapabilityFlavors has a corresponding imageVersion.CapabilityFlavors

			if len(capabilityDefinitions) > 0 {
				// Group provider versions by version string to handle mixed format
				// (old format may have multiple entries per version with different architectures)
				groupedVersions := helper.GroupVersionsByVersionString(providerImage.Versions)
				providerVersions, versionExists := groupedVersions[version.Version]
				if !versionExists || len(providerVersions) == 0 {
					allErrs = append(allErrs, field.Required(imageVersionPath,
						fmt.Sprintf("machine image version %s@%s is not defined in the providerConfig", machineImage.Name, version.Version),
					))
					continue
				}
				allErrs = append(allErrs, validateImageFlavorMappingMixed(version, providerVersions, capabilityDefinitions, machineImage, imageVersionPath)...)
			} else {
				allErrs = append(allErrs, validateArchitectureMapping(version, cpConfigImages, machineImage, imageVersionPath)...)
			}
		}
	}
	return allErrs
}

// validateImageFlavorMappingMixed validates that each flavor in a version has a corresponding mapping.
// This function handles both the new format (capabilityFlavors) and old format (image with architecture).
// For mixed format support, multiple provider version entries may exist for the same version string.
func validateImageFlavorMappingMixed(version core.MachineImageVersion, providerVersions []apisazure.MachineImageVersion, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, machineImage core.MachineImage, machineImageVersionPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	var v1beta1Version gardencorev1beta1.MachineImageVersion
	if err := gardencoreapi.Scheme.Convert(&version, &v1beta1Version, nil); err != nil {
		return append(allErrs, field.InternalError(machineImageVersionPath, err))
	}

	defaultedCapabilityFlavors := gardencorev1beta1helper.GetImageFlavorsWithAppliedDefaults(v1beta1Version.CapabilityFlavors, capabilityDefinitions)

	capabilityFlavorsVersion := FindCapabilityFlavorsVersion(providerVersions)
	if capabilityFlavorsVersion != nil {
		// New format: validate against capabilityFlavors
		allErrs = append(allErrs, ValidateMissingCapabilityFlavors(
			machineImage.Name, version.Version,
			defaultedCapabilityFlavors,
			capabilityFlavorsVersion.CapabilityFlavors,
			capabilityDefinitions,
			machineImageVersionPath,
			"providerConfig",
			true, // include index path for CloudProfile validation
		)...)
	} else {
		// Old format: collect architectures from all provider version entries
		availableArchitectures := CollectAvailableArchitectures(providerVersions)
		allErrs = append(allErrs, ValidateMissingArchitectures(
			machineImage.Name, version.Version,
			defaultedCapabilityFlavors,
			availableArchitectures,
			machineImageVersionPath,
			"providerConfig",
			true, // include index path for CloudProfile validation
		)...)
	}
	return allErrs
}

func validateArchitectureMapping(
	version core.MachineImageVersion,
	cpConfigImages []apisazure.MachineImages,
	machineImage core.MachineImage,
	machineImageVersionPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	cpConfigImagesContext := NewProviderImagesContextLegacy(cpConfigImages)

	for _, expectedArchitecture := range version.Architectures {
		// validate machine image version architectures
		if !slices.Contains(v1beta1constants.ValidArchitectures, expectedArchitecture) {
			allErrs = append(allErrs, field.NotSupported(
				machineImageVersionPath.Child("architectures"),
				expectedArchitecture, v1beta1constants.ValidArchitectures))
		}
		// validate machine image version with architecture x exists in cpConfig
		_, exists := cpConfigImagesContext.GetImageVersion(machineImage.Name, VersionArchitectureKey(version.Version, expectedArchitecture))
		if !exists {
			allErrs = append(allErrs, field.Required(machineImageVersionPath,
				fmt.Sprintf("machine image version %s@%s and architecture: %s is not defined in the providerConfig",
					machineImage.Name, version.Version, expectedArchitecture),
			))
			continue
		}
	}
	return allErrs
}

// validateProvidedImageIdCount validates that only one of urn/id/communityGalleryImageID/sharedGalleryImageID is provided
func validateProvidedImageIdCount(version apisazure.Image, fldPath *field.Path) field.ErrorList {
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

// NewProviderImagesContextLegacy creates a new images context for provider images.
func NewProviderImagesContextLegacy(providerImages []apisazure.MachineImages) *gutil.ImagesContext[apisazure.MachineImages, apisazure.MachineImageVersion] {
	return gutil.NewImagesContext(
		utils.CreateMapFromSlice(providerImages, func(mi apisazure.MachineImages) string { return mi.Name }),
		func(mi apisazure.MachineImages) map[string]apisazure.MachineImageVersion {
			return utils.CreateMapFromSlice(mi.Versions, func(v apisazure.MachineImageVersion) string { return providerMachineImageKey(v) })
		},
	)
}

// NewProviderImagesContext creates a new ImagesContext for provider images.
func NewProviderImagesContext(providerImages []apisazure.MachineImages) *gutil.ImagesContext[apisazure.MachineImages, apisazure.MachineImageVersion] {
	return gutil.NewImagesContext(
		utils.CreateMapFromSlice(providerImages, func(mi apisazure.MachineImages) string { return mi.Name }),
		func(mi apisazure.MachineImages) map[string]apisazure.MachineImageVersion {
			return utils.CreateMapFromSlice(mi.Versions, func(v apisazure.MachineImageVersion) string { return v.Version })
		},
	)
}

// GroupVersionsByVersionString groups all provider versions by their version string.
// This is needed because the old format may have multiple entries for the same version
// with different architectures (mixed format support).
func GroupVersionsByVersionString(versions []apisazure.MachineImageVersion) map[string][]apisazure.MachineImageVersion {
	result := make(map[string][]apisazure.MachineImageVersion)
	for _, v := range versions {
		result[v.Version] = append(result[v.Version], v)
	}
	return result
}

// FindCapabilityFlavorsVersion finds the first provider version that uses the new format (capabilityFlavors).
// Returns nil if no version uses the new format.
func FindCapabilityFlavorsVersion(providerVersions []apisazure.MachineImageVersion) *apisazure.MachineImageVersion {
	for i := range providerVersions {
		if len(providerVersions[i].CapabilityFlavors) > 0 {
			return &providerVersions[i]
		}
	}
	return nil
}

// CollectAvailableArchitectures collects unique architectures from all provider version entries (old format).
func CollectAvailableArchitectures(providerVersions []apisazure.MachineImageVersion) []string {
	architecturesMap := utils.CreateMapFromSlice(providerVersions, func(v apisazure.MachineImageVersion) string {
		return ptr.Deref(v.Architecture, v1beta1constants.ArchitectureAMD64)
	})
	return slices.Collect(maps.Keys(architecturesMap))
}

// ValidateMissingCapabilityFlavors checks that all expected capability flavors from the spec are defined in the provider config.
// If includeIndexPath is true, adds .capabilityFlavors[idx] to the field path.
func ValidateMissingCapabilityFlavors(
	imageName, versionStr string,
	defaultedSpecCapabilities []gardencorev1beta1.MachineImageFlavor,
	providerCapabilityFlavors []apisazure.MachineImageFlavor,
	capabilityDefinitions []gardencorev1beta1.CapabilityDefinition,
	path *field.Path,
	errorMsgSuffix string,
	includeIndexPath bool,
) field.ErrorList {
	allErrs := field.ErrorList{}

	for idxCapability, specCapabilitySet := range defaultedSpecCapabilities {
		isFound := false
		for _, providerCapabilityFlavor := range providerCapabilityFlavors {
			providerDefaultedCapabilities := gardencorev1beta1.GetCapabilitiesWithAppliedDefaults(providerCapabilityFlavor.Capabilities, capabilityDefinitions)
			if gardencorev1beta1helper.AreCapabilitiesEqual(specCapabilitySet.Capabilities, providerDefaultedCapabilities) {
				isFound = true
				break
			}
		}
		if !isFound {
			errPath := path
			if includeIndexPath {
				errPath = path.Child("capabilityFlavors").Index(idxCapability)
			}
			allErrs = append(allErrs, field.Required(errPath,
				fmt.Sprintf("machine image version %s@%s and capabilityFlavor %v is not defined in the %s", imageName, versionStr, specCapabilitySet.Capabilities, errorMsgSuffix)))
		}
	}

	return allErrs
}

// ValidateExcessCapabilityFlavors checks that the provider config doesn't have extra capability flavors not defined in the spec.
func ValidateExcessCapabilityFlavors(
	imageName, versionStr string,
	defaultedSpecCapabilities []gardencorev1beta1.MachineImageFlavor,
	providerCapabilityFlavors []apisazure.MachineImageFlavor,
	capabilityDefinitions []gardencorev1beta1.CapabilityDefinition,
	path *field.Path,
	errorMsgSuffix string,
) field.ErrorList {
	allErrs := field.ErrorList{}

	for _, providerCapabilityFlavor := range providerCapabilityFlavors {
		isFound := false
		for _, specCapabilitySet := range defaultedSpecCapabilities {
			providerDefaultedCapabilities := gardencorev1beta1.GetCapabilitiesWithAppliedDefaults(providerCapabilityFlavor.Capabilities, capabilityDefinitions)
			if gardencorev1beta1helper.AreCapabilitiesEqual(specCapabilitySet.Capabilities, providerDefaultedCapabilities) {
				isFound = true
				break
			}
		}
		if !isFound {
			allErrs = append(allErrs, field.Forbidden(path,
				fmt.Sprintf("machine image version %s@%s has an excess capabilityFlavor %v, which is not defined in the %s", imageName, versionStr, providerCapabilityFlavor.Capabilities, errorMsgSuffix)))
		}
	}

	return allErrs
}

// ValidateMissingArchitectures checks that all expected architectures from capability flavors are available in the provider.
// If includeIndexPath is true, adds .capabilityFlavors[idx] to the field path.
func ValidateMissingArchitectures(
	imageName, versionStr string,
	defaultedSpecCapabilities []gardencorev1beta1.MachineImageFlavor,
	availableArchitectures []string,
	path *field.Path,
	errorMsgSuffix string,
	includeIndexPath bool,
) field.ErrorList {
	allErrs := field.ErrorList{}

	for idxCapability, specCapabilitySet := range defaultedSpecCapabilities {
		expectedArchitectures := specCapabilitySet.Capabilities[v1beta1constants.ArchitectureName]
		for _, expectedArch := range expectedArchitectures {
			if !slices.Contains(availableArchitectures, expectedArch) {
				errPath := path
				if includeIndexPath {
					errPath = path.Child("capabilityFlavors").Index(idxCapability)
				}
				allErrs = append(allErrs, field.Required(errPath,
					fmt.Sprintf("machine image version %s@%s and capabilityFlavor %v is not defined in the %s", imageName, versionStr, specCapabilitySet.Capabilities, errorMsgSuffix)))
			}
		}
	}

	return allErrs
}

// ValidateExcessArchitectures checks that the provider doesn't have extra architectures not defined in the spec capabilities.
func ValidateExcessArchitectures(
	imageName, versionStr string,
	defaultedSpecCapabilities []gardencorev1beta1.MachineImageFlavor,
	availableArchitectures []string,
	path *field.Path,
	errorMsgSuffix string,
) field.ErrorList {
	allErrs := field.ErrorList{}

	for _, arch := range availableArchitectures {
		isFound := false
		for _, specCapabilitySet := range defaultedSpecCapabilities {
			expectedArchitectures := specCapabilitySet.Capabilities[v1beta1constants.ArchitectureName]
			if slices.Contains(expectedArchitectures, arch) {
				isFound = true
				break
			}
		}
		if !isFound {
			allErrs = append(allErrs, field.Forbidden(path,
				fmt.Sprintf("machine image version %s@%s has an excess architecture %q, which is not defined in the %s", imageName, versionStr, arch, errorMsgSuffix)))
		}
	}

	return allErrs
}
