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
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2019-04-01/storage"
	"k8s.io/utils/pointer"
)

// CreateStorageAccount creates a storage account.
func (c StorageAccountClient) CreateStorageAccount(ctx context.Context, resourceGroupName, storageAccountName, region string) error {
	future, err := c.client.Create(ctx, resourceGroupName, storageAccountName, storage.AccountCreateParameters{
		Kind:     storage.BlobStorage,
		Location: &region,
		Sku: &storage.Sku{
			Name: storage.StandardLRS,
		},
		AccountPropertiesCreateParameters: &storage.AccountPropertiesCreateParameters{
			AccessTier:             storage.Cool,
			EnableHTTPSTrafficOnly: pointer.BoolPtr(true),
			AllowBlobPublicAccess:  pointer.BoolPtr(false),
			MinimumTLSVersion:      storage.TLS12,
		},
	})
	if err != nil {
		return err
	}

	return future.WaitForCompletionRef(ctx, c.client.Client)
}

// ListStorageAccountKey lists the first key of a storage account.
func (c StorageAccountClient) ListStorageAccountKey(ctx context.Context, resourceGroupName, storageAccountName string) (string, error) {
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
