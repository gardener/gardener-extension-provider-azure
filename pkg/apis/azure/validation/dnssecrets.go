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
	// dnsCredentialMapping defines validation rules for DNS provider secrets using infrastructure keys
	dnsCredentialMapping = CredentialMapping{
		Fields: map[string]FieldSpec{
			azure.SubscriptionIDKey: {
				Required:    true,
				IsGUID:      true,
				IsImmutable: false,
			},
			azure.TenantIDKey: {
				Required:    true,
				IsGUID:      true,
				IsImmutable: false,
			},
			azure.ClientIDKey: {
				Required:    true,
				IsGUID:      true,
				IsImmutable: false,
			},
			azure.ClientSecretKey: {
				Required:    true,
				IsGUID:      false,
				IsImmutable: false,
			},
			azure.AzureCloud: {
				Required:      false,
				IsGUID:        false,
				IsImmutable:   false,
				AllowedValues: []string{"AzurePublic", "AzureChina", "AzureGovernment"},
			},
			azure.DNSAzureCloud: {
				Required:      false,
				IsGUID:        false,
				IsImmutable:   false,
				AllowedValues: []string{"AzurePublic", "AzureChina", "AzureGovernment"},
			},
		},
	}

	// dnsCredentialMappingAlias defines validation rules for DNS provider secrets using DNS-specific keys
	dnsCredentialMappingAlias = CredentialMapping{
		Fields: map[string]FieldSpec{
			azure.DNSSubscriptionIDKey: {
				Required:    true,
				IsGUID:      true,
				IsImmutable: false,
			},
			azure.DNSTenantIDKey: {
				Required:    true,
				IsGUID:      true,
				IsImmutable: false,
			},
			azure.DNSClientIDKey: {
				Required:    true,
				IsGUID:      true,
				IsImmutable: false,
			},
			azure.DNSClientSecretKey: {
				Required:    true,
				IsGUID:      false,
				IsImmutable: false,
			},
			azure.DNSAzureCloud: {
				Required:      false,
				IsGUID:        false,
				IsImmutable:   false,
				AllowedValues: []string{"AzurePublic", "AzureChina", "AzureGovernment"},
			},
		},
	}
)

// ValidateDNSProviderSecret validates Azure DNS provider credentials in a secret.
func ValidateDNSProviderSecret(secret *corev1.Secret, fldPath *field.Path) field.ErrorList {
	mapping := selectDNSCredentialMapping(secret)
	return mapping.Validate(secret, nil, fldPath, "DNS records")
}

// selectDNSCredentialMapping determines which credential mapping to use based on the
// keys present in the secret.
func selectDNSCredentialMapping(secret *corev1.Secret) *CredentialMapping {
	// Check for any DNS-specific key to determine which key set is used
	if _, ok := secret.Data[azure.DNSSubscriptionIDKey]; ok {
		return &dnsCredentialMappingAlias
	}
	if _, ok := secret.Data[azure.DNSTenantIDKey]; ok {
		return &dnsCredentialMappingAlias
	}
	if _, ok := secret.Data[azure.DNSClientIDKey]; ok {
		return &dnsCredentialMappingAlias
	}
	if _, ok := secret.Data[azure.DNSClientSecretKey]; ok {
		return &dnsCredentialMappingAlias
	}
	return &dnsCredentialMapping
}
