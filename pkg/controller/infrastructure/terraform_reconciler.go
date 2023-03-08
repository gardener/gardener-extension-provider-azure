package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewTerraformReconciler creates a new TerraformReconciler
func NewTerraformReconciler(a *actuator, logger logr.Logger, stateInitializer terraformer.StateConfigMapInitializer) (Reconciler, error) {
	client := a.Client()
	if client == nil {
		return nil, fmt.Errorf("infrastructure actuator has no client set")
	}

	return &TerraformReconciler{
		Client:                     client,
		Logger:                     logger,
		RestConfig:                 a.RESTConfig(),
		DisableProjectedTokenMount: a.disableProjectedTokenMount,
		StateInitializer:           stateInitializer,
	}, nil
}

type TerraformReconciler struct {
	Client                     client.Client
	Logger                     logr.Logger
	RestConfig                 *rest.Config
	DisableProjectedTokenMount bool
	StateInitializer           terraformer.StateConfigMapInitializer
	tf                         terraformer.Terraformer
}

func (r *TerraformReconciler) Reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig, cluster *controller.Cluster) (*v1alpha1.InfrastructureStatus, error) {
	terraformFiles, err := infrastructure.RenderTerraformerTemplate(infra, cfg, cluster)
	if err != nil {
		return nil, err
	}

	r.tf, err = internal.NewTerraformerWithAuth(r.Logger, r.RestConfig, infrastructure.TerraformerPurpose, infra, r.DisableProjectedTokenMount)
	if err != nil {
		return nil, err
	}

	if err := r.tf.
		InitializeWith(ctx, terraformer.DefaultInitializer(r.Client, terraformFiles.Main, terraformFiles.Variables, terraformFiles.TFVars, r.StateInitializer)).
		Apply(ctx); err != nil {

		return nil, fmt.Errorf("failed to apply the terraform config: %w", err)
	}

	status, err := infrastructure.ComputeTerraformStatus(ctx, r.tf, infra, cfg, cluster)
	if err != nil {
		return nil, err
	}
	return status, nil

}

func (r *TerraformReconciler) GetState(ctx context.Context, status *v1alpha1.InfrastructureStatus) ([]byte, error) {

	terraformState, err := r.tf.GetRawState(ctx)
	if err != nil {
		return nil, err
	}

	stateByte, err := terraformState.Marshal()
	if err != nil {
		return nil, err
	}

	infraState := InfrastructureState{
		SavedProviderStatus: &runtime.RawExtension{
			Object: status,
		},
		TerraformState: &runtime.RawExtension{
			Raw: stateByte,
		},
	}
	return json.Marshal(infraState)
}

func (r *TerraformReconciler) Delete(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig, cluster *controller.Cluster) error {
	tf, err := internal.NewTerraformer(r.Logger, r.RestConfig, infrastructure.TerraformerPurpose, infra, r.DisableProjectedTokenMount)
	if err != nil {
		return err
	}
	// terraform pod from previous reconciliation might still be running, ensure they are gone before doing any operations
	if err := tf.EnsureCleanedUp(ctx); err != nil {
		return err
	}

	azureClientFactory, err := NewAzureClientFactory(ctx, r.Client, infra.Spec.SecretRef)
	if err != nil {
		return err
	}
	resourceGroupExists, err := infrastructure.IsShootResourceGroupAvailable(ctx, azureClientFactory, infra, cfg)
	if err != nil {
		if azureclient.IsAzureAPIUnauthorized(err) {
			r.Logger.Error(err, "Failed to check resource group availability due to invalid credentials")
		} else {
			return err
		}
	}

	if !resourceGroupExists {
		if !azureclient.IsAzureAPIUnauthorized(err) {
			if err := infrastructure.DeleteNodeSubnetIfExists(ctx, azureClientFactory, infra, cfg); err != nil {
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

	return tf.
		InitializeWith(ctx, terraformer.DefaultInitializer(r.Client, terraformFiles.Main, terraformFiles.Variables, terraformFiles.TFVars, r.StateInitializer)).
		SetEnvVars(internal.TerraformerEnvVars(infra.Spec.SecretRef)...).
		Destroy(ctx)
}
