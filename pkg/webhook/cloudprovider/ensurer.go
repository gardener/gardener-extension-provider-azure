// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
