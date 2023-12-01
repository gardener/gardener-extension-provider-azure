//  Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	azuretypes "github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	infrainternal "github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

var (
	// NewAzureClientFactory initializes a new AzureClientFactory. Exposed for testing.
	NewAzureClientFactory = newAzureClientFactory
)

func newAzureClientFactory(ctx context.Context, client client.Client, secretRef v1.SecretReference) (azureclient.Factory, error) {
	return azureclient.NewAzureClientFactory(ctx, client, secretRef)
}

func patchProviderStatusAndState(
	ctx context.Context,
	runtimeClient client.Client,
	infra *extensionsv1alpha1.Infrastructure,
	status *v1alpha1.InfrastructureStatus,
	state *runtime.RawExtension,
) error {
	modded := infra.DeepCopy()
	if status != nil {
		modded.Status.ProviderStatus = &runtime.RawExtension{Object: status}
	}
	if state != nil {
		modded.Status.State = state
	}

	return runtimeClient.Status().Patch(ctx, modded, client.MergeFrom(infra))
}

// CleanupTerraformerResources deletes terraformer artifacts (config, state, secrets).
func CleanupTerraformerResources(ctx context.Context, tf terraformer.Terraformer) error {
	if err := tf.EnsureCleanedUp(ctx); err != nil {
		return nil
	}
	if err := tf.CleanupConfiguration(ctx); err != nil {
		return err
	}
	return tf.RemoveTerraformerFinalizerFromConfig(ctx)
}

func hasFlowState(status extensionsv1alpha1.InfrastructureStatus) (bool, error) {
	if status.State == nil {
		return false, nil
	}

	flowState := runtime.TypeMeta{}
	stateJson, err := status.State.MarshalJSON()
	if err != nil {
		return false, err
	}

	if err := json.Unmarshal(stateJson, &flowState); err != nil {
		return false, err
	}

	if flowState.GroupVersionKind().GroupVersion() == v1alpha1.SchemeGroupVersion {
		return true, nil
	}

	infraState := &infrainternal.InfrastructureState{}
	if err := json.Unmarshal(status.State.Raw, infraState); err != nil {
		return false, err
	}

	if infraState.TerraformState != nil {
		return false, nil
	}

	return false, fmt.Errorf("unknown infrastructure state format")
}

// HasFlowAnnotation returns true if the new flow reconciler should be used for the reconciliation.
func HasFlowAnnotation(infrastructure *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) bool {
	if hasShootAnnotation(infrastructure, cluster, azuretypes.AnnotationKeyUseTF) {
		return false
	}

	if hasShootAnnotation(infrastructure, cluster, azuretypes.AnnotationKeyUseFlow) {
		return true
	}

	return cluster.Seed != nil && cluster.Seed.Annotations != nil && strings.EqualFold(cluster.Seed.Annotations[azuretypes.AnnotationKeyUseFlow], "true")
}

func hasShootAnnotation(infrastructure *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster, key string) bool {
	return (infrastructure.Annotations != nil && strings.EqualFold(infrastructure.Annotations[key], "true")) || (cluster.Shoot != nil && cluster.Shoot.Annotations != nil && strings.EqualFold(cluster.Shoot.Annotations[key], "true"))
}
