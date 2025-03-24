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
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	if backupBucketConfig != nil && backupBucketConfig.RotationConfig != nil {
		keyExpirationDays = backupBucketConfig.RotationConfig.ExpirationPeriodDays
	}
	if err := storageAccountClient.CreateOrUpdateStorageAccount(ctx, resourceGroupName, storageAccountName, backupBucket.Spec.Region, keyExpirationDays); err != nil {
		return "", "", err
	}
	return resourceGroupName, storageAccountName, nil
}
func SortKeysByAge(
	keys []*armstorage.AccountKey,
) []*armstorage.AccountKey {
	slices.SortFunc(keys, func(a, b *armstorage.AccountKey) int {
		if a.CreationTime == nil {
			return 1
		}
		if b.CreationTime == nil {
			return -1
		}
		return b.CreationTime.Compare(*a.CreationTime)
	})

	return keys
}

// ensureKeyRotated ensures that they storage account key is rotated if it is older than the expected age.
// In case of no operation to be performed, it returns the current key. It is required to return a value in case no
// error occurred.
func (a *actuator) ensureKeyRotated(
	ctx context.Context,
	log logr.Logger,
	storageAccount azureclient.StorageAccount,
	resourceGroupName, storageAccountName string,
	currentKeys []*armstorage.AccountKey,
	backupBucket *extensionsv1alpha1.BackupBucket,
	backupBucketConfig *azure.BackupBucketConfig,
) ([]*armstorage.AccountKey, error) {
	if backupBucketConfig.RotationConfig == nil {
		log.Info("Skipping rotation because it's not configured for the backup bucket")
		return currentKeys, nil
	}
	// we should never enter this condition as it is protected by the admission controller. Still we assert our invariants.
	if backupBucketConfig.RotationConfig.RotationPeriodDays == 0 {
		log.Error(nil, "backup bucket rotation config is required")
		return currentKeys, nil
	}
	mostRecentKey := SortKeysByAge(currentKeys)[0]

	shouldRotateByAge := mostRecentKey.CreationTime == nil ||
		time.Now().After(mostRecentKey.CreationTime.Add(time.Hour*24*time.Duration(backupBucketConfig.RotationConfig.RotationPeriodDays)))
	shouldRotateByAnnotation := backupBucket.GetAnnotations()[azuretypes.StorageAccountKeyMustRotate] == "true"

	if !shouldRotateByAge && !shouldRotateByAnnotation {
		// no need to rotate
		return currentKeys, nil
	}

	reasonForRotation := "rotation due to age"
	if shouldRotateByAnnotation {
		reasonForRotation = "rotation due to annotation"
		secret, err := a.getBackupBucketGeneratedSecret(ctx, backupBucket)
		if err != nil {
			return nil, err
		}
		if v, ok := secret.Data[azuretypes.StorageKey]; ok {
			// The key in the secret can either be the most recent, the oldest, or not found in the list of keys from Azure API.
			// Unless the secret value matches the most recent key, we can assume that the rotation already happened but the secret was not properly
			// updated. Hence, we no-op and return the current most recent key.
			if string(v) != *mostRecentKey.Value {
				log.Info("Skipping rotation because backup bucket current key does not match storage account keys. Using most recent rotation key instead.")
				return currentKeys, nil
			}
		}
	}

	log.Info("Rotating key", "name", *mostRecentKey.KeyName, "reasonForRotation", reasonForRotation)
	keys, err := storageAccount.RotateKey(ctx, resourceGroupName, storageAccountName, *currentKeys[1].KeyName)
	if err != nil {
		return nil, err
	}

	if shouldRotateByAnnotation {
		log.Info("removing rotation annotation")
		backupBucketPatch := client.MergeFrom(backupBucket.DeepCopy())
		delete(backupBucket.GetAnnotations(), azuretypes.StorageAccountKeyMustRotate)
		if err := a.client.Patch(ctx, backupBucket, backupBucketPatch); err != nil {
			return nil, fmt.Errorf("failed to remove rotation annotation: %w", err)
		}
	}
	return keys, nil
}
