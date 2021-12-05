// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"

	"github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type actuator struct {
	backupbucket.Actuator
	client client.Client
	logger logr.Logger
}

func newActuator() backupbucket.Actuator {
	return &actuator{
		logger: log.Log.WithName("azure-backupbucket-actuator"),
	}
}

func (a *actuator) InjectClient(client client.Client) error {
	a.client = client
	return nil
}

func (a *actuator) Reconcile(ctx context.Context, backupBucket *extensionsv1alpha1.BackupBucket) error {
	var factory = azureclient.NewAzureClientFactory(a.client)

	// If the generated secret in the backupbucket status not exists that means
	// no backupbucket exists and it need to be created.
	if backupBucket.Status.GeneratedSecretRef == nil {
		storageAccountName, storageAccountKey, err := ensureBackupBucket(ctx, a.client, factory, backupBucket)
		if err != nil {
			return err
		}
		// Create the generated backupbucket secret.
		if err := a.createBackupBucketGeneratedSecret(ctx, backupBucket, storageAccountName, storageAccountKey); err != nil {
			return err
		}
	}

	storageClient, err := factory.Storage(ctx, *backupBucket.Status.GeneratedSecretRef)
	if err != nil {
		return err
	}
	return storageClient.CreateContainerIfNotExists(ctx, backupBucket.Name)
}

func (a *actuator) Delete(ctx context.Context, backupBucket *extensionsv1alpha1.BackupBucket) error {
	// If the backupBucket has no generated secret in the status that means
	// no backupbucket exists and therefore there is no need for deletion.
	if backupBucket.Status.GeneratedSecretRef == nil {
		return nil
	}

	var factory = azureclient.NewAzureClientFactory(a.client)

	// Get a storage account client to delete the backup container in the storage account.
	storageClient, err := factory.Storage(ctx, *backupBucket.Status.GeneratedSecretRef)
	if err != nil {
		return err
	}
	if err := storageClient.DeleteContainerIfExists(ctx, backupBucket.Name); err != nil {
		return err
	}

	// Get resource group client and delete the resource group which contains the backup storage account.
	groupClient, err := factory.Group(ctx, backupBucket.Spec.SecretRef)
	if err != nil {
		return err
	}
	if err := groupClient.DeleteIfExits(ctx, backupBucket.Name); err != nil {
		return err
	}

	// Delete the generated backup secret in the garden namespace.
	return a.deleteBackupBucketGeneratedSecret(ctx, backupBucket)
}
