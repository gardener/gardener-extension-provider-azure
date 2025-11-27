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

	allErrs = append(allErrs, cm.validateRequired(secret, dataPath)...)
	allErrs = append(allErrs, cm.validateFormats(secret, dataPath)...)
	allErrs = append(allErrs, cm.validateNoUnexpected(secret, dataPath)...)
	allErrs = append(allErrs, cm.validatePredefinedValues(secret, dataPath)...)

	if oldSecret != nil {
		allErrs = append(allErrs, cm.validateImmutable(secret, oldSecret, resourceType, dataPath)...)
	}

	return allErrs
}

// validateRequired validates that all required credential fields are present and non-empty
func (cm *CredentialMapping) validateRequired(secret *corev1.Secret, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	secretKey := fmt.Sprintf("%s/%s", secret.Namespace, secret.Name)

	for dataKey, spec := range cm.Fields {
		if !spec.Required {
			continue
		}

		value, exists := secret.Data[dataKey]
		if !exists {
			allErrs = append(allErrs, field.Required(fldPath.Key(dataKey),
				fmt.Sprintf("missing required field %q in secret %s", dataKey, secretKey)))
			continue
		}
		if len(value) == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(dataKey), "",
				fmt.Sprintf("field %q cannot be empty in secret %s", dataKey, secretKey)))
		}
	}

	return allErrs
}

// validateFormats validates the format of credential values
func (cm *CredentialMapping) validateFormats(secret *corev1.Secret, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	secretKey := fmt.Sprintf("%s/%s", secret.Namespace, secret.Name)

	for dataKey, spec := range cm.Fields {
		value, exists := secret.Data[dataKey]
		if !exists || len(value) == 0 {
			continue
		}

		// Validate GUID format
		if spec.IsGUID && !guidRegex.Match(value) {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(dataKey), "(hidden)",
				fmt.Sprintf("field %q must be a valid GUID in secret %s", dataKey, secretKey)))
		}

		// Validate no leading/trailing whitespace
		valueStr := string(value)
		if strings.TrimSpace(valueStr) != valueStr {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(dataKey), "(hidden)",
				fmt.Sprintf("field %q must not contain leading or trailing whitespace in secret %s", dataKey, secretKey)))
		}
	}

	return allErrs
}

// validateNoUnexpected validates that no unexpected fields are present in the secret
func (cm *CredentialMapping) validateNoUnexpected(secret *corev1.Secret, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	secretKey := fmt.Sprintf("%s/%s", secret.Namespace, secret.Name)

	allowedKeys := sets.New[string]()
	for dataKey := range cm.Fields {
		allowedKeys.Insert(dataKey)
	}

	for key := range secret.Data {
		if !allowedKeys.Has(key) {
			allErrs = append(allErrs, field.Forbidden(fldPath.Key(key),
				fmt.Sprintf("unexpected field %q in secret %s", key, secretKey)))
		}
	}

	return allErrs
}

// validateImmutable validates that immutable fields haven't changed
func (cm *CredentialMapping) validateImmutable(newSecret, oldSecret *corev1.Secret, resourceType string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	secretKey := fmt.Sprintf("%s/%s", newSecret.Namespace, newSecret.Name)

	for dataKey, spec := range cm.Fields {
		if !spec.IsImmutable {
			continue
		}

		if !bytes.Equal(newSecret.Data[dataKey], oldSecret.Data[dataKey]) {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(dataKey), "(hidden)",
				fmt.Sprintf("field %q must not be changed for existing %s in secret %s", dataKey, resourceType, secretKey)))
		}
	}

	return allErrs
}

// validatePredefinedValues validates that a field contains one of the predefined values
func (cm *CredentialMapping) validatePredefinedValues(secret *corev1.Secret, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for dataKey, spec := range cm.Fields {
		if len(spec.AllowedValues) == 0 {
			continue
		}

		value, exists := secret.Data[dataKey]
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
