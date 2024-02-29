// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

var (
	// DefaultClientFactoryFunc is the default function to get an azure client. Can be overridden for tests.
	DefaultClientFactoryFunc = azureclient.NewAzureClientFactory
	// DefaultBlobStorageClient is the default function to get a backupbucket client. Can be overridden for tests.
	DefaultBlobStorageClient = azureclient.NewBlobStorageClient
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
	factory, err := DefaultClientFactoryFunc(ctx, a.client, backupBucket.Spec.SecretRef)
	if err != nil {
		return err
	}

	// If the generated secret in the backupbucket status not exists that means
	// no backupbucket exists and it need to be created.
	if backupBucket.Status.GeneratedSecretRef == nil {
		storageAccountName, storageAccountKey, err := ensureBackupBucket(ctx, factory, backupBucket)
		if err != nil {
			return util.DetermineError(err, helper.KnownCodes)
		}
		// Create the generated backupbucket secret.
		if err := a.createBackupBucketGeneratedSecret(ctx, backupBucket, storageAccountName, storageAccountKey); err != nil {
			return util.DetermineError(err, helper.KnownCodes)
		}
	}

	storageClient, err := DefaultBlobStorageClient(ctx, a.client, *backupBucket.Status.GeneratedSecretRef)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}
	return util.DetermineError(storageClient.CreateContainerIfNotExists(ctx, backupBucket.Name), helper.KnownCodes)
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
	if secret != nil {
		// Get a storage account client to delete the backup container in the storage account.
		storageClient, err := DefaultBlobStorageClient(ctx, a.client, *backupBucket.Status.GeneratedSecretRef)
		if err != nil {
			return err
		}
		if err := storageClient.DeleteContainerIfExists(ctx, backupBucket.Name); err != nil {
			return err
		}
	}

	factory, err := DefaultClientFactoryFunc(ctx, a.client, backupBucket.Spec.SecretRef)
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
