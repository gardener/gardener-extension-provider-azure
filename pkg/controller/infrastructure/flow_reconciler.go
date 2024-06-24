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

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
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
	var (
		infraState *azure.InfrastructureState
		err        error
	)
	fsOk, err := hasFlowState(infra.Status)
	if err != nil {
		return err
	}

	if fsOk {
		infraState, err = helper.InfrastructureStateFromRaw(infra.Status.State)
		if err != nil {
			return err
		}
	} else {
		// otherwise migrate it from the terraform state if needed.
		infraState, err = f.migrateFromTerraform(ctx, infra)
		if err != nil {
			return err
		}
	}

	auth, err := internal.GetClientAuthData(ctx, f.client, infra.Spec.SecretRef, false)
	if err != nil {
		return err
	}

	cloudProfile, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return err
	}

	azCloudConfiguration, err := azureclient.AzureCloudConfigurationFromCloudConfiguration(cloudProfile.CloudConfiguration)
	if err != nil {
		return err
	}

	factory, err := azureclient.NewAzureClientFactoryFromSecret(
		ctx,
		f.client,
		infra.Spec.SecretRef,
		false,
		azureclient.WithCloudConfiguration(azCloudConfiguration),
	)
	if err != nil {
		return err
	}

	fctx, err := infraflow.NewFlowContext(infraflow.Opts{
		Client:  f.client,
		Factory: factory,
		Auth:    auth,
		Logger:  f.log,
		Infra:   infra,
		Cluster: cluster,
		State:   infraState,
	})
	if err != nil {
		return err
	}

	return fctx.Reconcile(ctx)
}

// Delete deletes the infrastructure resource using the flow reconciler.
func (f *FlowReconciler) Delete(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	cloudProfile, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return err
	}

	azCloudConfiguration, err := azureclient.AzureCloudConfigurationFromCloudConfiguration(cloudProfile.CloudConfiguration)
	if err != nil {
		return err
	}

	factory, err := azureclient.NewAzureClientFactoryFromSecret(
		ctx,
		f.client,
		infra.Spec.SecretRef,
		false,
		azureclient.WithCloudConfiguration(azCloudConfiguration),
	)
	if err != nil {
		return err
	}

	infraState, err := helper.InfrastructureStateFromRaw(infra.Status.State)
	if err != nil {
		return err
	}

	fctx, err := infraflow.NewFlowContext(infraflow.Opts{
		Client:  f.client,
		Factory: factory,
		Auth:    nil,
		Logger:  f.log,
		Infra:   infra,
		Cluster: cluster,
		State:   infraState,
	})
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

func (f *FlowReconciler) migrateFromTerraform(ctx context.Context, infra *extensionsv1alpha1.Infrastructure) (*azure.InfrastructureState, error) {
	var (
		state = &azure.InfrastructureState{
			Data: map[string]string{},
		}
	)
	tf, err := internal.NewTerraformer(f.log, f.restConfig, infrainternal.TerraformerPurpose, infra, f.disableProjectedTokenMount)
	if err != nil {
		return nil, err
	}

	// nothing to do if state is empty
	if tf.IsStateEmpty(ctx) {
		return state, nil
	}

	// this is a special case when migrating from Terraform. If TF had created any resources (meaning there is an actual content in tf.state written)
	// we will use a specific "marker" to make the reconciler aware of existing resources. This will prevent the reconciler from skipping the deletion flow.
	state.Data[infraflow.CreatedResourcesExistKey] = "true"

	return state, infrainternal.PatchProviderStatusAndState(ctx, f.client, infra, nil, &runtime.RawExtension{Object: state}, nil)
}
