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

package backupbucket

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"

	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

func ensureBackupBucket(ctx context.Context, factory azureclient.Factory, backupBucket *extensionsv1alpha1.BackupBucket) (string, string, error) {
	var (
		backupBucketNameSha = utils.ComputeSHA1Hex([]byte(backupBucket.Name))
		storageAccountName  = fmt.Sprintf("bkp%s", backupBucketNameSha[:15])
	)

	// Get resource group client to ensure resource group to host backup storage account exists.
	groupClient, err := factory.Group()
	if err != nil {
		return "", "", err
	}
	if _, err := groupClient.CreateOrUpdate(ctx, backupBucket.Name, armresources.ResourceGroup{
		Location: to.Ptr(backupBucket.Spec.Region),
	}); err != nil {
		return "", "", err
	}

	// Get storage account client to create the backup storage account.
	storageAccountClient, err := factory.StorageAccount()
	if err != nil {
		return "", "", err
	}
	if err := storageAccountClient.CreateStorageAccount(ctx, backupBucket.Name, storageAccountName, backupBucket.Spec.Region); err != nil {
		return "", "", err
	}

	// Get the key of the storage account.
	storageAccountKey, err := storageAccountClient.ListStorageAccountKey(ctx, backupBucket.Name, storageAccountName)
	if err != nil {
		return "", "", err
	}

	return storageAccountName, storageAccountKey, nil
}
