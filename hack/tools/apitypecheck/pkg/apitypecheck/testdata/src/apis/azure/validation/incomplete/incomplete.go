// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package validation // want `field WidgetConfig\.Tags \(apis/azure\) has no field\.Child\("tags"\) reference`

import (
	"apis/azure"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateWidgetConfig validates a WidgetConfig — tags field is intentionally not referenced.
func ValidateWidgetConfig(cfg *azure.WidgetConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if cfg.Color == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("color"), "color must not be empty"))
	}
	if cfg.Size <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("size"), cfg.Size, "size must be positive"))
	}
	// tags is intentionally NOT referenced here to trigger the diagnostic
	return allErrs
}
