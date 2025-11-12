// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

var (
	// dnsCredentialMapping maps logical field names for DNS provider secrets to their keys
	dnsCredentialMapping = CredentialMapping{
		RequiredFields: map[string]string{
			"subscriptionID": azure.DNSSubscriptionIDKey,
			"tenantID":       azure.DNSTenantIDKey,
			"clientID":       azure.DNSClientIDKey,
			"clientSecret":   azure.DNSClientSecretKey,
		},
		OptionalFields: map[string]string{
			"azureCloud": azure.DNSAzureCloud,
		},
		GUIDFields: map[string]string{
			"subscriptionID": azure.DNSSubscriptionIDKey,
			"tenantID":       azure.DNSTenantIDKey,
			"clientID":       azure.DNSClientIDKey,
		},
	}

	// allowedAzureCloudValues defines the valid values for Azure Cloud
	allowedAzureCloudValues = []string{
		"AzurePublic",
		"AzureChina",
		"AzureGovernment",
	}
)

// ValidateDNSProviderSecret validates Azure DNS provider credentials in a secret.
func ValidateDNSProviderSecret(secret *corev1.Secret, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	dataPath := fldPath.Child("data")

	// Validate required credentials
	allErrs = append(allErrs, ValidateRequiredCredentials(secret, dnsCredentialMapping, dataPath)...)

	// Validate optional credentials
	allErrs = append(allErrs, ValidateOptionalCredentials(secret, dnsCredentialMapping, dataPath)...)

	// Validate credential formats
	allErrs = append(allErrs, ValidateCredentialFormats(secret, dnsCredentialMapping, dataPath)...)

	// Validate no unexpected fields
	allErrs = append(allErrs, ValidateNoUnexpectedFields(secret, dnsCredentialMapping, dataPath)...)

	// Validate Azure Cloud values if present
	allErrs = append(allErrs, ValidatePredefinedValues(secret, azure.DNSAzureCloud, allowedAzureCloudValues, dataPath)...)

	return allErrs
}
