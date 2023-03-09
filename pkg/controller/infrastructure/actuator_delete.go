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
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"

	v1 "k8s.io/api/core/v1"
)

var (
	// NewAzureClientFactory initializes a new AzureClientFactory. Exposed for testing.
	NewAzureClientFactory = newAzureClientFactory
)

func newAzureClientFactory(ctx context.Context, client client.Client, secretRef v1.SecretReference) (azureclient.Factory, error) {
	return azureclient.NewAzureClientFactory(ctx, client, secretRef)
}

// Delete implements infrastructure.Actuator.
func (a *actuator) Delete(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	config, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return err
	}

	selector := StrategySelector{}
	useFlow, err := selector.ShouldDeleteWithFlow(infra.Status)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}
	var reconciler Reconciler
	if useFlow {
		if err := cleanupTerraform(ctx, log, a, infra); err != nil {
			return fmt.Errorf("failed to cleanup terraform resources: %w", err)
		}
		reconciler, err = NewFlowReconciler(ctx, a, infra, log)
		if err != nil {
			return err
		}
	} else {
		reconciler, err = NewTerraformReconciler(a, log, terraformer.StateConfigMapInitializerFunc(NoOpStateInitializer))
		if err != nil {
			return fmt.Errorf("failed to initialize terraform reconciler: %w", err)
		}
	}
	return util.DetermineError(reconciler.Delete(ctx, infra, config, cluster), helper.KnownCodes)
}

// NoOpStateInitializer is a no-op StateConfigMapInitializerFunc.
func NoOpStateInitializer(ctx context.Context, c client.Client, namespace, name string, owner *metav1.OwnerReference) error {
	return nil
}
