// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package client

import (
	"context"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"

	azureresources "github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	azurestorage "github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2019-04-01/storage"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewAzureClientFactory returns a new factory to produce clients for various Azure services.
func NewAzureClientFactory(client client.Client) Factory {
	return AzureFactory{
		client: client,
	}
}

// Group reads the secret from the passed reference and return an Azure resource group client.
func (f AzureFactory) Group(ctx context.Context, secretRef corev1.SecretReference) (Group, error) {
	authorizer, subscriptionID, err := internal.GetAuthorizerAndSubscriptionID(ctx, f.client, secretRef)
	if err != nil {
		return nil, err
	}
	groupClient := azureresources.NewGroupsClient(subscriptionID)
	groupClient.Authorizer = authorizer

	return GroupClient{
		client: groupClient,
	}, nil
}

// Storage reads the secret from the passed reference and return an Azure (blob) storage client.
func (f AzureFactory) Storage(ctx context.Context, secretRef corev1.SecretReference) (Storage, error) {
	serviceURL, err := newStorageClient(ctx, f.client, &secretRef)
	if err != nil {
		return nil, err
	}

	return StorageClient{
		serviceURL: serviceURL,
	}, nil
}

// StorageAccount reads the secret from the passed reference and return an Azure storage account client.
func (f AzureFactory) StorageAccount(ctx context.Context, secretRef corev1.SecretReference) (StorageAccount, error) {
	authorizer, subscriptionID, err := internal.GetAuthorizerAndSubscriptionID(ctx, f.client, secretRef)
	if err != nil {
		return nil, err
	}
	storageAccountClient := azurestorage.NewAccountsClient(subscriptionID)
	storageAccountClient.Authorizer = authorizer

	return StorageAccountClient{
		client: storageAccountClient,
	}, nil
}
