// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	"github.com/gardener/gardener/extensions/pkg/util"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	securityv1alpha1constants "github.com/gardener/gardener/pkg/apis/security/v1alpha1/constants"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
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

	cloudProviderSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.SecretNameCloudProvider,
			Namespace: infra.Namespace,
		},
	}
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(cloudProviderSecret), cloudProviderSecret); err != nil {
		return fmt.Errorf("failed getting cloudprovider secret: %w", err)
	}
	useWorkloadIdentity := cloudProviderSecret.Labels[securityv1alpha1constants.LabelPurpose] == securityv1alpha1constants.LabelPurposeWorkloadIdentityTokenRequestor

	terraformFiles, err := infrastructure.RenderTerraformerTemplate(infra, cfg, cluster, useWorkloadIdentity)
	if err != nil {
		return err
	}

	tf, err := internal.NewTerraformerWithAuth(r.Logger, r.RestConfig, infrastructure.TerraformerPurpose, infra, r.disableProjectedTokenMount, useWorkloadIdentity)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	if err := tf.
		InitializeWith(ctx, terraformer.DefaultInitializer(r.Client, terraformFiles.Main, terraformFiles.Variables, terraformFiles.TFVars, initializer)).
		Apply(ctx); err != nil {

		codes := util.DetermineErrorCodes(err, helper.KnownCodes)
		isDependenciesError := false
		for _, code := range codes {
			if code == gardencorev1beta1.ErrorInfraDependencies {
				isDependenciesError = true
			}
		}
		if !isDependenciesError {
			return err
		}
		r.Logger.Info(
			"Terraform application failed with infrastructure dependencies error. Will attempt to cleanup the resource group if it is empty",
			"error", err)

		ok, inErr := r.cleanResourceGroupIfNeeded(ctx, infra, cluster, cfg)
		if inErr == nil && ok {
			// we return a retryable error for the controller to retry instead of locking the user to a non-retryable error.
			return gardencorev1beta1helper.NewErrorWithCodes(fmt.Errorf("retry after resource group cleanup"), gardencorev1beta1.ErrorRetryableInfraDependencies)
		}
		if inErr != nil {
			r.Logger.Error(inErr, "Checking and cleaning up the resource group after an unsuccessful terraform apply failed")
		}

		return fmt.Errorf("failed to apply the terraform config: %w", err)
	}

	status, err := infrastructure.ComputeTerraformStatus(ctx, tf, infra, cfg, cluster)
	if err != nil {
		return err
	}
	terraformState, err := tf.GetRawState(ctx)
	if err != nil {
		return err
	}
	state, err := r.getState(terraformState, status)
	if err != nil {
		return err
	}
	egressCidrs, err := infrastructure.EgressCidrs(terraformState)
	if err != nil {
		return err
	}

	return infrastructure.PatchProviderStatusAndState(ctx, r.Client, infra, status, state, egressCidrs)
}

// getState calculates the State resource after each reconciliation.
func (r *TerraformReconciler) getState(terraformState *terraformer.RawState, status *v1alpha1.InfrastructureStatus) (*runtime.RawExtension, error) {

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

	clientFactory, err := r.getClientFactory(ctx, infra, cluster)
	if err != nil {
		return err
	}

	cfg, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return err
	}
	status, err := helper.InfrastructureStatusFromInfrastructure(infra)
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

	cloudProviderSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.SecretNameCloudProvider,
			Namespace: infra.Namespace,
		},
	}
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(cloudProviderSecret), cloudProviderSecret); err != nil {
		return fmt.Errorf("failed getting cloudprovider secret: %w", err)
	}
	useWorkloadIdentity := cloudProviderSecret.Labels[securityv1alpha1constants.LabelPurpose] == securityv1alpha1constants.LabelPurposeWorkloadIdentityTokenRequestor

	terraformFiles, err := infrastructure.RenderTerraformerTemplate(infra, cfg, cluster, useWorkloadIdentity)
	if err != nil {
		return err
	}

	if err = tf.
		InitializeWith(ctx, terraformer.DefaultInitializer(r.Client, terraformFiles.Main, terraformFiles.Variables, terraformFiles.TFVars, terraformer.StateConfigMapInitializerFunc(NoOpStateInitializer))).
		SetEnvVars(internal.TerraformerEnvVars(infra.Spec.SecretRef, useWorkloadIdentity)...).
		Destroy(ctx); err != nil {
		return err
	}

	// make sure the resource group for the shoot is properly cleaned up even if it is missing from terraform state.
	return infrastructure.DeleteShootResourceGroupIfExists(ctx, clientFactory, infra, cfg, status)
}

// NoOpStateInitializer is a no-op StateConfigMapInitializerFunc.
func NoOpStateInitializer(_ context.Context, _ client.Client, _, _ string, _ *metav1.OwnerReference) error {
	return nil
}

func (r *TerraformReconciler) getClientFactory(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, _ *controller.Cluster) (azureclient.Factory, error) {
	return DefaultAzureClientFactoryFunc(
		ctx,
		r.Client,
		infra.Spec.SecretRef,
		false,
	)
}

func (r *TerraformReconciler) cleanResourceGroupIfNeeded(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster, cfg *azure.InfrastructureConfig) (bool, error) {
	var err error
	// skip operations on user resource groups
	if cfg.ResourceGroup != nil {
		return false, nil
	}
	// skip operations if we are not creating the resource group for the first time.
	if lastOp := infra.Status.LastOperation; lastOp == nil || lastOp.Type != gardencorev1beta1.LastOperationTypeCreate {
		return false, nil
	}

	status, err := helper.InfrastructureStatusFromInfrastructure(infra)
	if err != nil {
		return false, err
	}

	rgName := infrastructure.ShootResourceGroupName(infra, cfg, status)

	clientFactory, err := r.getClientFactory(ctx, infra, cluster)
	if err != nil {
		return false, err
	}

	resourceGroupClient, err := clientFactory.Group()
	if err != nil {
		return false, err
	}
	if ok, err := resourceGroupClient.CheckExistence(ctx, rgName); err != nil {
		return false, err
	} else if !ok {
		return false, nil
	}

	resourceClient, err := clientFactory.Resource()
	if err != nil {
		return false, err
	}
	res, err := resourceClient.ListByResourceGroup(ctx, rgName, &armresources.ClientListByResourceGroupOptions{
		Top: ptr.To(int32(1)),
	})
	if err != nil {
		return false, err
	} else if len(res) > 0 {
		return false, nil
	}

	r.Logger.Info("empty resource group detected on operation of type Create. Attempting to delete resource group", "resourceGroup", rgName)
	return true, resourceGroupClient.Delete(ctx, rgName)
}
