// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry

import (
	"context"
	"fmt"
	"strings"

	"github.com/gardener/gardener/extensions/pkg/controller/backupentry/genericactuator"
	"github.com/gardener/gardener/extensions/pkg/util"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

var (
	// DefaultBlobStorageClient is the default function to get a backupbucket client. Can be overridden for tests.
	DefaultBlobStorageClient = azureclient.NewBlobStorageClientFromSecretRef
)

type actuator struct {
	client client.Client
}

func newActuator(mgr manager.Manager) genericactuator.BackupEntryDelegate {
	return &actuator{
		client: mgr.GetClient(),
	}
}

func (a *actuator) GetETCDSecretData(_ context.Context, _ logr.Logger, _ *extensionsv1alpha1.BackupEntry, backupSecretData map[string][]byte) (map[string][]byte, error) {
	return backupSecretData, nil
}

func (a *actuator) Delete(ctx context.Context, _ logr.Logger, backupEntry *extensionsv1alpha1.BackupEntry) error {
	storageClient, err := DefaultBlobStorageClient(ctx, a.client, &backupEntry.Spec.SecretRef, backupEntry.Spec.BucketName)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}
	entryName := strings.TrimPrefix(backupEntry.Name, v1beta1constants.BackupSourcePrefix+"-")
	return util.DetermineError(storageClient.CleanupObjectsWithPrefix(ctx, fmt.Sprintf("%s/", entryName)), helper.KnownCodes)
}
