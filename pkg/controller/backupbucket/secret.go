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

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

func (a *actuator) createBackupBucketGeneratedSecret(ctx context.Context, backupBucket *extensionsv1alpha1.BackupBucket, storageAccountName, storageKey string) error {
	var generatedSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("generated-bucket-%s", backupBucket.Name),
			Namespace: "garden",
		},
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, a.client, generatedSecret, func() error {
		generatedSecret.Data = map[string][]byte{
			azure.StorageAccount: []byte(storageAccountName),
			azure.StorageKey:     []byte(storageKey),
		}
		return nil
	}); err != nil {
		return err
	}

	patch := client.MergeFrom(backupBucket.DeepCopy())
	backupBucket.Status.GeneratedSecretRef = &corev1.SecretReference{
		Name:      generatedSecret.Name,
		Namespace: generatedSecret.Namespace,
	}
	return a.client.Status().Patch(ctx, backupBucket, patch)
}

// deleteBackupBucketGeneratedSecret deletes generated secret referred by core BackupBucket resource in garden.
func (a *actuator) deleteBackupBucketGeneratedSecret(ctx context.Context, backupBucket *extensionsv1alpha1.BackupBucket) error {
	if backupBucket.Status.GeneratedSecretRef == nil {
		return nil
	}
	return kutil.DeleteSecretByReference(ctx, a.client, backupBucket.Status.GeneratedSecretRef)
}

// getBackupBucketGeneratedSecret get generated secret referred by core BackupBucket resource in garden.
func (a *actuator) getBackupBucketGeneratedSecret(ctx context.Context, backupBucket *extensionsv1alpha1.BackupBucket) (*corev1.Secret, error) {
	if backupBucket.Status.GeneratedSecretRef == nil {
		return nil, nil
	}
	secret, err := kutil.GetSecretByReference(ctx, a.client, backupBucket.Status.GeneratedSecretRef)
	if err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return secret, nil
}
