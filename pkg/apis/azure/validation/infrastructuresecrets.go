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
	// infrastructureCredentialMapping defines validation rules for infrastructure provider secrets
	infrastructureCredentialMapping = CredentialMapping{
		Fields: map[string]FieldSpec{
			azure.SubscriptionIDKey: {
				Required:    true,
				IsGUID:      true,
				IsImmutable: true,
			},
			azure.TenantIDKey: {
				Required:    true,
				IsGUID:      true,
				IsImmutable: true,
			},
			azure.ClientIDKey: {
				Required:    false,
				IsGUID:      true,
				IsImmutable: false,
			},
			azure.ClientSecretKey: {
				Required:    false,
				IsGUID:      false,
				IsImmutable: false,
			},
		},
	}
)

// ValidateCloudProviderSecret validates Azure infrastructure credentials
func ValidateCloudProviderSecret(secret, oldSecret *corev1.Secret, fldPath *field.Path) field.ErrorList {
	return infrastructureCredentialMapping.Validate(secret, oldSecret, fldPath, "shoot clusters")
}

// ValidateCloudProviderSecretData validates Azure infrastructure credentials from a raw data map.
// It accepts a map[string][]byte directly, allowing validation of both corev1.Secret and
// gardencorev1beta1.InternalSecret data. When secretKey is non-empty it is included in error
// messages. When oldData is non-nil, immutability constraints are also checked.
func ValidateCloudProviderSecretData(data, oldData map[string][]byte, fldPath *field.Path) field.ErrorList {
	return infrastructureCredentialMapping.ValidateData(data, oldData, "shoot clusters", fldPath)
}
