// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
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

// CredentialMapping maps logical credential field names to their credential keys
// to enable usage across different credential types (infrastructure, DNS)
type CredentialMapping struct {
	// RequiredFields keeps fields that are required
	RequiredFields map[string]string
	// OptionalFields keeps fields that are optional
	OptionalFields map[string]string
	// GUIDFields keeps fields that must be validated as GUIDs
	GUIDFields map[string]string
	// ImmutableFields keeps fields that must not change once set
	ImmutableFields map[string]string
}

// ValidateRequiredCredentials validates that all required credential fields are present and non-empty
func ValidateRequiredCredentials(secret *corev1.Secret, mapping CredentialMapping, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	secretKey := fmt.Sprintf("%s/%s", secret.Namespace, secret.Name)

	for logicalName, dataKey := range mapping.RequiredFields {
		value, exists := secret.Data[dataKey]
		if !exists {
			allErrs = append(allErrs, field.Required(fldPath.Key(dataKey), fmt.Sprintf("missing required field %s in secret %s", logicalName, secretKey)))
			continue
		}
		if len(value) == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(dataKey), "", fmt.Sprintf("field %s cannot be empty in secret %s", logicalName, secretKey)))
		}
	}

	return allErrs
}

// ValidateOptionalCredentials validates optional credential fields if they are present
func ValidateOptionalCredentials(secret *corev1.Secret, mapping CredentialMapping, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	secretKey := fmt.Sprintf("%s/%s", secret.Namespace, secret.Name)

	for logicalName, dataKey := range mapping.OptionalFields {
		if value, exists := secret.Data[dataKey]; exists && len(value) == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(dataKey), "", fmt.Sprintf("field %s cannot be empty if specified in secret %s", logicalName, secretKey)))
		}
	}

	return allErrs
}

// ValidateCredentialFormats validates the format of credential values
func ValidateCredentialFormats(secret *corev1.Secret, mapping CredentialMapping, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	secretKey := fmt.Sprintf("%s/%s", secret.Namespace, secret.Name)

	// Validate GUID fields
	for logicalName, dataKey := range mapping.GUIDFields {
		if value, exists := secret.Data[dataKey]; exists && len(value) > 0 {
			if !guidRegex.Match(value) {
				allErrs = append(allErrs, field.Invalid(fldPath.Key(dataKey), "(hidden)", fmt.Sprintf("field %s must be a valid GUID in secret %s", logicalName, secretKey)))
			}
		}
	}

	// Validate no trailing whitespace for all credential fields
	allFields := make(map[string]string)
	for logicalName, dataKey := range mapping.RequiredFields {
		allFields[logicalName] = dataKey
	}
	for logicalName, dataKey := range mapping.OptionalFields {
		allFields[logicalName] = dataKey
	}

	for logicalName, dataKey := range allFields {
		if value, exists := secret.Data[dataKey]; exists && len(value) > 0 {
			valueStr := string(value)
			if strings.TrimSpace(valueStr) != valueStr {
				allErrs = append(allErrs, field.Invalid(fldPath.Key(dataKey), "(hidden)", fmt.Sprintf("field %s must not contain leading or trailing whitespace in secret %s", logicalName, secretKey)))
			}
		}
	}

	return allErrs
}

// ValidateNoUnexpectedFields validates that no unexpected fields are present in the secret
func ValidateNoUnexpectedFields(secret *corev1.Secret, mapping CredentialMapping, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	secretKey := fmt.Sprintf("%s/%s", secret.Namespace, secret.Name)

	allowedKeys := sets.New[string]()
	for _, key := range mapping.RequiredFields {
		allowedKeys.Insert(key)
	}
	for _, key := range mapping.OptionalFields {
		allowedKeys.Insert(key)
	}

	for key := range secret.Data {
		if !allowedKeys.Has(key) {
			allErrs = append(allErrs, field.Forbidden(fldPath.Key(key), fmt.Sprintf("unexpected field %q in secret %s", key, secretKey)))
		}
	}

	return allErrs
}

// ValidateImmutableCredentials validates that immutable fields haven't changed
func ValidateImmutableCredentials(newSecret, oldSecret *corev1.Secret, mapping CredentialMapping, resourceType string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	secretKey := fmt.Sprintf("%s/%s", newSecret.Namespace, newSecret.Name)

	for logicalName, dataKey := range mapping.ImmutableFields {
		newValue := string(newSecret.Data[dataKey])
		oldValue := string(oldSecret.Data[dataKey])
		if newValue != oldValue {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(dataKey), "(hidden)", fmt.Sprintf("field %s must not be changed for existing %s in secret %s", logicalName, resourceType, secretKey)))
		}
	}

	return allErrs
}

// ValidatePredefinedValues validates that a field contains one of the predefined values
func ValidatePredefinedValues(secret *corev1.Secret, dataKey string, allowedValues []string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if value, exists := secret.Data[dataKey]; exists && len(value) > 0 {
		valueStr := string(value)
		allowedSet := sets.New(allowedValues...)
		if !allowedSet.Has(valueStr) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Key(dataKey), valueStr, allowedValues))
		}
	}

	return allErrs
}
