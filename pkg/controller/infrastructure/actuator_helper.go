// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
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

func newTerraformer(
	logger logr.Logger,
	restConfig *rest.Config,
	purpose string,
	infra *extensionsv1alpha1.Infrastructure,
	disableProjectedTokenMount bool,
) (
	terraformer.Terraformer,
	error,
) {
	tf, err := terraformer.NewForConfig(logger, restConfig, purpose, infra.Namespace, infra.Name, "")
	if err != nil {
		return nil, err
	}

	owner := metav1.NewControllerRef(infra, extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.InfrastructureResource))
	return tf.
		UseProjectedTokenMount(!disableProjectedTokenMount).
		SetTerminationGracePeriodSeconds(630).
		SetDeadlineCleaning(5 * time.Minute).
		SetDeadlinePod(15 * time.Minute).
		SetOwnerRef(owner), nil
}

// TerraformInfrastructureState represents the last known State of an Infrastructure resource.
// It is saved after a reconciliation and used during restore operations. This was used for terraform state and is only kept for backwards compatibility and allowing unmigrated shoots to
// deprecated: use FlowState instead.
// TODO: remove this in a future release
type TerraformInfrastructureState struct {
	// SavedProviderStatus contains the infrastructure's ProviderStatus.
	SavedProviderStatus *runtime.RawExtension `json:"savedProviderStatus,omitempty"`
	// TerraformState contains the state of the last applied terraform config.
	TerraformState *runtime.RawExtension `json:"terraformState,omitempty"`
	// // FlowState contains the state of the last applied Flow reconciliation.
	// FlowState *runtime.RawExtension `json:"flowState,omitempty"`
}

// ToRawExtension marshalls the struct and returns a runtime.RawExtension.
func (i *TerraformInfrastructureState) ToRawExtension() (*runtime.RawExtension, error) {
	j, err := json.Marshal(i)
	if err != nil {
		return nil, err
	}

	return &runtime.RawExtension{Raw: j}, nil
}
