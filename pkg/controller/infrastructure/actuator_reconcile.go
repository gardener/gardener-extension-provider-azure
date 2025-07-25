// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow"
	infrainternal "github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

const (
	terraformerPurpose = "infra"
)

// Reconcile implements infrastructure.Actuator.
func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	return util.DetermineError(a.reconcile(ctx, log, infra, cluster), helper.KnownCodes)
}

func (a *actuator) reconcile(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	var (
		infraState *azure.InfrastructureState
		err        error
	)
	fsOk, err := helper.HasFlowState(infra.Status)
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
		infraState, err = a.migrateFromTerraform(ctx, log, infra)
		if err != nil {
			return err
		}
	}

	auth, _, err := azureclient.GetClientAuthData(ctx, a.client, infra.Spec.SecretRef, false)
	if err != nil {
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

	azCloudConfiguration, err := azureclient.AzureCloudConfiguration(cloudConfiguration, &cluster.Shoot.Spec.Region)
	if err != nil {
		return err
	}

	factory, err := azureclient.NewAzureClientFactoryFromSecret(
		ctx,
		a.client,
		infra.Spec.SecretRef,
		false,
		azureclient.WithCloudConfiguration(azCloudConfiguration),
	)
	if err != nil {
		return err
	}

	fctx, err := infraflow.NewFlowContext(infraflow.Opts{
		Client:  a.client,
		Factory: factory,
		Auth:    auth,
		Logger:  log,
		Infra:   infra,
		Cluster: cluster,
		State:   infraState,
	})
	if err != nil {
		return err
	}

	return fctx.Reconcile(ctx)
}

func (a *actuator) migrateFromTerraform(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure) (*azure.InfrastructureState, error) {
	var (
		state = &azure.InfrastructureState{
			Data: map[string]string{},
		}
	)
	tf, err := newTerraformer(log, a.restConfig, terraformerPurpose, infra, a.disableProjectedTokenMount)
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

	return state, infrainternal.PatchProviderStatusAndState(ctx, a.client, infra, nil, &runtime.RawExtension{Object: state}, nil)
}
