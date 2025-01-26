// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprovider

import (
	"context"
	"errors"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/util"
	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	securityv1alpha1constants "github.com/gardener/gardener/pkg/apis/security/v1alpha1/constants"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	apiazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

// NewEnsurer creates cloudprovider ensurer.
func NewEnsurer(mgr manager.Manager, logger logr.Logger) cloudprovider.Ensurer {
	return &ensurer{
		client:  mgr.GetClient(),
		logger:  logger,
		decoder: serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
	}
}

type ensurer struct {
	logger  logr.Logger
	client  client.Client
	decoder runtime.Decoder
}

// EnsureCloudProviderSecret ensures that cloudprovider secret contain
// a service principal clientID and clientSecret (if not present) that match
// to a corresponding tenantID.
func (e *ensurer) EnsureCloudProviderSecret(ctx context.Context, _ gcontext.GardenContext, newSecret, _ *corev1.Secret) error {
	if newSecret.ObjectMeta.Labels != nil && newSecret.ObjectMeta.Labels[securityv1alpha1constants.LabelWorkloadIdentityProvider] == "azure" {
		if _, ok := newSecret.Data[securityv1alpha1constants.DataKeyConfig]; !ok {
			return errors.New("cloudprovider secret is missing a 'config' data key")
		}
		workloadIdentityConfig := &apiazure.WorkloadIdentityConfig{}
		if err := util.Decode(e.decoder, newSecret.Data[securityv1alpha1constants.DataKeyConfig], workloadIdentityConfig); err != nil {
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

	if hasSecretKey(newSecret, azure.ClientIDKey) || hasSecretKey(newSecret, azure.ClientSecretKey) {
		return nil
	}

	servicePrincipalSecret, err := e.fetchTenantServicePrincipalSecret(ctx, string(newSecret.Data[azure.TenantIDKey]))
	if err != nil {
		return err
	}

	e.logger.V(5).Info("mutate cloudprovider secret", "namespace", newSecret.Namespace, "name", newSecret.Name)
	newSecret.Data[azure.ClientIDKey] = servicePrincipalSecret.Data[azure.ClientIDKey]
	newSecret.Data[azure.ClientSecretKey] = servicePrincipalSecret.Data[azure.ClientSecretKey]

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
