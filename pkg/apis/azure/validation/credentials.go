// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var (
	guidRegex = regexp.MustCompile("^[0-9A-Fa-f]{8}-([0-9A-Fa-f]{4}-){3}[0-9A-Fa-f]{12}$")
)

// FieldSpec defines validation rules for a single credential field
type FieldSpec struct {
	Required      bool
	IsGUID        bool
	IsImmutable   bool
	AllowedValues []string
}

// CredentialMapping defines all credential fields and their validation rules
// The map key is the data key used in the secret
type CredentialMapping struct {
	Fields map[string]FieldSpec
}

// Validate performs complete validation of credentials in a secret
func (cm *CredentialMapping) Validate(secret, oldSecret *corev1.Secret, fldPath *field.Path, resourceType string) field.ErrorList {
	allErrs := field.ErrorList{}
	dataPath := fldPath.Child("data")
	secretKey := fmt.Sprintf("%s/%s", secret.Namespace, secret.Name)

	allErrs = append(allErrs, cm.validateRequired(secret.Data, secretKey, dataPath)...)
	allErrs = append(allErrs, cm.validateFormats(secret.Data, secretKey, dataPath)...)
	allErrs = append(allErrs, cm.validateNoUnexpected(secret.Data, secretKey, dataPath)...)
	allErrs = append(allErrs, cm.validatePredefinedValues(secret.Data, dataPath)...)

	if oldSecret != nil {
		allErrs = append(allErrs, cm.validateImmutable(secret.Data, oldSecret.Data, resourceType, secretKey, dataPath)...)
	}

	return allErrs
}

// ValidateData performs validation of credentials from a raw data map.
// It accepts a map[string][]byte directly, allowing validation of both
// corev1.Secret and gardencorev1beta1.InternalSecret data.
// When secretKey is non-empty it is included in error messages for context.
// When oldData is non-nil, immutability constraints are also checked.
func (cm *CredentialMapping) ValidateData(data, oldData map[string][]byte, resourceType string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	dataPath := fldPath.Child("data")

	allErrs = append(allErrs, cm.validateRequired(data, dataPath)...)
	allErrs = append(allErrs, cm.validateFormats(data, dataPath)...)
	allErrs = append(allErrs, cm.validateNoUnexpected(data, dataPath)...)
	allErrs = append(allErrs, cm.validatePredefinedValues(data, dataPath)...)

	if oldData != nil {
		allErrs = append(allErrs, cm.validateImmutable(data, oldData, resourceType, dataPath)...)
	}

	return allErrs
}

// validateRequired validates that all required credential fields are present and non-empty.
// When secretKey is non-empty it is appended to error messages for context.
func (cm *CredentialMapping) validateRequired(data map[string][]byte, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for dataKey, spec := range cm.Fields {
		if !spec.Required {
			continue
		}

		value, exists := data[dataKey]
		if !exists {
			allErrs = append(allErrs, field.Required(fldPath.Key(dataKey), fmt.Sprintf("missing required field %q", dataKey)))
			continue
		}
		if len(value) == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(dataKey), "", fmt.Sprintf("field %q cannot be empty", dataKey)))
		}
	}

	return allErrs
}

// validateFormats validates the format of credential values.
// When secretKey is non-empty it is appended to error messages for context.
func (cm *CredentialMapping) validateFormats(data map[string][]byte, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for dataKey, spec := range cm.Fields {
		value, exists := data[dataKey]
		if !exists || len(value) == 0 {
			continue
		}

		if spec.IsGUID && !guidRegex.Match(value) {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(dataKey), "(hidden)", fmt.Sprintf("field %q must be a valid GUID", dataKey)))
		}

		valueStr := string(value)
		if strings.TrimSpace(valueStr) != valueStr {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(dataKey), "(hidden)", fmt.Sprintf("field %q must not contain leading or trailing whitespace", dataKey)))
		}
	}

	return allErrs
}

// validateNoUnexpected validates that no unexpected fields are present.
// When secretKey is non-empty it is appended to error messages for context.
func (cm *CredentialMapping) validateNoUnexpected(data map[string][]byte, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allowedKeys := sets.New[string]()
	for dataKey := range cm.Fields {
		allowedKeys.Insert(dataKey)
	}

	for key := range data {
		if !allowedKeys.Has(key) {
			allErrs = append(allErrs, field.Forbidden(fldPath.Key(key), fmt.Sprintf("unexpected field %q", key)))
		}
	}

	return allErrs
}

// validateImmutable validates that immutable fields haven't changed
func (cm *CredentialMapping) validateImmutable(newData, oldData map[string][]byte, resourceType string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for dataKey, spec := range cm.Fields {
		if !spec.IsImmutable {
			continue
		}

		if !bytes.Equal(newData[dataKey], oldData[dataKey]) {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(dataKey), "(hidden)",
				fmt.Sprintf("field %q must not be changed for existing %s", dataKey, resourceType)))
		}
	}

	return allErrs
}

// validatePredefinedValues validates that a field contains one of the predefined values
func (cm *CredentialMapping) validatePredefinedValues(data map[string][]byte, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for dataKey, spec := range cm.Fields {
		if len(spec.AllowedValues) == 0 {
			continue
		}

		value, exists := data[dataKey]
		if !exists || len(value) == 0 {
			continue
		}

		allowedSet := sets.New(spec.AllowedValues...)
		if !allowedSet.Has(string(value)) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Key(dataKey), string(value), spec.AllowedValues))
		}
	}

	return allErrs
}
