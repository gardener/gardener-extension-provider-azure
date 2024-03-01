// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	infrainternal "github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

// FlowReconciler an implementation of an infrastructure reconciler using native SDKs.
type FlowReconciler struct {
	client                     client.Client
	restConfig                 *rest.Config
	log                        logr.Logger
	disableProjectedTokenMount bool
}

// NewFlowReconciler creates a new flow reconciler.
func NewFlowReconciler(a *actuator, log logr.Logger, projToken bool) (Reconciler, error) {
	return &FlowReconciler{
		client:                     a.client,
		restConfig:                 a.restConfig,
		log:                        log,
		disableProjectedTokenMount: projToken,
	}, nil
}

// Reconcile reconciles the infrastructure and returns the status (state of the world), the state (input for the next loops) and any errors that occurred.
func (f *FlowReconciler) Reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	infraState, err := helper.InfrastructureStateFromRaw(infra.Status.State)
	if err != nil {
		return err
	}

	tf, err := internal.NewTerraformer(f.log, f.restConfig, infrainternal.TerraformerPurpose, infra, f.disableProjectedTokenMount)
	if err != nil {
		return err
	}

	if !tf.IsStateEmpty(ctx) {
		// this is a special case when migrating from Terraform. If TF had created any resources (meaning there is an actual tf.state written)
		// we mark that there are infra resources created.
		infraState.Data[infraflow.CreatedResourcesExistKey] = "true"
	}

	auth, err := internal.GetClientAuthData(ctx, f.client, infra.Spec.SecretRef, false)
	if err != nil {
		return err
	}

	factory, err := NewAzureClientFactory(ctx, f.client, infra.Spec.SecretRef)
	if err != nil {
		return err
	}

	persistFunc := func(ctx context.Context, state *runtime.RawExtension) error {
		return patchProviderStatusAndState(ctx, f.client, infra, nil, state)
	}

	fctx, err := infraflow.NewFlowContext(factory, auth, f.log, infra, cluster, infraState, persistFunc)
	if err != nil {
		return err
	}

	status, state, err := fctx.Reconcile(ctx)
	if err != nil {
		return err
	}

	if err := patchProviderStatusAndState(ctx, f.client, infra, status, state); err != nil {
		return err
	}

	return CleanupTerraformerResources(ctx, tf)
}

// Delete deletes the infrastructure resource using the flow reconciler.
func (f *FlowReconciler) Delete(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	factory, err := NewAzureClientFactory(ctx, f.client, infra.Spec.SecretRef)
	if err != nil {
		return err
	}

	infraState, err := helper.InfrastructureStateFromRaw(infra.Status.State)
	if err != nil {
		return err
	}

	fctx, err := infraflow.NewFlowContext(factory, nil, f.log, infra, cluster, infraState, nil)
	if err != nil {
		return err
	}

	err = fctx.Delete(ctx)
	if err != nil {
		return err
	}

	tf, err := internal.NewTerraformer(f.log, f.restConfig, infrainternal.TerraformerPurpose, infra, f.disableProjectedTokenMount)
	if err != nil {
		return err
	}
	return CleanupTerraformerResources(ctx, tf)
}

// Restore implements the restoration of an infrastructure resource during the control plane migration.
func (f *FlowReconciler) Restore(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	return f.Reconcile(ctx, infra, cluster)
}
