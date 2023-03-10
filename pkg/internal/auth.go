// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal

import (
	"context"
	"fmt"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	azureautorest "github.com/Azure/go-autorest/autorest"
	azureauth "github.com/Azure/go-autorest/autorest/azure/auth"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
func GetClientAuthData(ctx context.Context, c client.Client, secretRef corev1.SecretReference, allowDNSKeys bool) (*ClientAuth, error) {
	secret, err := extensionscontroller.GetSecretByReference(ctx, c, &secretRef)
	if err != nil {
		return nil, err
	}

	return NewClientAuthDataFromSecret(secret, allowDNSKeys)
}

// NewClientAuthDataFromSecret reads the client auth details from the given secret.
func NewClientAuthDataFromSecret(secret *corev1.Secret, allowDNSKeys bool) (*ClientAuth, error) {
	var altSubscriptionIDIDKey, altTenantIDKey, altClientIDKey, altClientSecretKey *string
	if allowDNSKeys {
		altSubscriptionIDIDKey = pointer.String(azure.DNSSubscriptionIDKey)
		altTenantIDKey = pointer.String(azure.DNSTenantIDKey)
		altClientIDKey = pointer.String(azure.DNSClientIDKey)
		altClientSecretKey = pointer.String(azure.DNSClientSecretKey)
	}

	subscriptionID, ok := getSecretDataValue(secret, azure.SubscriptionIDKey, altSubscriptionIDIDKey)
	if !ok {
		return nil, fmt.Errorf("secret %s/%s doesn't have a subscription ID", secret.Namespace, secret.Name)
	}

	tenantID, ok := getSecretDataValue(secret, azure.TenantIDKey, altTenantIDKey)
	if !ok {
		return nil, fmt.Errorf("secret %s/%s doesn't have a tenant ID", secret.Namespace, secret.Name)
	}

	clientID, ok := getSecretDataValue(secret, azure.ClientIDKey, altClientIDKey)
	if !ok {
		return nil, fmt.Errorf("secret %s/%s doesn't have a client ID", secret.Namespace, secret.Name)
	}

	clientSecret, ok := getSecretDataValue(secret, azure.ClientSecretKey, altClientSecretKey)
	if !ok {
		return nil, fmt.Errorf("secret %s/%s doesn't have a client secret", secret.Namespace, secret.Name)
	}

	return &ClientAuth{
		SubscriptionID: string(subscriptionID),
		TenantID:       string(tenantID),
		ClientID:       string(clientID),
		ClientSecret:   string(clientSecret),
	}, nil
}

// GetAuthorizerAndSubscriptionIDFromSecretRef retrieves the client auth data specified by the secret reference
// to create and return an Azure Authorizer and a subscription id.
func GetAuthorizerAndSubscriptionIDFromSecretRef(ctx context.Context, c client.Client, secretRef corev1.SecretReference, allowDNSKeys bool) (azureautorest.Authorizer, string, error) {
	clientAuth, err := GetClientAuthData(ctx, c, secretRef, allowDNSKeys)
	if err != nil {
		return nil, "", err
	}
	return GetAuthorizerAndSubscriptionID(clientAuth)
}

// GetAuthorizerAndSubscriptionID creates and returns an Azure Authorizer and a subscription id
func GetAuthorizerAndSubscriptionID(clientAuth *ClientAuth) (azureautorest.Authorizer, string, error) {
	clientCredentialsConfig := azureauth.NewClientCredentialsConfig(clientAuth.ClientID, clientAuth.ClientSecret, clientAuth.TenantID)
	authorizer, err := clientCredentialsConfig.Authorizer()
	if err != nil {
		return nil, "", err
	}

	return authorizer, clientAuth.SubscriptionID, nil
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
