// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"strings"

	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	"github.com/go-logr/logr"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// ShouldUseFlow returns true if the new flow reconciler should be used for the reconciliation.
func ShouldUseFlow(infrastructure *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) bool {
	return (infrastructure.Annotations != nil && strings.EqualFold(infrastructure.Annotations[AnnotationKeyUseFlow], "true")) ||
		(cluster.Shoot != nil && cluster.Shoot.Annotations != nil && strings.EqualFold(cluster.Shoot.Annotations[AnnotationKeyUseFlow], "true"))
}

// NewFlowReconciler creates a new flow reconciler.
func NewFlowReconciler(ctx context.Context, a *actuator, infra *extensionsv1alpha1.Infrastructure, logger logr.Logger) (*infraflow.FlowReconciler, error) {
	client := a.Client()
	if client == nil {
		return nil, fmt.Errorf("infrastructure actuator has no client set")
	}
	auth, err := internal.GetClientAuthData(ctx, client, infra.Spec.SecretRef, false)
	if err != nil {
		return nil, err
	}
	factory, err := azureclient.NewAzureClientFactoryWithAuthAndClient(auth, client)
	if err != nil {
		return nil, err
	}
	reconciler := infraflow.NewFlowReconciler(factory, logger)
	return reconciler, nil
}
