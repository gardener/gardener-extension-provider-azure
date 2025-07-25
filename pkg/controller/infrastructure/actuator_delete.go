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

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow"
)

func (a *actuator) Delete(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	return util.DetermineError(a.delete(ctx, log, infra, cluster), helper.KnownCodes)
}

// Delete implements infrastructure.Actuator.
func (a *actuator) delete(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
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

	infraState, err := helper.InfrastructureStateFromRaw(infra.Status.State)
	if err != nil {
		return err
	}

	fctx, err := infraflow.NewFlowContext(infraflow.Opts{
		Client:  a.client,
		Factory: factory,
		Auth:    nil,
		Logger:  log,
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

	tf, err := newTerraformer(log, a.restConfig, terraformerPurpose, infra, a.disableProjectedTokenMount)
	if err != nil {
		return err
	}
	return CleanupTerraformerResources(ctx, tf)
}

func (a *actuator) ForceDelete(_ context.Context, _ logr.Logger, _ *extensionsv1alpha1.Infrastructure, _ *controller.Cluster) error {
	return nil
}
