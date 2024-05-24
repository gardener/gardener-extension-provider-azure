// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

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

// GetFlowAnnotationValue returns the boolean value of the expected flow annotation. Returns false if the annotation was not found, if it couldn't be converted to bool,
// or had a "false" value.
func GetFlowAnnotationValue(o v1.Object) bool {
	if annotations := o.GetAnnotations(); annotations != nil {
		for _, k := range azuretypes.ValidFlowAnnotations {
			if str, ok := annotations[k]; ok {
				if v, err := strconv.ParseBool(str); err != nil {
					return false
				} else {
					return v
				}
			}
		}
	}
	return false
}
