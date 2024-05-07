// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	azuretypes "github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

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

	return false, nil
}

// HasFlowAnnotation returns true if the new flow reconciler should be used for the reconciliation.
func HasFlowAnnotation(infrastructure *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) bool {
	if ok := hasBoolAnnotation(infrastructure, azuretypes.GlobalAnnotationKeyUseFlow, azuretypes.AnnotationKeyUseFlow); ok != nil {
		return *ok
	}
	if shoot := cluster.Shoot; shoot != nil {
		if ok := hasBoolAnnotation(shoot, azuretypes.GlobalAnnotationKeyUseFlow, azuretypes.AnnotationKeyUseFlow); ok != nil {
			return *ok
		}
	}

	return false
}

func hasBoolAnnotation(o v1.Object, keys ...string) *bool {
	if annotations := o.GetAnnotations(); annotations != nil {
		for _, k := range keys {
			if v, ok := annotations[k]; ok {
				return ptr.To(strings.EqualFold(v, "true"))
			}
		}
	}

	return nil
}
