// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"strconv"

	"github.com/gardener/gardener/extensions/pkg/terraformer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	azuretypes "github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

// CleanupTerraformerResources deletes terraformer artifacts (config, state, secrets).
func CleanupTerraformerResources(ctx context.Context, tf terraformer.Terraformer) error {
	if err := tf.EnsureCleanedUp(ctx); err != nil {
		return err
	}
	if err := tf.CleanupConfiguration(ctx); err != nil {
		return err
	}
	return tf.RemoveTerraformerFinalizerFromConfig(ctx)
}

// GetFlowAnnotationValue returns the boolean value of the expected flow annotation. Returns false if the annotation was not found, if it couldn't be converted to bool,
// or had a "false" value.
func GetFlowAnnotationValue(o metav1.Object) bool {
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
