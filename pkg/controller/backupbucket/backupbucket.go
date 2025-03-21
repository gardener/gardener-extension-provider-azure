// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/go-logr/logr"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	azuretypes "github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

// GenerateStorageAccountName generates the name of the storage account from the bucket name <backupBucketName>.
func GenerateStorageAccountName(backupBucketName string) string {
	backupBucketNameSHA := utils.ComputeSHA256Hex([]byte(backupBucketName))
	return fmt.Sprintf("bkp%s", backupBucketNameSHA[:15])
}

// ensureResourceGroupAndStorageAccount ensures the existence of the necessary resourcegroup and storageacccount for the backupbucket
func ensureResourceGroupAndStorageAccount(
	ctx context.Context,
	factory azureclient.Factory,
	backupBucket *extensionsv1alpha1.BackupBucket,
	backupBucketConfig *azure.BackupBucketConfig,
) (string, string, error) {
	var (
		resourceGroupName  = backupBucket.Name
		storageAccountName = GenerateStorageAccountName(backupBucket.Name)
	)
	// Get resource group client to ensure resource group to host backup storage account exists.
	groupClient, err := factory.Group()
	if err != nil {
		return "", "", err
	}
	if _, err := groupClient.CreateOrUpdate(ctx, resourceGroupName, armresources.ResourceGroup{
		Location: to.Ptr(backupBucket.Spec.Region),
	}); err != nil {
		return "", "", err
	}

	// Get storage account client to create the backup storage account.
	storageAccountClient, err := factory.StorageAccount()
	if err != nil {
		return "", "", err
	}

	var keyExpirationDays *int32
	if backupBucketConfig != nil && backupBucketConfig.RotationConfig != nil && backupBucketConfig.RotationConfig.ExpirationPeriodDays != nil {
		keyExpirationDays = backupBucketConfig.RotationConfig.ExpirationPeriodDays
	}
	if err := storageAccountClient.CreateOrUpdateStorageAccount(ctx, resourceGroupName, storageAccountName, backupBucket.Spec.Region, keyExpirationDays); err != nil {
		return "", "", err
	}
	return resourceGroupName, storageAccountName, nil
}

func getMostRecentKey(
	ctx context.Context,
	factory azureclient.Factory,
	resourceGroupName, storageAccountName string,
) (*armstorage.AccountKey, error) {
	storageAccountClient, err := factory.StorageAccount()
	if err != nil {
		return nil, err
	}
	keys, err := storageAccountClient.ListStorageAccountKeys(ctx, resourceGroupName, storageAccountName)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(keys, func(a, b *armstorage.AccountKey) int {
		if a.CreationTime == nil {
			return 1
		}
		if b.CreationTime == nil {
			return -1
		}
		return a.CreationTime.Compare(*b.CreationTime)
	})

	return keys[0], nil
}

// ensureKeyRotated ensures that they storage account key is rotated if it is older than the expected age.
// In case of no operation to be performed, it returns the current key. It is required to return a value in case no
// error occurred.
func ensureKeyRotated(
	ctx context.Context,
	log logr.Logger,
	factory azureclient.Factory,
	resourceGroupName, storageAccountName, currentKey string,
	bb *extensionsv1alpha1.BackupBucket,
	bbCfg *azure.BackupBucketConfig,
) (*armstorage.AccountKey, bool, error) {
	// skip rotation if not configured
	if bbCfg.RotationConfig == nil || bbCfg.RotationConfig.RotationPeriodDays == 0 {
		return nil, false, nil
	}
	storageAccountClient, err := factory.StorageAccount()
	if err != nil {
		return nil, false, err
	}
	keys, err := storageAccountClient.ListStorageAccountKeys(ctx, resourceGroupName, storageAccountName)
	if err != nil {
		return nil, false, err
	}

	idx := slices.IndexFunc(keys, func(key *armstorage.AccountKey) bool {
		return ptr.Deref(key.Value, "") == currentKey
	})
	if idx == -1 {
		return nil, false, fmt.Errorf("key %s not found in storage account %s", currentKey, resourceGroupName)
	}
	otherKeyIdx := (idx + 1) % 2

	if keys[idx].CreationTime != nil &&
		time.Now().Before(keys[idx].CreationTime.Add(time.Hour*24*time.Duration(bbCfg.RotationConfig.RotationPeriodDays))) &&
		bb.GetAnnotations()[azuretypes.StorageAccountKeyMustRotate] != "true" {
		// not need to rotate
		return keys[idx], false, nil
	}

	log.Info("Rotating key", "name", *keys[otherKeyIdx].KeyName)
	newKey, err := storageAccountClient.RotateKey(ctx, resourceGroupName, storageAccountName, ptr.Deref(keys[otherKeyIdx].KeyName, ""))
	if err != nil {
		return nil, false, err
	}

	return newKey, true, nil
}
