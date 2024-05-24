//  SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
//  SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/go-logr/logr"
)

// Reconciler is an interface for the infrastructure reconciliation.
type Reconciler interface {
	// Reconcile manages infrastructure resources according to desired spec.
	Reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error
	// Delete removes any created infrastructure resource on the provider.
	Delete(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error
	// Restore restores the infrastructure after a control plane migration. Effectively it performs a recovery of data from the infrastructure.status.state and
	// proceeds to reconcile.
	Restore(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error
}

// ReconcilerFactory can construct the different infrastructure reconciler implementations.
type ReconcilerFactory interface {
	Build(useFlow bool) (Reconciler, error)
}

// ReconcilerFactoryImpl is an implementation of a ReconcilerFactory
type ReconcilerFactoryImpl struct {
	ctx   context.Context
	log   logr.Logger
	a     *actuator
	infra *extensionsv1alpha1.Infrastructure
}

// Build builds the Reconciler according to the arguments.
func (f ReconcilerFactoryImpl) Build(useFlow bool) (Reconciler, error) {
	if useFlow {
		reconciler, err := NewFlowReconciler(f.a, f.log, f.a.disableProjectedTokenMount)
		if err != nil {
			return nil, fmt.Errorf("failed to init flow reconciler: %w", err)
		}
		return reconciler, nil
	}

	reconciler, err := NewTerraformReconciler(f.a, f.log, f.a.restConfig, f.a.disableProjectedTokenMount)
	if err != nil {
		return nil, fmt.Errorf("failed to init terraform reconciler: %w", err)
	}
	return reconciler, nil
}

// SelectorFunc decides the reconciler used.
type SelectorFunc func(*extensionsv1alpha1.Infrastructure, *extensions.Cluster) (bool, error)

// OnReconcile returns true if the operation should use the Flow for the given cluster.
func OnReconcile(infra *extensionsv1alpha1.Infrastructure, _ *extensions.Cluster) (bool, error) {
	hasState, err := hasFlowState(infra.Status)
	if err != nil {
		return false, err
	}
	return hasState || GetFlowAnnotationValue(infra), nil
}

// OnDelete returns true if the operation should use the Flow deletion for the given cluster.
func OnDelete(infra *extensionsv1alpha1.Infrastructure, _ *extensions.Cluster) (bool, error) {
	return hasFlowState(infra.Status)
}

// OnRestore decides the reconciler used on migration.
var OnRestore = OnDelete
