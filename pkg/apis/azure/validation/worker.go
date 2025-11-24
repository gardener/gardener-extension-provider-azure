// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/gardener/gardener/pkg/apis/core"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	apiazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
)

// ValidateWorkerConfig validates a WorkerConfig object.
func ValidateWorkerConfig(workerConfig *apiazure.WorkerConfig, dataVolumes []core.DataVolume, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if workerConfig == nil {
		return allErrs
	}

	if diagnosticsProfile := workerConfig.DiagnosticsProfile; diagnosticsProfile != nil {
		if diagnosticsProfile.StorageURI != nil {
			allErrs = append(allErrs, storageURIValidation(*diagnosticsProfile.StorageURI, fldPath.Child("diagnosticsProfile").Child("storageURI"))...)
		}
	}

	allErrs = append(allErrs, validateNodeTemplate(workerConfig.NodeTemplate, fldPath.Child("nodeTemplate"))...)
	allErrs = append(allErrs, validateDataVolumeConf(workerConfig.DataVolumes, dataVolumes, fldPath.Child("dataVolumes"))...)
	allErrs = append(allErrs, validateOSDiskConf(workerConfig.Volume, fldPath.Child("volume"))...)

	if capacityReservationConfig := workerConfig.CapacityReservation; capacityReservationConfig != nil {
		allErrs = append(allErrs, validateCapacityReservationConfig(capacityReservationConfig, fldPath.Child("capacityReservation"))...)
	}

	return allErrs
}

func validateNodeTemplate(nodeTemplate *extensionsv1alpha1.NodeTemplate, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if nodeTemplate == nil {
		return nil
	}
	resources := []corev1.ResourceName{corev1.ResourceCPU, "gpu", corev1.ResourceMemory, corev1.ResourceStorage, corev1.ResourceEphemeralStorage}
	resourceSet := sets.New[corev1.ResourceName](resources...)
	for resourceName := range nodeTemplate.Capacity {
		if !resourceSet.Has(resourceName) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("capacity").Child(string(resourceName)), resourceName, fmt.Sprintf("%s is an unsupported resource name. Valid values are: %v", resourceName, resourceSet.UnsortedList())))
		}
	}

	for _, capacityAttribute := range resources {
		value, ok := nodeTemplate.Capacity[capacityAttribute]
		if !ok {
			continue
		}
		allErrs = append(allErrs, validateResourceQuantityValue(capacityAttribute, value, fldPath.Child("capacity").Child(string(capacityAttribute)))...)
	}

	return allErrs
}

func validateDataVolumeConf(dataVolumeConfigs []apiazure.DataVolume, dataVolumes []core.DataVolume, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	var dataVolumeNames []string

	for _, dataVolume := range dataVolumes {
		dataVolumeNames = append(dataVolumeNames, dataVolume.Name)
	}

	for idx, dataVolumeConf := range dataVolumeConfigs {
		dvPath := fldPath.Index(idx)
		imgRefPath := dvPath.Child("imageRef")

		if imgRef := dataVolumeConf.ImageRef; imgRef != nil {
			if !slices.Contains(dataVolumeNames, dataVolumeConf.Name) {
				allErrs = append(allErrs, field.Invalid(dvPath.Child("name"), dataVolumeConf.Name, "no dataVolume with this name exists"))
			}

			if *imgRef == (apiazure.Image{}) {
				allErrs = append(allErrs, field.Invalid(imgRefPath, dataVolumeConf.ImageRef, "imageRef is defined but empty"))
			}
			if imgRef.URN != nil {
				allErrs = append(allErrs, urnValidation(*imgRef.URN, imgRefPath.Child("urn"))...)
			}
			if imgRef.CommunityGalleryImageID != nil {
				allErrs = append(allErrs, communityGalleryImageIDValidation(*imgRef.CommunityGalleryImageID, imgRefPath.Child("communityGalleryImageID"))...)
			}
			if imgRef.SharedGalleryImageID != nil {
				allErrs = append(allErrs, sharedGalleryImageIDValidation(*imgRef.SharedGalleryImageID, imgRefPath.Child("sharedGalleryImageID"))...)
			}
			if imgRef.ID != nil {
				resourceID, err := arm.ParseResourceID(*imgRef.ID)
				if err != nil {
					return append(allErrs, field.Invalid(imgRefPath.Child("id"), *imgRef.ID, fmt.Sprintf("invalid image ID: %v", err)))
				}

				allErrs = append(allErrs, validateResourceID(resourceID, ptr.To("images"), imgRefPath.Child("images"))...)
			}
		}
	}
	return allErrs
}

func validateOSDiskConf(osDiskConf *apiazure.Volume, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if osDiskConf == nil {
		return nil
	}

	allErrs = append(allErrs, validateOsDiskCaching(osDiskConf.Caching, fldPath.Child("caching"))...)

	return allErrs
}

func validateOsDiskCaching(cachingType *string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if cachingType == nil {
		return nil
	}

	if !slices.Contains(armcompute.PossibleCachingTypesValues(), armcompute.CachingTypes(*cachingType)) {
		allErrs = append(allErrs, field.NotSupported(fldPath, *cachingType, armcompute.PossibleCachingTypesValues()))
	}

	return allErrs
}

func validateResourceQuantityValue(key corev1.ResourceName, value resource.Quantity, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if value.Cmp(resource.Quantity{}) < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, value.String(), fmt.Sprintf("%s value must not be negative", key)))
	}

	return allErrs
}

func validateCapacityReservationConfig(capacityReservationConfig *apiazure.CapacityReservation, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if capacityReservationGroupID := capacityReservationConfig.CapacityReservationGroupID; capacityReservationGroupID != nil {
		resourceID, err := arm.ParseResourceID(*capacityReservationGroupID)
		if err != nil {
			allErrs = append(
				allErrs,
				field.Invalid(
					fldPath.Child("capacityReservationGroupID"),
					*capacityReservationGroupID,
					fmt.Sprintf("invalid Azure resource ID: %v", err),
				),
			)
		} else if resourceType := resourceID.ResourceType.Type; !strings.EqualFold(resourceType, "CapacityReservationGroups") {
			allErrs = append(
				allErrs,
				field.Invalid(
					fldPath.Child(
						"capacityReservationGroupID"),
					*capacityReservationGroupID,
					fmt.Sprintf("provided resource ID must be of type 'CapacityReservationGroups', got '%s'", resourceType),
				),
			)
		}
	}

	return allErrs
}
