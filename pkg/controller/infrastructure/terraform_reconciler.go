package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

// NewTerraformReconciler creates a new TerraformReconciler
func NewTerraformReconciler(a *actuator, logger logr.Logger, restConfig *rest.Config, disableProjectedTokenMount bool) (Reconciler, error) {
	return &TerraformReconciler{
		Client:                     a.client,
		Logger:                     logger,
		RestConfig:                 restConfig,
		disableProjectedTokenMount: disableProjectedTokenMount,
	}, nil
}

var _ Reconciler = &TerraformReconciler{}

// DefaultAzureClientFactoryFunc is a hook to monkeypatch factory ctor during tests
var DefaultAzureClientFactoryFunc = azureclient.NewAzureClientFactoryFromSecret

// TerraformReconciler can reconcile infrastructure objects using Terraform.
type TerraformReconciler struct {
	Client                     client.Client
	Logger                     logr.Logger
	RestConfig                 *rest.Config
	disableProjectedTokenMount bool
}

// Restore restores the infrastructure after a control plane migration. Effectively it performs a recovery of data from the infrastructure.status.state and
// proceeds to reconcile.
func (r *TerraformReconciler) Restore(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	var initializer terraformer.StateConfigMapInitializer
	infraState := &infrastructure.InfrastructureState{}
	if err := json.Unmarshal(infra.Status.State.Raw, infraState); err != nil {
		return err
	}

	terraformState, err := terraformer.UnmarshalRawState(infraState.TerraformState)
	if err != nil {
		return err
	}
	initializer = terraformer.CreateOrUpdateState{State: &terraformState.Data}
	patch := client.MergeFrom(infra.DeepCopy())
	infra.Status.ProviderStatus = infraState.SavedProviderStatus
	if err := r.Client.Status().Patch(ctx, infra, patch); err != nil {
		return err
	}

	return r.reconcile(ctx, infra, cluster, initializer)
}

// Reconcile manages infrastructure resources according to desired spec.
func (r *TerraformReconciler) Reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	return r.reconcile(ctx, infra, cluster, terraformer.StateConfigMapInitializerFunc(terraformer.CreateState))
}

// Reconcile reconciles the infrastructure resource according to spec.
func (r *TerraformReconciler) reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster, initializer terraformer.StateConfigMapInitializer) error {
	cfg, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return err
	}
	terraformFiles, err := infrastructure.RenderTerraformerTemplate(infra, cfg, cluster)
	if err != nil {
		return err
	}

	tf, err := internal.NewTerraformerWithAuth(r.Logger, r.RestConfig, infrastructure.TerraformerPurpose, infra, r.disableProjectedTokenMount)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	if err := tf.
		InitializeWith(ctx, terraformer.DefaultInitializer(r.Client, terraformFiles.Main, terraformFiles.Variables, terraformFiles.TFVars, initializer)).
		Apply(ctx); err != nil {

		return fmt.Errorf("failed to apply the terraform config: %w", err)
	}

	status, err := infrastructure.ComputeTerraformStatus(ctx, tf, infra, cfg, cluster)
	if err != nil {
		return err
	}
	state, err := r.getState(ctx, tf, status)
	if err != nil {
		return err
	}

	return patchProviderStatusAndState(ctx, r.Client, infra, status, state)
}

// getState calculates the State resource after each reconciliation.
func (r *TerraformReconciler) getState(ctx context.Context, tf terraformer.Terraformer, status *v1alpha1.InfrastructureStatus) (*runtime.RawExtension, error) {
	terraformState, err := tf.GetRawState(ctx)
	if err != nil {
		return nil, err
	}

	stateByte, err := terraformState.Marshal()
	if err != nil {
		return nil, err
	}

	infraState := &infrastructure.InfrastructureState{
		SavedProviderStatus: &runtime.RawExtension{
			Object: status,
		},
		TerraformState: &runtime.RawExtension{
			Raw: stateByte,
		},
	}
	return infraState.ToRawExtension()
}

// Delete removes any created infrastructure resource on the provider.
func (r *TerraformReconciler) Delete(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {

	tf, err := internal.NewTerraformer(r.Logger, r.RestConfig, infrastructure.TerraformerPurpose, infra, r.disableProjectedTokenMount)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	// terraform pod from previous reconciliation might still be running, ensure they are gone before doing any operations
	if err := tf.EnsureCleanedUp(ctx); err != nil {
		return err
	}

	cloudProfile, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return err
	}

	var cloudConfiguration *azure.CloudConfiguration
	if cloudProfile != nil {
		cloudConfiguration = cloudProfile.CloudConfiguration
	}

	azCloudConfiguration, err := azureclient.AzureCloudConfigurationFromCloudConfiguration(cloudConfiguration)
	if err != nil {
		return err
	}

	clientFactory, err := DefaultAzureClientFactoryFunc(
		ctx,
		r.Client,
		infra.Spec.SecretRef,
		false,
		azureclient.WithCloudConfiguration(azCloudConfiguration),
	)
	if err != nil {
		return err
	}

	cfg, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return err
	}

	resourceGroupExists, err := infrastructure.IsShootResourceGroupAvailable(ctx, clientFactory, infra, cfg)
	if err != nil {
		if azureclient.IsAzureAPIUnauthorized(err) {
			r.Logger.Error(err, "Failed to check resource group availability due to invalid credentials")
		} else {
			return err
		}
	}

	if !resourceGroupExists {
		if !azureclient.IsAzureAPIUnauthorized(err) {
			if err := infrastructure.DeleteNodeSubnetIfExists(ctx, clientFactory, infra, cfg); err != nil {
				return err
			}
		}

		if err := tf.RemoveTerraformerFinalizerFromConfig(ctx); err != nil {
			return err
		}

		return tf.CleanupConfiguration(ctx)
	}

	// If the Terraform state is empty then we can exit early as we didn't create anything. Though, we clean up potentially
	// created configmaps/secrets related to the Terraformer.
	stateIsEmpty := tf.IsStateEmpty(ctx)
	if stateIsEmpty {
		r.Logger.Info("exiting early as infrastructure state is empty - nothing to do")
		return tf.CleanupConfiguration(ctx)
	}

	terraformFiles, err := infrastructure.RenderTerraformerTemplate(infra, cfg, cluster)
	if err != nil {
		return err
	}

	if err = tf.
		InitializeWith(ctx, terraformer.DefaultInitializer(r.Client, terraformFiles.Main, terraformFiles.Variables, terraformFiles.TFVars, terraformer.StateConfigMapInitializerFunc(NoOpStateInitializer))).
		SetEnvVars(internal.TerraformerEnvVars(infra.Spec.SecretRef)...).
		Destroy(ctx); err != nil {
		return err
	}

	// make sure the resource group for the shoot is properly cleaned up even if it is missing from terraform state.
	return infrastructure.DeleteShootResourceGroupIfExists(ctx, clientFactory, infra, cfg)
}

// NoOpStateInitializer is a no-op StateConfigMapInitializerFunc.
func NoOpStateInitializer(_ context.Context, _ client.Client, _, _ string, _ *metav1.OwnerReference) error {
	return nil
}
