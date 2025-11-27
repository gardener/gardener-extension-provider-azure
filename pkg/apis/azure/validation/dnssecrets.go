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
	// dnsCredentialMapping defines validation rules for DNS provider secrets
	dnsCredentialMapping = CredentialMapping{
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
	return dnsCredentialMapping.Validate(secret, nil, fldPath, "DNS records")
}
