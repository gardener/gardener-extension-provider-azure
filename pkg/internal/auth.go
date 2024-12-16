// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

// ClientAuth represents a Azure Client Auth credentials.
type ClientAuth struct {
	// SubscriptionID is the Azure subscription ID.
	SubscriptionID string `yaml:"subscriptionID"`
	// TenantID is the Azure tenant ID.
	TenantID string `yaml:"tenantID"`
	// ClientID is the Azure client ID.
	ClientID string `yaml:"clientID"`
	// ClientSecret is the Azure client secret.
	ClientSecret string `yaml:"clientSecret"`
}

// GetAzClientCredentials returns the credential struct consumed by the Azure client
func (clientAuth ClientAuth) GetAzClientCredentials() (*azidentity.ClientSecretCredential, error) {
	return azidentity.NewClientSecretCredential(clientAuth.TenantID, clientAuth.ClientID, clientAuth.ClientSecret, nil)
}

// GetClientAuthData retrieves the client auth data specified by the secret reference.
func GetClientAuthData(ctx context.Context, c client.Client, secretRef corev1.SecretReference, allowDNSKeys bool) (*ClientAuth, *corev1.Secret, error) {
	secret, err := extensionscontroller.GetSecretByReference(ctx, c, &secretRef)
	if err != nil {
		return nil, nil, err
	}

	return NewClientAuthDataFromSecret(secret, allowDNSKeys)
}

// NewClientAuthDataFromSecret reads the client auth details from the given secret.
func NewClientAuthDataFromSecret(secret *corev1.Secret, allowDNSKeys bool) (*ClientAuth, *corev1.Secret, error) {
	var altSubscriptionIDIDKey, altTenantIDKey, altClientIDKey, altClientSecretKey *string
	if allowDNSKeys {
		altSubscriptionIDIDKey = ptr.To(azure.DNSSubscriptionIDKey)
		altTenantIDKey = ptr.To(azure.DNSTenantIDKey)
		altClientIDKey = ptr.To(azure.DNSClientIDKey)
		altClientSecretKey = ptr.To(azure.DNSClientSecretKey)
	}

	subscriptionID, ok := getSecretDataValue(secret, azure.SubscriptionIDKey, altSubscriptionIDIDKey)
	if !ok {
		return nil, nil, fmt.Errorf("secret %s/%s doesn't have a subscription ID", secret.Namespace, secret.Name)
	}

	tenantID, ok := getSecretDataValue(secret, azure.TenantIDKey, altTenantIDKey)
	if !ok {
		return nil, nil, fmt.Errorf("secret %s/%s doesn't have a tenant ID", secret.Namespace, secret.Name)
	}

	clientID, ok := getSecretDataValue(secret, azure.ClientIDKey, altClientIDKey)
	if !ok {
		return nil, nil, fmt.Errorf("secret %s/%s doesn't have a client ID", secret.Namespace, secret.Name)
	}

	clientSecret, ok := getSecretDataValue(secret, azure.ClientSecretKey, altClientSecretKey)
	if !ok {
		return nil, nil, fmt.Errorf("secret %s/%s doesn't have a client secret", secret.Namespace, secret.Name)
	}

	return &ClientAuth{
		SubscriptionID: string(subscriptionID),
		TenantID:       string(tenantID),
		ClientID:       string(clientID),
		ClientSecret:   string(clientSecret),
	}, secret, nil
}

func getSecretDataValue(secret *corev1.Secret, key string, altKey *string) ([]byte, bool) {
	if value, ok := secret.Data[key]; ok {
		return value, true
	}
	if altKey != nil {
		if value, ok := secret.Data[*altKey]; ok {
			return value, true
		}
	}
	return nil, false
}
