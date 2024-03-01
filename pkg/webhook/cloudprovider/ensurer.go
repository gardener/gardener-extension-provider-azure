// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprovider

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

// NewEnsurer creates cloudprovider ensurer.
func NewEnsurer(mgr manager.Manager, logger logr.Logger) cloudprovider.Ensurer {
	return &ensurer{
		client: mgr.GetClient(),
		logger: logger,
	}
}

type ensurer struct {
	logger logr.Logger
	client client.Client
}

// EnsureCloudProviderSecret ensures that cloudprovider secret contain
// a service principal clientID and clientSecret (if not present) that match
// to a corresponding tenantID.
func (e *ensurer) EnsureCloudProviderSecret(ctx context.Context, _ gcontext.GardenContext, new, _ *corev1.Secret) error {
	if !hasSecretKey(new, azure.TenantIDKey) {
		return fmt.Errorf("could not mutate cloudprovider secret as %q field is missing", azure.TenantIDKey)
	}

	if hasSecretKey(new, azure.ClientIDKey) || hasSecretKey(new, azure.ClientSecretKey) {
		return nil
	}

	servicePrincipalSecret, err := e.fetchTenantServicePrincipalSecret(ctx, string(new.Data[azure.TenantIDKey]))
	if err != nil {
		return err
	}

	e.logger.V(5).Info("mutate cloudprovider secret", "namespace", new.Namespace, "name", new.Name)
	new.Data[azure.ClientIDKey] = servicePrincipalSecret.Data[azure.ClientIDKey]
	new.Data[azure.ClientSecretKey] = servicePrincipalSecret.Data[azure.ClientSecretKey]

	return nil
}

func (e *ensurer) fetchTenantServicePrincipalSecret(ctx context.Context, tenantID string) (*corev1.Secret, error) {
	var (
		servicePrincipalSecretList = &corev1.SecretList{}
		matchingSecrets            = []*corev1.Secret{}
		labelSelector              = client.MatchingLabels{azure.ExtensionPurposeLabel: azure.ExtensionPurposeServicePrincipalSecret}
	)

	if err := e.client.List(ctx, servicePrincipalSecretList, labelSelector); err != nil {
		return nil, err
	}

	for _, sec := range servicePrincipalSecretList.Items {
		if !hasSecretKey(&sec, azure.TenantIDKey) {
			e.logger.V(5).Info("service principal secret is invalid as it does not contain a tenant id", "namespace", sec.Namespace, "name", sec.Name)
			continue
		}

		if string(sec.Data[azure.TenantIDKey]) == tenantID {
			tmp := &sec
			matchingSecrets = append(matchingSecrets, tmp)
		}
	}

	if len(matchingSecrets) == 0 {
		return nil, fmt.Errorf("found no service principal secrets matching to tenant id %q", tenantID)
	}

	if len(matchingSecrets) > 1 {
		return nil, fmt.Errorf("found more than one service principal matching to tenant id %q", tenantID)
	}

	return matchingSecrets[0], nil
}

func hasSecretKey(secret *corev1.Secret, key string) bool {
	if _, ok := secret.Data[key]; ok {
		return true
	}
	return false
}
