// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"
	"fmt"
	"reflect"

	"github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/go-logr/logr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

// var (
// 	// DefaultBlobStorageClient is the default function to get a backupbucket client. Can be overridden for tests.
// 	DefaultBlobStorageClient = azureclient.NewBlobStorageClientFromSecretRef
// )

type actuator struct {
	backupbucket.Actuator
	client client.Client
}

func newActuator(mgr manager.Manager) backupbucket.Actuator {
	return &actuator{
		client: mgr.GetClient(),
	}
}

func (a *actuator) Reconcile(ctx context.Context, logger logr.Logger, backupBucket *extensionsv1alpha1.BackupBucket) error {
	logger.Info("Starting reconciliation for the backupbucket")
	backupBucketConfig, err := helper.BackupConfigFromBackupBucket(backupBucket)
	if err != nil {
		logger.Error(err, "failed to decode the provider specific configuration from the backupbucket resource")
		return err
	}

	azCloudConfiguration, err := azureclient.AzureCloudConfiguration(backupBucketConfig.CloudConfiguration, &backupBucket.Spec.Region)
	if err != nil {
		return err
	}

	factory, err := azureclient.NewAzureClientFactoryFromSecret(
		ctx,
		a.client,
		backupBucket.Spec.SecretRef,
		false,
		azureclient.WithCloudConfiguration(azCloudConfiguration),
	)
	if err != nil {
		return err
	}

	// If the generated secret in the backupbucket status does not exist
	// it means no backupbucket exists and it needs to be created.
	var storageAccountName string
	if backupBucket.Status.GeneratedSecretRef == nil {
		var storageAccountKey string
		storageAccountName, storageAccountKey, err = ensureResourceGroupAndStorageAccount(ctx, factory, backupBucket)
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

	// the resourcegroup is of the same name as the container
	if _, err = blobContainersClient.GetContainer(ctx, backupBucket.Name, storageAccountName, backupBucket.Name); !azureclient.IsAzureAPINotFoundError(err) {
		logger.Error(err, "Errored while fetching information", "container", backupBucket.Name)
		return util.DetermineError(err, helper.KnownCodes)
	}

	// container does not exist, create the container
	if azureclient.IsAzureAPINotFoundError(err) {
		logger.Info("Container does not exist; creating", "name", backupBucket.Name)
		blobContainersClient.CreateContainer(ctx, backupBucket.Name, storageAccountName, backupBucket.Name)
	}

	// update the bucket if necessary
	currentContainerImmutabilityDays, currentlyLocked, err := blobContainersClient.GetImmutabilityPolicy(ctx, backupBucket.Name, storageAccountName, backupBucket.Name)
	if err != nil {
		logger.Error(err, "Errored while fetching immutability information", "container", backupBucket.Name)
		return util.DetermineError(err, helper.KnownCodes)
	}
	if err = updateBackupBucketIfNeeded(
		ctx, logger,
		blobContainersClient, backupBucketConfig,
		storageAccountName, backupBucket.Name,
		currentContainerImmutabilityDays, currentlyLocked,
	); err != nil {
		logger.Error(err, "Errored while updating the container")
		return util.DetermineError(err, helper.KnownCodes)
	}

	// lock the policy if configured
	if backupBucketConfig.Immutability != nil && !currentlyLocked && backupBucketConfig.Immutability.Locked {
		err = blobContainersClient.LockImmutabilityPolicy(ctx, backupBucket.Name, storageAccountName, backupBucket.Name)
		if err != nil {
			logger.Error(err, "Errored while locking the immutability policy of the container")
			return util.DetermineError(err, helper.KnownCodes)
		}
	}

	logger.Info("Reconciled successfully", "container", backupBucket.Name)
	return nil
}

// There are three cases for unlocked
//  1. Create an immutability policy. currentContainerImmutabilityDays must be nil or 0, desiredContainerImmutabilityDays must be non-nil
//  2. Update an immutability policy. currentContainerImmutabilityDays must be non-nil, desiredContainerImmutabilityDays must be non-nil
//  3. Delete an immutability policy. currentContainerImmutabilityDays must be non-nil, desiredContainerImmutabilityDays must be nil.
//
// There is one case for locked
//  1. Extend an immutability policy. currentContainerImmutabilityDays must be non-nil, desiredContainerImmutabilityDays must be non-nil and greater than current
func updateBackupBucketIfNeeded(
	ctx context.Context, logger logr.Logger,
	blobContainersClient azureclient.BlobContainers,
	backupBucketConfig azure.BackupBucketConfig,
	storageAccountName, backupBucketName string,
	currentContainerImmutabilityDays *int32, currentlyLocked bool,
) error {
	var desiredContainerImmutabilityDays *int32
	if backupBucketConfig.Immutability != nil {
		desiredContainerImmutabilityDays = ptr.To(int32(backupBucketConfig.Immutability.RetentionPeriod.Duration.Hours()) / 24)
	}

	// Extend policy if requested
	if currentlyLocked {
		if desiredContainerImmutabilityDays != nil &&
			currentContainerImmutabilityDays != nil &&
			*desiredContainerImmutabilityDays > *currentContainerImmutabilityDays {
			logger.Info("Extending container immutability period", "period", *desiredContainerImmutabilityDays)
			return blobContainersClient.ExtendImmutabilityPolicy(ctx, backupBucketName, storageAccountName, backupBucketName, desiredContainerImmutabilityDays)
		}
		return nil
	}

	// Delete the policy if requested
	if currentContainerImmutabilityDays != nil && desiredContainerImmutabilityDays == nil {
		logger.Info("Deleting the container immutability policy")
		return blobContainersClient.DeleteImmutabilityPolicy(ctx, backupBucketName, storageAccountName, backupBucketName)
	}

	// Update the policy if requested
	if !reflect.DeepEqual(currentContainerImmutabilityDays, desiredContainerImmutabilityDays) {
		logger.Info("Updating the container immutability policy")
		return blobContainersClient.CreateOrUpdateImmutabilityPolicy(ctx, backupBucketName, storageAccountName, backupBucketName, desiredContainerImmutabilityDays)
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

	factory, err := azureclient.NewAzureClientFactoryFromSecret(
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
		// Get a storage account client to delete the backup container in the storage account.
		blobContainersClient, err := factory.BlobContainers()
		if err != nil {
			return err
		}
		storageAccountName := fmt.Sprintf("bkp%s", utils.ComputeSHA256Hex([]byte(backupBucket.Name))[:15])
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
