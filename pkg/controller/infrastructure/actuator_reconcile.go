// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

// Reconcile implements infrastructure.Actuator.
func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	return a.reconcile(ctx, log, infra, cluster, terraformer.StateConfigMapInitializerFunc(terraformer.CreateState))
}

func (a *actuator) reconcile(ctx context.Context, logger logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster, stateInitializer terraformer.StateConfigMapInitializer) error {
	config, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return err
	}

	// TOOD reconcile template still not used
	//selector := StrategySelector{
	//	//Factory: MockFactory{ctrl, tfStateRaw},
	//	Client: a.Client(),
	//}
	//selector.Reconcile(useFlow, ctx, infra, config, cluster) // TODO add cleanupTF

	var reconciler Reconciler
	factory := ReconcilerFactoryImpl{
		ctx:              ctx,
		log:              logger,
		a:                a,
		infra:            infra,
		stateInitializer: stateInitializer,
	}
	strategy := StrategySelector{
		Factory: factory,
		Client:  a.Client(),
	}
	useFlow, err := strategy.ShouldReconcileWithFlow(infra, cluster)
	if err != nil {
		return err
	}
	//strategy.Reconcile(useFlow,ctx,infra,config,cluster) // TODO use instead of below
	if useFlow {
		if err := cleanupTerraform(ctx, logger, a, infra); err != nil {
			return fmt.Errorf("failed to cleanup terraform resources: %w", err)
		}
		reconciler, err = NewFlowReconciler(ctx, a, infra, logger)
		if err != nil {
			return err
		}
	} else {
		reconciler, err = NewTerraformReconciler(a, logger, stateInitializer)
		if err != nil {
			return fmt.Errorf("failed to init terraform reconciler: %w", err)
		}
	}
	status, err := reconciler.Reconcile(ctx, infra, config, cluster)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}
	state, err := reconciler.GetState(ctx, status)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}
	return patchProviderStatusAndState(ctx, infra, status, state, a.Client())
}

func cleanupTerraform(ctx context.Context, logger logr.Logger, a *actuator, infra *extensionsv1alpha1.Infrastructure) error {
	tf, err := internal.NewTerraformer(logger, a.RESTConfig(), infrastructure.TerraformerPurpose, infra, a.disableProjectedTokenMount)
	if err != nil {
		return err
	}
	// terraform pod from previous reconciliation might still be running, ensure they are gone before doing any operations
	if err := tf.EnsureCleanedUp(ctx); err != nil {
		return err
	}

	if err := tf.CleanupConfiguration(ctx); err != nil {
		return err
	}

	return tf.RemoveTerraformerFinalizerFromConfig(ctx)
}
