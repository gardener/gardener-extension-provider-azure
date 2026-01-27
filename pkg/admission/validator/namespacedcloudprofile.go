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
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/gardener"
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

	// TODO(Roncossek): Remove TransformSpecToParentFormat once all CloudProfiles have been migrated to use CapabilityFlavors and the Architecture fields are effectively forbidden or have been removed.
	if err := helper.SimulateTransformToParentFormat(cpConfig, cloudProfile, parentProfile.Spec.MachineCapabilities); err != nil {
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

func (p *namespacedCloudProfile) validateMachineImages(providerConfig *api.CloudProfileConfig, namespacedMachineImages []core.MachineImage, parentSpec gardencorev1beta1.CloudProfileSpec) field.ErrorList {
	allErrs := field.ErrorList{}

	machineImagesPath := field.NewPath("spec.providerConfig.machineImages")
	for i, machineImage := range providerConfig.MachineImages {
		idxPath := machineImagesPath.Index(i)
		allErrs = append(allErrs, validation.ValidateProviderMachineImage(machineImage, parentSpec.MachineCapabilities, idxPath)...)
	}

	namespacedImages := gardener.NewCoreImagesContext(namespacedMachineImages)
	parentImages := gardener.NewV1beta1ImagesContext(parentSpec.MachineImages)
	providerImagesLegacy := validation.NewProviderImagesContextLegacy(providerConfig.MachineImages)
	providerImages := validation.NewProviderImagesContext(providerConfig.MachineImages)

	for _, machineImage := range namespacedImages.Images {
		// Check that for each new image version defined in the NamespacedCloudProfile, the image is also defined in the providerConfig.
		_, existsInParent := parentImages.GetImage(machineImage.Name)
		if _, existsInProvider := providerImagesLegacy.GetImage(machineImage.Name); !existsInParent && !existsInProvider {
			allErrs = append(allErrs, field.Required(machineImagesPath,
				fmt.Sprintf("machine image %s is not defined in the NamespacedCloudProfile providerConfig", machineImage.Name),
			))
			continue
		}
		for _, version := range machineImage.Versions {
			_, existsInParent := parentImages.GetImageVersion(machineImage.Name, version.Version)
			if len(parentSpec.MachineCapabilities) == 0 {
				for _, expectedArchitecture := range version.Architectures {
					if _, existsInProvider := providerImagesLegacy.GetImageVersion(machineImage.Name, validation.VersionArchitectureKey(version.Version, expectedArchitecture)); !existsInParent && !existsInProvider {
						allErrs = append(allErrs, field.Required(machineImagesPath,
							fmt.Sprintf("machine image version %s@%s is not defined in the NamespacedCloudProfile providerConfig", machineImage.Name, version.Version),
						))
					}
				}
			} else {
				providerImageVersion, exists := providerImages.GetImageVersion(machineImage.Name, version.Version)
				if !exists {
					allErrs = append(allErrs, field.Required(machineImagesPath,
						fmt.Sprintf("machine image version %s@%s is not defined in the NamespacedCloudProfile providerConfig", machineImage.Name, version.Version),
					))
					continue
				}

				var v1betaVersion gardencorev1beta1.MachineImageVersion
				if err := gardencoreapi.Scheme.Convert(&version, &v1betaVersion, nil); err != nil {
					return append(allErrs, field.InternalError(machineImagesPath, err))
				}
				defaultedCapabilityFlavors := gardencorev1beta1helper.GetImageFlavorsWithAppliedDefaults(v1betaVersion.CapabilityFlavors, parentSpec.MachineCapabilities)
				for _, expectedCapabilityFlavor := range defaultedCapabilityFlavors {
					isFound := false
					for _, capabilityFlavor := range providerImageVersion.CapabilityFlavors {
						defaultedProviderCapabilities := gardencorev1beta1.GetCapabilitiesWithAppliedDefaults(capabilityFlavor.Capabilities, parentSpec.MachineCapabilities)
						if gardencorev1beta1helper.AreCapabilitiesEqual(expectedCapabilityFlavor.Capabilities, defaultedProviderCapabilities) {
							isFound = true
						}
					}
					if !isFound {
						allErrs = append(allErrs, field.Required(machineImagesPath,
							fmt.Sprintf("machine image version %s@%s has a capabilityFlavor %v not defined in the NamespacedCloudProfile providerConfig",
								machineImage.Name, version.Version, expectedCapabilityFlavor.Capabilities)))
					}
				}
			}
		}
	}
	for imageIdx, machineImage := range providerConfig.MachineImages {
		// Check that the machine image version is not already defined in the parent CloudProfile.
		if _, exists := parentImages.GetImage(machineImage.Name); exists {
			for versionIdx, version := range machineImage.Versions {
				if _, exists := parentImages.GetImageVersion(machineImage.Name, version.Version); exists {
					allErrs = append(allErrs, field.Forbidden(
						machineImagesPath.Index(imageIdx).Child("versions").Index(versionIdx),
						fmt.Sprintf("machine image version %s@%s is already defined in the parent CloudProfile", machineImage.Name, version.Version),
					))
				}
			}
		}
		// Check that the machine image version is defined in the NamespacedCloudProfile.
		if _, exists := namespacedImages.GetImage(machineImage.Name); !exists {
			allErrs = append(allErrs, field.Required(machineImagesPath.Index(imageIdx),
				fmt.Sprintf("machine image %s is not defined in the NamespacedCloudProfile .spec.machineImages", machineImage.Name),
			))
			continue
		}

		for versionIdx, version := range machineImage.Versions {
			profileImageVersion, exists := namespacedImages.GetImageVersion(machineImage.Name, version.Version)
			if !exists {
				allErrs = append(allErrs, field.Invalid(
					machineImagesPath.Index(imageIdx).Child("versions").Index(versionIdx),
					fmt.Sprintf("%s@%s", machineImage.Name, version.Version),
					"machine image version is not defined in the NamespacedCloudProfile",
				))
				// no need to check the architecture and capabilityFlavors below if the version does not exist in the profile
				continue
			}
			if len(parentSpec.MachineCapabilities) == 0 {
				providerConfigArchitecture := ptr.Deref(version.Architecture, constants.ArchitectureAMD64)
				if !slices.Contains(profileImageVersion.Architectures, providerConfigArchitecture) {
					allErrs = append(allErrs, field.Forbidden(
						field.NewPath("spec.providerConfig.machineImages"),
						fmt.Sprintf("machine image version %s@%s has an excess entry for architecture %q, which is not defined in the machineImages spec",
							machineImage.Name, version.Version, providerConfigArchitecture),
					))
				}
			} else {
				var v1betaVersion gardencorev1beta1.MachineImageVersion
				if err := gardencoreapi.Scheme.Convert(&profileImageVersion, &v1betaVersion, nil); err != nil {
					return append(allErrs, field.InternalError(machineImagesPath, err))
				}
				defaultedCapabilityFlavors := gardencorev1beta1helper.GetImageFlavorsWithAppliedDefaults(v1betaVersion.CapabilityFlavors, parentSpec.MachineCapabilities)
				// checked above that profileImageVersion exists
				providerImageVersion, _ := providerImages.GetImageVersion(machineImage.Name, version.Version)

				for _, capabilityFlavor := range providerImageVersion.CapabilityFlavors {
					defaultedProviderCapabilities := gardencorev1beta1.GetCapabilitiesWithAppliedDefaults(capabilityFlavor.Capabilities, parentSpec.MachineCapabilities)
					isFound := false

					for _, coreDefaultedCapabilityFlavor := range defaultedCapabilityFlavors {
						if gardencorev1beta1helper.AreCapabilitiesEqual(coreDefaultedCapabilityFlavor.Capabilities, defaultedProviderCapabilities) {
							isFound = true
						}
					}
					if !isFound {
						allErrs = append(allErrs, field.Forbidden(machineImagesPath,
							fmt.Sprintf("machine image version %s@%s has an excess capabilityFlavor %v, which is not defined in the machineImages spec",
								machineImage.Name, version.Version, capabilityFlavor.Capabilities)))
						// no need to check the regions if the capabilityFlavor is not defined in the providerConfig
						continue
					}
				}
			}
		}
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
