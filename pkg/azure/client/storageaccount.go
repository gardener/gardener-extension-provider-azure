// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2019-04-01/storage"
	"k8s.io/utils/pointer"
)

var _ StorageAccount = &StorageAccountClient{}

// StorageAccountClient is an implementation of StorageAccount for storage account k8sClient.
type StorageAccountClient struct {
	client storage.AccountsClient
}

// CreateStorageAccount creates a storage account.
func (c *StorageAccountClient) CreateStorageAccount(ctx context.Context, resourceGroupName, storageAccountName, region string) error {
	future, err := c.client.Create(ctx, resourceGroupName, storageAccountName, storage.AccountCreateParameters{
		Kind:     storage.BlobStorage,
		Location: &region,
		Sku: &storage.Sku{
			Name: storage.StandardLRS,
		},
		AccountPropertiesCreateParameters: &storage.AccountPropertiesCreateParameters{
			AccessTier:             storage.Cool,
			EnableHTTPSTrafficOnly: pointer.Bool(true),
			AllowBlobPublicAccess:  pointer.Bool(false),
			MinimumTLSVersion:      storage.TLS12,
		},
	})
	if err != nil {
		return err
	}

	return future.WaitForCompletionRef(ctx, c.client.Client)
}

// ListStorageAccountKey lists the first key of a storage account.
func (c *StorageAccountClient) ListStorageAccountKey(ctx context.Context, resourceGroupName, storageAccountName string) (string, error) {
	response, err := c.client.ListKeys(ctx, resourceGroupName, storageAccountName, storage.Kerb)
	if err != nil {
		return "", err
	}

	if len(*response.Keys) < 1 {
		return "", fmt.Errorf("could not list keys as less then one key exists for storage account %s", storageAccountName)
	}

	firstKey := (*response.Keys)[0]
	return *firstKey.Value, nil
}
