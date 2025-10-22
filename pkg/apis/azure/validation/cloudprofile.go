// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"strings"

	gardencoreapi "github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"k8s.io/utils/strings/slices"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
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
		allErrs = append(allErrs, validateMachineImageVersion(version, capabilityDefinitions, jdxPath)...)
	}

	return allErrs
}

// validateMachineImageVersion validates a specific machine image version
func validateMachineImageVersion(version apisazure.MachineImageVersion, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, jdxPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(version.Version) == 0 {
		allErrs = append(allErrs, field.Required(jdxPath.Child("version"), "must provide a version"))
	}

	if len(capabilityDefinitions) > 0 {
		allErrs = append(allErrs, validateCapabilityFlavors(version, capabilityDefinitions, jdxPath)...)
	} else {
		allErrs = append(allErrs, validateLegacyProvidedImage(version, jdxPath)...)
	}
	return allErrs
}

// validateLegacyProvidedImage validates a machine image version when no capability definitions are provided in the cloud profile
func validateLegacyProvidedImage(version apisazure.MachineImageVersion, jdxPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateProvidedImageIdCount(version.Image, jdxPath)...)
	allErrs = append(allErrs, validateProviderImageId(version.Image, jdxPath)...)
	if !slices.Contains(v1beta1constants.ValidArchitectures, *version.Architecture) {
		allErrs = append(allErrs, field.NotSupported(jdxPath.Child("architecture"), *version.Architecture, v1beta1constants.ValidArchitectures))
	}

	if len(version.CapabilityFlavors) != 0 {
		allErrs = append(allErrs, field.Forbidden(jdxPath.Child("capabilityFlavors"), "capabilityFlavors must not be set when cloudprofile are not using capabilities"))
	}
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
	// ensure the version.Image is not set when capabilityFlavors are defined
	if version.Image != (apisazure.Image{}) {
		allErrs = append(allErrs, field.Forbidden(idxPath, "image references must not be set when cloudprofile capabilities are defined. Use reference fields in capabilityFlavors instead"))
	}
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
	providerImagesLegacy := NewProviderImagesContextLegacy(cpConfigImages)
	providerImages := NewProviderImagesContext(cpConfigImages)

	// for each image in the CloudProfile, check if it exists in the CloudProfileConfig
	for idxImage, machineImage := range machineImages {
		machineImagePath := fldPath.Index(idxImage)
		if _, existsInConfig := providerImages.GetImage(machineImage.Name); !existsInConfig {
			allErrs = append(allErrs, field.Required(machineImagePath, fmt.Sprintf("must provide a provider image mapping for image %q", machineImage.Name)))
			continue
		}

		for versionIdx, version := range machineImage.Versions {
			imageVersionPath := machineImagePath.Child("versions").Index(versionIdx)
			// check that each MachineImageFlavor in version.CapabilityFlavors has a corresponding imageVersion.CapabilityFlavors

			if len(capabilityDefinitions) > 0 {
				// validate that for each machine image version entry a mapped entry in cpConfig exists
				imageVersion, exists := providerImages.GetImageVersion(machineImage.Name, version.Version)
				if !exists {
					allErrs = append(allErrs, field.Required(imageVersionPath,
						fmt.Sprintf("machine image version %s@%s is not defined in the providerConfig",
							machineImage.Name, version.Version),
					))
					continue
				}
				// validate that for each version and capabilityFlavor of an image in the cloud profile a
				// corresponding provider specific image in the cloud profile config exists
				allErrs = append(allErrs, validateImageFlavorMapping(machineImage, version, imageVersionPath, capabilityDefinitions, imageVersion)...)
			} else {
				// validate that for each version and architecture of an image in the cloud profile a
				// corresponding provider specific image in the cloud profile config exists
				for _, expectedArchitecture := range version.Architectures {
					if _, exists := providerImagesLegacy.GetImageVersion(machineImage.Name, VersionArchitectureKey(version.Version, expectedArchitecture)); !exists {
						allErrs = append(allErrs, field.Required(imageVersionPath,
							fmt.Sprintf("must provide an image mapping for version %q and architecture: %s", version.Version, expectedArchitecture)))
					}
				}
			}
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

// validateImageFlavorMapping validates that each flavor in a version has a corresponding mapping
func validateImageFlavorMapping(machineImage core.MachineImage, version core.MachineImageVersion, imageVersionPath *field.Path, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, imageVersion apisazure.MachineImageVersion) field.ErrorList {
	allErrs := field.ErrorList{}

	var v1beta1Version gardencorev1beta1.MachineImageVersion
	if err := gardencoreapi.Scheme.Convert(&version, &v1beta1Version, nil); err != nil {
		return append(allErrs, field.InternalError(imageVersionPath, err))
	}

	defaultedCapabilityFlavors := gardencorev1beta1helper.GetImageFlavorsWithAppliedDefaults(v1beta1Version.CapabilityFlavors, capabilityDefinitions)

	for idxCapability, defaultedCapabilityFlavor := range defaultedCapabilityFlavors {
		isFound := false
		// search for the corresponding imageVersion.MachineImageFlavor
		for _, providerCapabilityFlavor := range imageVersion.CapabilityFlavors {
			providerDefaultedCapabilities := gardencorev1beta1.GetCapabilitiesWithAppliedDefaults(providerCapabilityFlavor.Capabilities, capabilityDefinitions)
			if gardencorev1beta1helper.AreCapabilitiesEqual(defaultedCapabilityFlavor.Capabilities, providerDefaultedCapabilities) {
				isFound = true
				break
			}
		}
		if !isFound {
			allErrs = append(allErrs, field.Required(imageVersionPath.Child("capabilityFlavors").Index(idxCapability),
				fmt.Sprintf("missing providerConfig mapping for machine image version %s@%s and capabilityFlavor %v", machineImage.Name, version.Version, defaultedCapabilityFlavor.Capabilities)))
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

// RestrictToArchitectureAndNetworkingCapability ensures that for the transition period from the deprecated architecture fields to the capabilities format only the `architecture` capability is used to support automatic transformation and migration.
// TODO(Roncossek): Delete this function once the dedicated architecture fields on MachineType and MachineImageVersion have been removed.
func RestrictToArchitectureAndNetworkingCapability(capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, child *field.Path) error {
	allErrs := field.ErrorList{}
	for i, def := range capabilityDefinitions {
		idxPath := child.Index(i)
		if def.Name != v1beta1constants.ArchitectureName && def.Name != azure.CapabilityNetworkName {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("name"), def.Name, []string{v1beta1constants.ArchitectureName, azure.CapabilityNetworkName}))
		}
		if def.Name == azure.CapabilityNetworkName {
			for j, val := range def.Values {
				jdxPath := idxPath.Child("values").Index(j)
				if val != azure.CapabilityNetworkAccelerated && val != azure.CapabilityNetworkBasic {
					allErrs = append(allErrs, field.NotSupported(jdxPath, val, []string{azure.CapabilityNetworkAccelerated, azure.CapabilityNetworkBasic}))
				}
			}
		}
	}
	return allErrs.ToAggregate()
}
