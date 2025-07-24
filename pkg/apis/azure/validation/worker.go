// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"slices"

	"github.com/gardener/gardener/pkg/apis/core"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"

	apiazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
)

// ValidateWorkerConfig validates a WorkerConfig object.
func ValidateWorkerConfig(workerConfig *apiazure.WorkerConfig, dataVolumes []core.DataVolume, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if workerConfig != nil {
		allErrs = append(allErrs, validateNodeTemplate(workerConfig.NodeTemplate, fldPath)...)
		allErrs = append(allErrs, validateDataVolumeConf(workerConfig.DataVolumes, dataVolumes, fldPath)...)
	}

	return allErrs
}

func validateNodeTemplate(nodeTemplate *extensionsv1alpha1.NodeTemplate, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if nodeTemplate == nil {
		return nil
	}
	for _, capacityAttribute := range []corev1.ResourceName{corev1.ResourceCPU, "gpu", corev1.ResourceMemory} {
		value, ok := nodeTemplate.Capacity[capacityAttribute]
		if !ok {
			allErrs = append(allErrs, field.Required(fldPath.Child("nodeTemplate").Child("capacity"), fmt.Sprintf("%s is a mandatory field", capacityAttribute)))
			continue
		}
		allErrs = append(allErrs, validateResourceQuantityValue(capacityAttribute, value, fldPath.Child("nodeTemplate").Child("capacity").Child(string(capacityAttribute)))...)
	}

	return allErrs
}

func validateDataVolumeConf(dataVolumeConfigs []apiazure.DataVolume, dataVolumes []core.DataVolume, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	imageRefPath := fldPath.Child("dataVolumes").Child("ImageRef")
	namePath := fldPath.Child("dataVolumes").Child("Name")
	var dataVolumeNames []string

	for _, dataVolume := range dataVolumes {
		dataVolumeNames = append(dataVolumeNames, dataVolume.Name)
	}

	for _, dataVolumeConf := range dataVolumeConfigs {
		if dataVolumeConf.ImageRef != nil && *dataVolumeConf.ImageRef == (apiazure.Image{}) {
			allErrs = append(allErrs, field.Invalid(imageRefPath, dataVolumeConf.ImageRef, "imageRef is defined but empty"))
		}
		if !slices.Contains(dataVolumeNames, dataVolumeConf.Name) {
			allErrs = append(allErrs, field.Invalid(namePath, dataVolumeConf.Name, "no dataVolume with this name exists"))
		}
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
