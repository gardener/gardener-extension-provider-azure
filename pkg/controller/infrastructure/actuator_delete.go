// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package infrastructure

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

var (
	// NewAzureClientFactory initializes a new AzureClientFactory. Exposed for testing.
	NewAzureClientFactory = newAzureClientFactory
)

func newAzureClientFactory(client client.Client) azureclient.Factory {
	return azureclient.NewAzureClientFactory(client)
}

// Delete implements infrastructure.Actuator.
func (a *actuator) Delete(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	tf, err := internal.NewTerraformer(log, a.restConfig, infrastructure.TerraformerPurpose, infra, a.disableProjectedTokenMount)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	// terraform pod from previous reconciliation might still be running, ensure they are gone before doing any operations
	if err := tf.EnsureCleanedUp(ctx); err != nil {
		return err
	}

	config, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return err
	}

	azureClientFactory := NewAzureClientFactory(a.client)
	resourceGroupExists, err := infrastructure.IsShootResourceGroupAvailable(ctx, azureClientFactory, infra, config)
	if err != nil {
		if azureclient.IsAzureAPIUnauthorized(err) {
			log.Error(err, "Failed to check resource group availability due to invalid credentials")
		} else {
			return util.DetermineError(err, helper.KnownCodes)
		}
	}

	if !resourceGroupExists {
		if !azureclient.IsAzureAPIUnauthorized(err) {
			if err := infrastructure.DeleteNodeSubnetIfExists(ctx, azureClientFactory, infra, config); err != nil {
				return util.DetermineError(err, helper.KnownCodes)
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
		log.Info("exiting early as infrastructure state is empty - nothing to do")
		return tf.CleanupConfiguration(ctx)
	}

	terraformFiles, err := infrastructure.RenderTerraformerTemplate(infra, config, cluster)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	return util.DetermineError(tf.
		InitializeWith(ctx, terraformer.DefaultInitializer(a.client, terraformFiles.Main, terraformFiles.Variables, terraformFiles.TFVars, terraformer.StateConfigMapInitializerFunc(NoOpStateInitializer))).
		SetEnvVars(internal.TerraformerEnvVars(infra.Spec.SecretRef)...).
		Destroy(ctx), helper.KnownCodes)
}

func (a *actuator) ForceDelete(_ context.Context, _ logr.Logger, _ *extensionsv1alpha1.Infrastructure, _ *controller.Cluster) error {
	return nil
}

// NoOpStateInitializer is a no-op StateConfigMapInitializerFunc.
func NoOpStateInitializer(_ context.Context, _ client.Client, _, _ string, _ *metav1.OwnerReference) error {
	return nil
}
