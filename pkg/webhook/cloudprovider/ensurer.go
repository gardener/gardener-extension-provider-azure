// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package cloudprovider

import (
	"context"
	"errors"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	securityv1alpha1constants "github.com/gardener/gardener/pkg/apis/security/v1alpha1/constants"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

// NewEnsurer creates cloudprovider ensurer.
func NewEnsurer(_ manager.Manager, logger logr.Logger) cloudprovider.Ensurer {
	return &ensurer{
		logger: logger,
	}
}

type ensurer struct {
	logger logr.Logger
}

// EnsureCloudProviderSecret ensures that cloudprovider secret contains the required fields.
func (e *ensurer) EnsureCloudProviderSecret(_ context.Context, _ gcontext.GardenContext, newSecret, _ *corev1.Secret) error {
	if newSecret.Labels != nil && newSecret.Labels[securityv1alpha1constants.LabelWorkloadIdentityProvider] == "azure" {
		config, ok := newSecret.Data[securityv1alpha1constants.DataKeyConfig]
		if !ok {
			return errors.New("cloudprovider secret is missing a 'config' data key")
		}

		workloadIdentityConfig, err := helper.WorkloadIdentityConfigFromBytes(config)
		if err != nil {
			return fmt.Errorf("could not decode 'config' as WorkloadIdentityConfig: %w", err)
		}

		newSecret.Data[azure.ClientIDKey] = []byte(workloadIdentityConfig.ClientID)
		newSecret.Data[azure.TenantIDKey] = []byte(workloadIdentityConfig.TenantID)
		newSecret.Data[azure.SubscriptionIDKey] = []byte(workloadIdentityConfig.SubscriptionID)
		newSecret.Data[azure.WorkloadIdentityTokenFileKey] = []byte(azure.WorkloadIdentityMountPath + "/token")
		return nil
	}

	if !hasSecretKey(newSecret, azure.TenantIDKey) {
		return fmt.Errorf("could not mutate cloudprovider secret as %q field is missing", azure.TenantIDKey)
	}

	if !hasSecretKey(newSecret, azure.ClientIDKey) || !hasSecretKey(newSecret, azure.ClientSecretKey) {
		return fmt.Errorf("could not mutate cloudprovider secret as %q or %q field is missing", azure.ClientIDKey, azure.ClientSecretKey)
	}

	return nil
}

func hasSecretKey(secret *corev1.Secret, key string) bool {
	if _, ok := secret.Data[key]; ok {
		return true
	}
	return false
}
