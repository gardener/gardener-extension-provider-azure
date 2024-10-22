// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	azuretypes "github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

var (
	// DefaultBlobStorageClient is the default function to get a backupbucket client. Can be overridden for tests.
	DefaultBlobStorageClient = azureclient.NewBlobStorageClientFromSecret
)

type actuator struct {
	backupbucket.Actuator
	client client.Client
}

func newActuator(mgr manager.Manager) backupbucket.Actuator {
	return &actuator{
		client: mgr.GetClient(),
	}
}

func (a *actuator) Reconcile(ctx context.Context, _ logr.Logger, backupBucket *extensionsv1alpha1.BackupBucket) error {
	backupConfig, err := helper.BackupConfigFromBackupBucket(backupBucket)
	if err != nil {
		return err
	}

	azCloudConfiguration, err := azureclient.AzureCloudConfiguration(backupConfig.CloudConfiguration, &backupBucket.Spec.Region)
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

	bucketCloudConfiguration, err := azureclient.CloudConfiguration(backupConfig.CloudConfiguration, &backupBucket.Spec.Region)
	if err != nil {
		return err
	}

	storageDomain, err := azureclient.BlobStorageDomainFromCloudConfiguration(bucketCloudConfiguration)
	if err != nil {
		return fmt.Errorf("failed to determine blob storage service domain: %w", err)
	}

	// If the generated secret in the backupbucket status not exists that means
	// no backupbucket exists and it need to be created.
	if backupBucket.Status.GeneratedSecretRef == nil {
		storageAccountName, storageAccountKey, err := ensureBackupBucket(ctx, factory, backupBucket)
		if err != nil {
			return util.DetermineError(err, helper.KnownCodes)
		}
		// Create the generated backupbucket secret.
		if err := a.createBackupBucketGeneratedSecret(ctx, backupBucket, storageAccountName, storageAccountKey, storageDomain); err != nil {
			return util.DetermineError(err, helper.KnownCodes)
		}
	}

	backupSecret, err := a.getBackupBucketGeneratedSecret(ctx, backupBucket)
	if err != nil {
		return err
	}

	blobStorageClient, err := DefaultBlobStorageClient(ctx, backupSecret)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	doRotation, err := shouldBeRotated(*backupSecret)
	if err != nil {
		return err
	}

	if doRotation {
		// 1. Set annotation to signal that the secret is (possibly) outdated
		err = kutil.SetAnnotationAndUpdate(ctx, a.client, backupSecret, azuretypes.AnnotationSecretPossiblyOutdated, "true")
		if err != nil {
			return err
		}

		// 2. Rotate SA and get new key
		newKey, err := rotateStorageAccountCredentials(ctx, factory, backupBucket, string(backupSecret.Data[azuretypes.StorageKey]))
		if err != nil {
			return err
		}

		// 3. Generate new secret
		oldSecretRef := &corev1.SecretReference{
			Name:      backupBucket.Status.GeneratedSecretRef.Name,
			Namespace: backupBucket.Status.GeneratedSecretRef.Namespace,
		}
		if err := a.createBackupBucketGeneratedSecret(ctx, backupBucket, getStorageAccountName(backupBucket), newKey, storageDomain); err != nil {
			return util.DetermineError(err, helper.KnownCodes)
		}

		// 4. Clean up now outdated secret
		err = a.deleteBackupBucketGeneratedSecretByRef(ctx, oldSecretRef)
		if err != nil {
			return err
		}
	}

	return util.DetermineError(blobStorageClient.CreateContainerIfNotExists(ctx, backupBucket.Name), helper.KnownCodes)
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

	if secret != nil {
		// Get a storage account client to delete the backup container in the storage account.
		storageClient, err := DefaultBlobStorageClient(ctx, secret)
		if err != nil {
			return err
		}
		if err := storageClient.DeleteContainerIfExists(ctx, backupBucket.Name); err != nil {
			return err
		}
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
