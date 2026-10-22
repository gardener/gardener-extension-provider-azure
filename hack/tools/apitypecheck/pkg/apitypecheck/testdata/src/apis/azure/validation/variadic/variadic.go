// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package validation // want `field WidgetConfig\.Tags \(apis/azure\) has no field\.Child\("tags"\) reference`

import (
	"apis/azure"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateWidgetConfig references "tags" only as a variadic second argument to
// Child, not as the first argument. The analyzer must not treat it as validated.
func ValidateWidgetConfig(cfg *azure.WidgetConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if cfg.Color == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("color"), "color must not be empty"))
	}
	if cfg.Size <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("size", "tags"), cfg.Size, "size must be positive"))
	}
	return allErrs
}
