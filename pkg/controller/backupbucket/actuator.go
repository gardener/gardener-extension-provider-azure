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
	"fmt"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	extensioncontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/backupbucket"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

func (a *actuator) Reconcile(ctx context.Context, bb *extensionsv1alpha1.BackupBucket) error {
	azureClient, err := a.getAzureClient(ctx, bb)
	if err != nil {
		return err
	}

	return azureClient.CreateContainerIfNotExists(ctx, bb.Name)
}

func (a *actuator) Delete(ctx context.Context, bb *extensionsv1alpha1.BackupBucket) error {
	azureClient, err := a.getAzureClient(ctx, bb)
	if err != nil {
		return err
	}

	if err := azureClient.DeleteContainerIfExists(ctx, bb.Name); err != nil {
		return err
	}

	return a.deleteGenerateBackupBucketSecret(ctx, bb)
}

func (a *actuator) getAzureClient(ctx context.Context, bb *extensionsv1alpha1.BackupBucket) (*azureclient.StorageClient, error) {
	if bb.Status.GeneratedSecretRef != nil {
		return azureclient.NewStorageClientFromSecretRef(ctx, a.client, bb.Status.GeneratedSecretRef)
	}
	backupBucketNameSha := utils.ComputeSHA1Hex([]byte(bb.Name))
	storageAccountName := fmt.Sprintf("bkp%s", backupBucketNameSha[:15])
	storageAuth, err := azureclient.NewStorageClientAuthFromSubscriptionSecretRef(ctx, a.client, &bb.Spec.SecretRef, bb.Name, storageAccountName, bb.Spec.Region)
	if err != nil {
		return nil, err
	}
	generatedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateGeneratedBackupBucketSecretName(bb.Name),
			Namespace: "garden",
		},
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, a.client, generatedSecret, func() error {
		generatedSecret.Data = map[string][]byte{
			azure.StorageAccount: storageAuth.StorageAccount,
			azure.StorageKey:     storageAuth.StorageKey,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if err := extensioncontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, a.client, bb, func() error {
		bb.Status.GeneratedSecretRef = &corev1.SecretReference{
			Name:      generatedSecret.Name,
			Namespace: generatedSecret.Namespace,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return azureclient.NewStorageClientFromStorageAuth(storageAuth)
}

func generateGeneratedBackupBucketSecretName(backupBucketName string) string {
	return fmt.Sprintf("generated-bucket-%s", backupBucketName)
}

// deleteGenerateBackupBucketSecret deletes generated secret referred by core BackupBucket resource in garden.
func (a *actuator) deleteGenerateBackupBucketSecret(ctx context.Context, bb *extensionsv1alpha1.BackupBucket) error {
	if bb.Status.GeneratedSecretRef != nil {
		if err := azureclient.DeleteResourceGroupFromSubscriptionSecretRef(ctx, a.client, &bb.Spec.SecretRef, bb.Name); err != nil {
			return err
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bb.Status.GeneratedSecretRef.Name,
				Namespace: bb.Status.GeneratedSecretRef.Namespace,
			},
		}
		return client.IgnoreNotFound(a.client.Delete(ctx, secret))
	}
	return nil
}
