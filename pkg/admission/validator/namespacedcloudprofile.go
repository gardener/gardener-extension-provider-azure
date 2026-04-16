// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"
	"slices"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencoreapi "github.com/gardener/gardener/pkg/api"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"
)

// NewNamespacedCloudProfileValidator returns a new instance of a namespaced cloud profile validator.
func NewNamespacedCloudProfileValidator(mgr manager.Manager) extensionswebhook.Validator {
	return &namespacedCloudProfile{
		client:  mgr.GetClient(),
		decoder: serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
	}
}

type namespacedCloudProfile struct {
	client  client.Client
	decoder runtime.Decoder
}

// Validate validates the given NamespacedCloudProfile objects.
func (p *namespacedCloudProfile) Validate(ctx context.Context, newObj, _ client.Object) error {
	cloudProfile, ok := newObj.(*core.NamespacedCloudProfile)
	if !ok {
		return fmt.Errorf("wrong object type %T", newObj)
	}

	if cloudProfile.DeletionTimestamp != nil {
		return nil
	}

	cpConfig := &api.CloudProfileConfig{}
	if cloudProfile.Spec.ProviderConfig != nil {
		var err error
		cpConfig, err = decodeCloudProfileConfig(p.decoder, cloudProfile.Spec.ProviderConfig)
		if err != nil {
			return fmt.Errorf("could not decode providerConfig of NamespacedCloudProfile for '%s': %w", cloudProfile.Name, err)
		}
	}

	parentCloudProfile := cloudProfile.Spec.Parent
	if parentCloudProfile.Kind != constants.CloudProfileReferenceKindCloudProfile {
		return fmt.Errorf("parent reference must be of kind CloudProfile (unsupported kind: %s)", parentCloudProfile.Kind)
	}
	parentProfile := &gardencorev1beta1.CloudProfile{}
	if err := p.client.Get(ctx, client.ObjectKey{Name: parentCloudProfile.Name}, parentProfile); err != nil {
		return err
	}

	return p.validateNamespacedCloudProfileProviderConfig(cpConfig, cloudProfile.Spec, parentProfile.Spec).ToAggregate()
}

// validateNamespacedCloudProfileProviderConfig validates the CloudProfileConfig passed with a NamespacedCloudProfile.
func (p *namespacedCloudProfile) validateNamespacedCloudProfileProviderConfig(providerConfig *api.CloudProfileConfig, profileSpec core.NamespacedCloudProfileSpec, parentSpec gardencorev1beta1.CloudProfileSpec) field.ErrorList {
	allErrs := field.ErrorList{}

	if providerConfig.CloudConfiguration != nil {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec.providerConfig.cloudConfiguration"),
			"cloud configuration is not allowed in a NamespacedCloudProfile providerConfig",
		))
	}
	if len(providerConfig.CountUpdateDomains) > 0 {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec.providerConfig.countUpdateDomains"),
			"count update domains is not allowed in a NamespacedCloudProfile providerConfig",
		))
	}
	if len(providerConfig.CountFaultDomains) > 0 {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec.providerConfig.countFaultDomains"),
			"count fault domains is not allowed in a NamespacedCloudProfile providerConfig",
		))
	}

	allErrs = append(allErrs, p.validateMachineImages(providerConfig, profileSpec.MachineImages, parentSpec)...)
	allErrs = append(allErrs, p.validateMachineTypes(providerConfig, profileSpec.MachineTypes, parentSpec)...)

	return allErrs
}

func (p *namespacedCloudProfile) validateMachineImages(providerConfig *api.CloudProfileConfig, machineImages []core.MachineImage, parentSpec gardencorev1beta1.CloudProfileSpec) field.ErrorList {
	allErrs := field.ErrorList{}
	capabilityDefinitions := parentSpec.MachineCapabilities

	imagesPath := field.NewPath("spec.providerConfig.machineImages")
	for i, machineImage := range providerConfig.MachineImages {
		idxPath := imagesPath.Index(i)
		allErrs = append(allErrs, validation.ValidateProviderMachineImage(machineImage, parentSpec.MachineCapabilities, idxPath)...)
	}

	namespacedImages := gutil.NewCoreImagesContext(machineImages)
	parentImages := gutil.NewV1beta1ImagesContext(parentSpec.MachineImages)
	providerImagesLegacy := validation.NewProviderImagesContextLegacy(providerConfig.MachineImages)
	providerImages := validation.NewProviderImagesContext(providerConfig.MachineImages)

	// Create a map of provider images grouped by version for mixed format support
	providerVersionsMap := make(map[string]map[string][]api.MachineImageVersion)
	for _, img := range providerConfig.MachineImages {
		providerVersionsMap[img.Name] = helper.GroupVersionsByVersionString(img.Versions)
	}

	for _, machineImage := range namespacedImages.Images {
		// Check that for each new image version defined in the NamespacedCloudProfile, the image is also defined in the providerConfig.
		_, existsInParent := parentImages.GetImage(machineImage.Name)
		_, existsInProvider := providerImages.GetImage(machineImage.Name)
		if !existsInParent && !existsInProvider {
			allErrs = append(allErrs, field.Required(
				imagesPath,
				fmt.Sprintf("machine image %s is not defined in the NamespacedCloudProfile providerConfig", machineImage.Name),
			))
			continue
		}
		for _, version := range machineImage.Versions {
			if len(capabilityDefinitions) == 0 {
				// check that each architecture defined has a corresponding entry in the providerConfig
				for _, expectedArchitecture := range version.Architectures {
					if _, exists := providerImagesLegacy.GetImageVersion(machineImage.Name, validation.VersionArchitectureKey(version.Version, expectedArchitecture)); !existsInParent && !exists {
						allErrs = append(allErrs, field.Required(imagesPath,
							fmt.Sprintf("machine image version %s@%s and architecture: %q is not defined in the NamespacedCloudProfile providerConfig",
								machineImage.Name, version.Version, expectedArchitecture),
						))
					}
				}
			} else {
				// check that each capabilityFlavor defined has a corresponding entry in the providerConfig
				// Support mixed format: group provider versions by version string
				providerVersions, versionExists := providerVersionsMap[machineImage.Name][version.Version]
				if !versionExists || len(providerVersions) == 0 {
					allErrs = append(allErrs, field.Required(imagesPath,
						fmt.Sprintf("machine image version %s@%s is not defined in the NamespacedCloudProfile providerConfig", machineImage.Name, version.Version),
					))
					continue
				}
				allErrs = append(allErrs, validateMissingCapabilitiesMixed(machineImage, version, providerVersions, capabilityDefinitions, imagesPath)...)
			}
		}
	}

	for imageIdx, machineImage := range providerConfig.MachineImages {
		// Check that the machine image version is not already defined in the parent CloudProfile.
		if _, exists := parentImages.GetImage(machineImage.Name); exists {
			for versionIdx, version := range machineImage.Versions {
				if _, exists := parentImages.GetImageVersion(machineImage.Name, version.Version); exists {
					allErrs = append(allErrs, field.Forbidden(
						imagesPath.Index(imageIdx).Child("versions").Index(versionIdx),
						fmt.Sprintf("machine image version %s@%s is already defined in the parent CloudProfile", machineImage.Name, version.Version),
					))
				}
			}
		}
		// Check that the machine image version is defined in the NamespacedCloudProfile.
		if _, exists := namespacedImages.GetImage(machineImage.Name); !exists {
			allErrs = append(allErrs, field.Required(
				imagesPath.Index(imageIdx),
				fmt.Sprintf("machine image %s is not defined in the NamespacedCloudProfile .spec.machineImages", machineImage.Name),
			))
			continue
		}
		for versionIdx, version := range machineImage.Versions {
			profileImageVersion, exists := namespacedImages.GetImageVersion(machineImage.Name, version.Version)
			if !exists {
				allErrs = append(allErrs, field.Invalid(
					imagesPath.Index(imageIdx).Child("versions").Index(versionIdx),
					fmt.Sprintf("%s@%s", machineImage.Name, version.Version),
					"machine image version is not defined in the NamespacedCloudProfile",
				))
				// no need to check the architecture and capabilityFlavors below if the version does not exist in the profile
				continue
			}

			if len(capabilityDefinitions) == 0 {
				// For non-capabilities CloudProfile, check if architecture is valid
				providerConfigArchitecture := ptr.Deref(version.Architecture, constants.ArchitectureAMD64)
				// If version doesn't exist, all architectures are excess
				if !slices.Contains(profileImageVersion.Architectures, providerConfigArchitecture) {
					allErrs = append(allErrs, field.Forbidden(
						imagesPath,
						fmt.Sprintf("machine image version %s@%s has an excess entry for architecture %q, which is not defined in the machineImages spec",
							machineImage.Name, version.Version, providerConfigArchitecture),
					))
				}
			} else {
				// For capabilities CloudProfile, validate excess capability flavors or architectures
				providerVersions := providerVersionsMap[machineImage.Name][version.Version]
				capabilityFlavorsVersion := validation.FindCapabilityFlavorsVersion(providerVersions)

				var v1betaVersion gardencorev1beta1.MachineImageVersion
				if err := gardencoreapi.Scheme.Convert(&profileImageVersion, &v1betaVersion, nil); err != nil {
					return append(allErrs, field.InternalError(imagesPath, err))
				}
				defaultedCapabilityFlavors := gardencorev1beta1helper.GetImageFlavorsWithAppliedDefaults(v1betaVersion.CapabilityFlavors, capabilityDefinitions)

				if capabilityFlavorsVersion != nil {
					// New format: check for excess capability flavors
					providerImageVersion, _ := providerImages.GetImageVersion(machineImage.Name, version.Version)
					for _, capabilityFlavor := range providerImageVersion.CapabilityFlavors {
						defaultedProviderCapabilities := gardencorev1beta1.GetCapabilitiesWithAppliedDefaults(capabilityFlavor.Capabilities, capabilityDefinitions)
						isFound := false

						for _, coreDefaultedCapabilityFlavor := range defaultedCapabilityFlavors {
							if gardencorev1beta1helper.AreCapabilitiesEqual(coreDefaultedCapabilityFlavor.Capabilities, defaultedProviderCapabilities) {
								isFound = true
								break
							}
						}
						if !isFound {
							allErrs = append(allErrs, field.Forbidden(imagesPath,
								fmt.Sprintf("machine image version %s@%s has an excess capabilityFlavor %v, which is not defined in the machineImages spec",
									machineImage.Name, version.Version, capabilityFlavor.Capabilities)))
						}
					}
				} else if version.Image != (api.Image{}) && len(version.CapabilityFlavors) == 0 {
					// Old format in capabilities CloudProfile: validate excess architectures
					providerArch := ptr.Deref(version.Architecture, constants.ArchitectureAMD64)
					allErrs = append(allErrs, validateExcessArchitecture(machineImage.Name, version.Version, providerArch, profileImageVersion.CapabilityFlavors, capabilityDefinitions, imagesPath)...)
				}
			}
		}
	}

	return allErrs
}

// validateExcessArchitecture checks if a provider's architecture is defined in the spec's capability flavors
func validateExcessArchitecture(imageName, versionStr, providerArch string, specCapabilityFlavors []core.MachineImageFlavor, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	var v1beta1Flavors []gardencorev1beta1.MachineImageFlavor
	for _, f := range specCapabilityFlavors {
		// Manually convert core.Capabilities to v1beta1.Capabilities
		v1betaCapabilities := convertCoreCapabilitiesToV1beta1(f.Capabilities)
		v1beta1Flavors = append(v1beta1Flavors, gardencorev1beta1.MachineImageFlavor{Capabilities: v1betaCapabilities})
	}
	defaultedCapabilityFlavors := gardencorev1beta1helper.GetImageFlavorsWithAppliedDefaults(v1beta1Flavors, capabilityDefinitions)

	isFound := false
	for _, flavor := range defaultedCapabilityFlavors {
		expectedArchitectures := flavor.Capabilities[constants.ArchitectureName]
		if slices.Contains(expectedArchitectures, providerArch) {
			isFound = true
			break
		}
	}
	if !isFound {
		allErrs = append(allErrs, field.Forbidden(path,
			fmt.Sprintf("machine image version %s@%s has an excess architecture %q, which is not defined in the machineImages spec",
				imageName, versionStr, providerArch)))
	}

	return allErrs
}

// convertCoreCapabilitiesToV1beta1 converts core.Capabilities to v1beta1.Capabilities manually
func convertCoreCapabilitiesToV1beta1(coreCapabilities core.Capabilities) gardencorev1beta1.Capabilities {
	v1beta1Capabilities := make(gardencorev1beta1.Capabilities)
	for k, v := range coreCapabilities {
		// Copy the slice values
		v1beta1Capabilities[k] = append([]string{}, v...)
	}
	return v1beta1Capabilities
}

// validateMissingCapabilitiesMixed validates missing machine image capabilities with mixed format support.
// It handles both old format (image with architecture) and new format (capabilityFlavors).
func validateMissingCapabilitiesMixed(machineImage core.MachineImage, version core.MachineImageVersion, providerVersions []api.MachineImageVersion, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	var v1betaVersion gardencorev1beta1.MachineImageVersion
	if err := gardencoreapi.Scheme.Convert(&version, &v1betaVersion, nil); err != nil {
		return append(allErrs, field.InternalError(path, err))
	}
	defaultedVersionCapabilityFlavors := gardencorev1beta1helper.GetImageFlavorsWithAppliedDefaults(v1betaVersion.CapabilityFlavors, capabilityDefinitions)

	capabilityFlavorsVersion := validation.FindCapabilityFlavorsVersion(providerVersions)
	if capabilityFlavorsVersion != nil {
		// New format: validate missing capability flavors
		allErrs = append(allErrs, validation.ValidateMissingCapabilityFlavors(
			machineImage.Name, version.Version,
			defaultedVersionCapabilityFlavors,
			capabilityFlavorsVersion.CapabilityFlavors,
			capabilityDefinitions,
			path,
			"NamespacedCloudProfile providerConfig",
			false, // don't include index path for NamespacedCloudProfile validation
		)...)
	} else {
		// Old format: validate missing architectures
		availableArchitectures := validation.CollectAvailableArchitectures(providerVersions)
		allErrs = append(allErrs, validation.ValidateMissingArchitectures(
			machineImage.Name, version.Version,
			defaultedVersionCapabilityFlavors,
			availableArchitectures,
			path,
			"NamespacedCloudProfile providerConfig",
			false, // don't include index path for NamespacedCloudProfile validation
		)...)
	}

	return allErrs
}

func (p *namespacedCloudProfile) validateMachineTypes(providerConfig *api.CloudProfileConfig, machineTypes []core.MachineType, parentSpec gardencorev1beta1.CloudProfileSpec) field.ErrorList {
	allErrs := field.ErrorList{}

	profileTypes := utils.CreateMapFromSlice(machineTypes, func(mi core.MachineType) string { return mi.Name })
	parentTypes := utils.CreateMapFromSlice(parentSpec.MachineTypes, func(mi gardencorev1beta1.MachineType) string { return mi.Name })

	for typeIdx, machineType := range providerConfig.MachineTypes {
		// Check that the machine type is not already defined in the parent CloudProfile.
		if _, exists := parentTypes[machineType.Name]; exists {
			allErrs = append(allErrs, field.Forbidden(
				field.NewPath("spec.providerConfig.machineTypes").Index(typeIdx),
				fmt.Sprintf("machine type %s is already defined in the parent CloudProfile", machineType.Name),
			))
		}
		// Check that the machine type is defined in the NamespacedCloudProfile.
		_, exists := profileTypes[machineType.Name]
		if !exists {
			allErrs = append(allErrs, field.Required(
				field.NewPath("spec.providerConfig.machineTypes").Index(typeIdx),
				fmt.Sprintf("machine type %s is not defined in the NamespacedCloudProfile .spec.machineTypes", machineType.Name),
			))
			continue
		}
	}

	return allErrs
}
