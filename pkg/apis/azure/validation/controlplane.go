// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	featurevalidation "github.com/gardener/gardener/pkg/utils/validation/features"
	"k8s.io/apimachinery/pkg/util/validation/field"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
)

// ValidateControlPlaneConfig validates a ControlPlaneConfig object.
func ValidateControlPlaneConfig(controlPlaneConfig *apisazure.ControlPlaneConfig, version string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if controlPlaneConfig.CloudControllerManager != nil {
		allErrs = append(allErrs, featurevalidation.ValidateFeatureGates(controlPlaneConfig.CloudControllerManager.FeatureGates, version, fldPath.Child("cloudControllerManager", "featureGates"))...)
	}

	return allErrs
}
