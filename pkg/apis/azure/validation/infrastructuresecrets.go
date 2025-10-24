// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

var (
	// infrastructureCredentialMapping maps the expected logical field names for infrastructure provider secrets to their keys
	infrastructureCredentialMapping = CredentialMapping{
		RequiredFields: map[string]string{
			"subscriptionID": azure.SubscriptionIDKey,
			"tenantID":       azure.TenantIDKey,
		},
		OptionalFields: map[string]string{
			"clientID":     azure.ClientIDKey,
			"clientSecret": azure.ClientSecretKey,
		},
		GUIDFields: map[string]string{
			"subscriptionID": azure.SubscriptionIDKey,
			"tenantID":       azure.TenantIDKey,
			"clientID":       azure.ClientIDKey,
		},
		ImmutableFields: map[string]string{
			"subscriptionID": azure.SubscriptionIDKey,
			"tenantID":       azure.TenantIDKey,
		},
	}
)

// ValidateCloudProviderSecret validates Azure infrastructure credentials
func ValidateCloudProviderSecret(secret, oldSecret *corev1.Secret, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	dataPath := fldPath.Child("data")

	// Validate required credentials
	allErrs = append(allErrs, ValidateRequiredCredentials(secret, infrastructureCredentialMapping, dataPath)...)

	// Validate optional credentials
	allErrs = append(allErrs, ValidateOptionalCredentials(secret, infrastructureCredentialMapping, dataPath)...)

	// Validate credential formats
	allErrs = append(allErrs, ValidateCredentialFormats(secret, infrastructureCredentialMapping, dataPath)...)

	// Validate no unexpected fields
	allErrs = append(allErrs, ValidateNoUnexpectedFields(secret, infrastructureCredentialMapping, dataPath)...)

	// Validate client credentials consistency (both clientID and clientSecret must be provided together)
	allErrs = append(allErrs, ValidateClientCredentialsConsistency(secret, dataPath)...)

	// Validate immutable fields on update
	if oldSecret != nil {
		allErrs = append(allErrs, ValidateImmutableCredentials(secret, oldSecret, infrastructureCredentialMapping, "shoot clusters", dataPath)...)
	}

	return allErrs
}

// ValidateClientCredentialsConsistency ensures that clientID and clientSecret are provided together
func ValidateClientCredentialsConsistency(secret *corev1.Secret, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	secretKey := fmt.Sprintf("%s/%s", secret.Namespace, secret.Name)

	clientID, hasClientID := secret.Data[azure.ClientIDKey]
	clientSecret, hasClientSecret := secret.Data[azure.ClientSecretKey]

	if hasClientID && !hasClientSecret {
		allErrs = append(allErrs, field.Required(fldPath.Key(azure.ClientSecretKey),
			fmt.Sprintf("%q must be provided when %q is specified in secret %s", azure.ClientSecretKey, azure.ClientIDKey, secretKey)))
	}

	if hasClientSecret && !hasClientID {
		allErrs = append(allErrs, field.Required(fldPath.Key(azure.ClientIDKey),
			fmt.Sprintf("%q must be provided when %q is specified in secret %s", azure.ClientIDKey, azure.ClientSecretKey, secretKey)))
	}

	if hasClientID && len(clientID) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Key(azure.ClientIDKey), "",
			fmt.Sprintf("%q cannot be empty if specified in secret %s", azure.ClientIDKey, secretKey)))
	}

	if hasClientSecret && len(clientSecret) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Key(azure.ClientSecretKey), "",
			fmt.Sprintf("%q cannot be empty if specified in secret %s", azure.ClientSecretKey, secretKey)))
	}

	return allErrs
}
