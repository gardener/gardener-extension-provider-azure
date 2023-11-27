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
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
)

func (a *actuator) Delete(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	return util.DetermineError(a.delete(ctx, log, SelectorFunc(OnDelete), infra, cluster), helper.KnownCodes)
}

// Delete implements infrastructure.Actuator.
func (a *actuator) delete(ctx context.Context, log logr.Logger, selector StrategySelector, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	useFlow, err := selector.Select(infra, cluster)
	if err != nil {
		return err
	}

	factory := ReconcilerFactoryImpl{
		ctx:   ctx,
		log:   log,
		a:     a,
		infra: infra,
	}

	reconciler, err := factory.Build(useFlow)
	if err != nil {
		return err
	}

	return reconciler.Delete(ctx, infra, cluster)
}

func (a *actuator) ForceDelete(_ context.Context, _ logr.Logger, _ *extensionsv1alpha1.Infrastructure, _ *controller.Cluster) error {
	return nil
}
