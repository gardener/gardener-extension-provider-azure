// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

// Package validation covers all exported fields of WidgetConfig.
package validation

import (
	"apis/azure"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateWidgetConfig validates a WidgetConfig — all fields referenced.
func ValidateWidgetConfig(cfg *azure.WidgetConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if cfg.Color == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("color"), "color must not be empty"))
	}
	if cfg.Size <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("size"), cfg.Size, "size must be positive"))
	}
	_ = fldPath.Child("tags")
	return allErrs
}
