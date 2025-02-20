// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// DefaultAzureClientFactoryFunc is the default function for creating a DNS client. It can be overridden for tests.
var DefaultAzureClientFactoryFunc = azureclient.NewAzureClientFactoryFromSecret

type actuator struct {
	backupbucket.Actuator
	client client.Client
}

func NewActuator(mgr manager.Manager) backupbucket.Actuator {
	return &actuator{
		client: mgr.GetClient(),
	}
}

func (a *actuator) Reconcile(ctx context.Context, logger logr.Logger, backupBucket *extensionsv1alpha1.BackupBucket) error {
	backupBucketConfig, err := helper.BackupConfigFromBackupBucket(backupBucket)
	if err != nil {
		logger.Error(err, "failed to decode the provider specific configuration from the backupbucket resource")
		return err
	}

	azCloudConfiguration, err := azureclient.AzureCloudConfiguration(backupBucketConfig.CloudConfiguration, &backupBucket.Spec.Region)
	if err != nil {
		return err
	}

	factory, err := DefaultAzureClientFactoryFunc(
		ctx,
		a.client,
		backupBucket.Spec.SecretRef,
		false,
		azureclient.WithCloudConfiguration(azCloudConfiguration),
	)
	if err != nil {
		return err
	}

	var (
		resourceGroupName  = backupBucket.Name // current implementation uses the same name for resourceGroup and backupBucket
		storageAccountName = GenerateStorageAccountName(backupBucket.Name)
	)
	// If the generated secret in the backupbucket status does not exist
	// it means no backupbucket exists and it needs to be created.
	if backupBucket.Status.GeneratedSecretRef == nil {
		storageAccountKey, err := ensureResourceGroupAndStorageAccount(ctx, factory, backupBucket)
		if err != nil {
			logger.Error(err, "Failed to ensure the resource group and storage account")
			return util.DetermineError(err, helper.KnownCodes)
		}

		bucketCloudConfiguration, err := azureclient.CloudConfiguration(backupBucketConfig.CloudConfiguration, &backupBucket.Spec.Region)
		if err != nil {
			logger.Error(err, "Failed to determine cloud configuration")
			return err
		}

		storageDomain, err := azureclient.BlobStorageDomainFromCloudConfiguration(bucketCloudConfiguration)
		if err != nil {
			logger.Error(err, "Failed to determine blob storage service domain")
			return fmt.Errorf("failed to determine blob storage service domain: %w", err)
		}
		// Create the generated backupbucket secret.
		if err := a.createBackupBucketGeneratedSecret(ctx, backupBucket, storageAccountName, storageAccountKey, storageDomain); err != nil {
			logger.Error(err, "Failed to generate the backupbucket secret")
			return util.DetermineError(err, helper.KnownCodes)
		}
	}

	blobContainersClient, err := factory.BlobContainers()
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	// the resourcegroup is of the same name as the bucket
	if _, err = blobContainersClient.GetContainer(ctx, resourceGroupName, storageAccountName, backupBucket.Name); err != nil && !azureclient.IsAzureAPINotFoundError(err) {
		logger.Error(err, "Errored while fetching information", "bucket", backupBucket.Name)
		return util.DetermineError(err, helper.KnownCodes)
	}

	// create the bucket if it does not exist
	if azureclient.IsAzureAPINotFoundError(err) {
		logger.Info("Bucket does not exist; creating", "name", backupBucket.Name)
		_, err = blobContainersClient.CreateContainer(ctx, resourceGroupName, storageAccountName, backupBucket.Name)
		if err != nil {
			logger.Error(err, "Errored while creating the container", "bucket", backupBucket.Name)
			return err
		}
	}

	// update the bucket if necessary
	currentContainerImmutabilityDays, currentlyLocked, etag, err := blobContainersClient.GetImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name)
	if err != nil {
		logger.Error(err, "Errored while fetching immutability information", "bucket", backupBucket.Name)
		return util.DetermineError(err, helper.KnownCodes)
	}
	if err = updateBackupBucketIfNeeded(
		ctx, logger,
		blobContainersClient, backupBucketConfig,
		resourceGroupName, storageAccountName, backupBucket.Name,
		currentContainerImmutabilityDays, currentlyLocked, etag,
	); err != nil {
		logger.Error(err, "Errored while updating the bucket")
		return util.DetermineError(err, helper.KnownCodes)
	}

	// lock the policy if configured
	if backupBucketConfig.Immutability != nil && backupBucketConfig.Immutability.Locked && !currentlyLocked {
		logger.Info("Locking bucket immutability policy")
		// Etag of the resource will change if the policy was updated in the same reconiliation
		// get the bucket policy again and use that etag
		_, _, etag, err = blobContainersClient.GetImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name)
		if err != nil {
			logger.Error(err, "Errored while fetching immutability information again before locking", "bucket", backupBucket.Name)
			return util.DetermineError(err, helper.KnownCodes)
		}
		err = blobContainersClient.LockImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name, etag)
		if err != nil {
			logger.Error(err, "Errored while locking the immutability policy of the bucket")
			return util.DetermineError(err, helper.KnownCodes)
		}
	}

	return nil
}

func updateBackupBucketIfNeeded(
	ctx context.Context, logger logr.Logger,
	blobContainersClient azureclient.BlobContainers,
	backupBucketConfig azure.BackupBucketConfig,
	resourceGroupName, storageAccountName, backupBucketName string,
	currentContainerImmutabilityDays *int32, currentlyLocked bool, etag *string,
) error {
	var currentDays, desiredDays int32
	if currentContainerImmutabilityDays != nil {
		currentDays = *currentContainerImmutabilityDays
	}
	if backupBucketConfig.Immutability != nil {
		desiredDays = int32(backupBucketConfig.Immutability.RetentionPeriod.Duration.Hours() / 24)
	}

	// Extend policy if necessary
	if currentlyLocked {
		if desiredDays > currentDays {
			logger.Info("Extending bucket immutability period", "new period days", desiredDays)
			return blobContainersClient.ExtendImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucketName, &desiredDays, etag)
		}
		return nil
	}

	// Delete the policy if requested
	if currentDays != 0 && desiredDays == 0 {
		logger.Info("Deleting the bucket immutability policy")
		return blobContainersClient.DeleteImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucketName, etag)
	}

	// Update the policy if requested
	if currentDays != desiredDays {
		logger.Info("Updating the bucket immutability policy", "new period days", desiredDays)
		return blobContainersClient.CreateOrUpdateImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucketName, &desiredDays)
	}
	return nil
}

func (a *actuator) Delete(ctx context.Context, logger logr.Logger, backupBucket *extensionsv1alpha1.BackupBucket) error {
	return util.DetermineError(a.delete(ctx, logger, backupBucket), helper.KnownCodes)
}

func (a *actuator) delete(ctx context.Context, _ logr.Logger, backupBucket *extensionsv1alpha1.BackupBucket) error {
	// If the backupBucket has no generated secret in the status that means
	// no backupbucket exists and therefore there is no need for deletion.
	if backupBucket.Status.GeneratedSecretRef == nil {
		return nil
	}

	secret, err := a.getBackupBucketGeneratedSecret(ctx, backupBucket)
	if err != nil {
		return err
	}

	backupBucketConfig, err := helper.BackupConfigFromBackupBucket(backupBucket)
	if err != nil {
		return err
	}

	var (
		cloudConfiguration *azure.CloudConfiguration
		region             *string
	)

	if backupBucket != nil {
		cloudConfiguration = backupBucketConfig.CloudConfiguration
		region = &backupBucket.Spec.Region
	}

	cloudConfiguration, err = azureclient.CloudConfiguration(cloudConfiguration, region)
	if err != nil {
		return err
	}

	azCloudConfiguration, err := azureclient.AzureCloudConfigurationFromCloudConfiguration(cloudConfiguration)
	if err != nil {
		return err
	}

	factory, err := DefaultAzureClientFactoryFunc(
		ctx,
		a.client,
		backupBucket.Spec.SecretRef,
		false,
		azureclient.WithCloudConfiguration(azCloudConfiguration),
	)
	if err != nil {
		return err
	}

	if secret != nil {
		// Get a storage account client to delete the backup bucket in the storage account.
		blobContainersClient, err := factory.BlobContainers()
		if err != nil {
			return err
		}
		storageAccountName := GenerateStorageAccountName(backupBucket.Name)
		// resourceGroupName and backupBucketName are identical
		if err := blobContainersClient.DeleteContainer(ctx, backupBucket.Name, storageAccountName, backupBucket.Name); err != nil {
			return err
		}
	}

	// Get resource group client and delete the resource group which contains the backup storage account.
	groupClient, err := factory.Group()
	if err != nil {
		return err
	}
	if err := groupClient.Delete(ctx, backupBucket.Name); err != nil {
		return err
	}

	// Delete the generated backup secret in the garden namespace.
	return a.deleteBackupBucketGeneratedSecret(ctx, backupBucket)
}
