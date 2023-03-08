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
	"encoding/json"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"

	"github.com/gardener/gardener/extensions/pkg/controller/common"
	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// AnnotationKeyUseFlow is the annotation key used to enable reconciliation with flow instead of terraformer.
	AnnotationKeyUseFlow = "azure.provider.extensions.gardener.cloud/use-flow"
)

// InfrastructureState represents the last known State of an Infrastructure resource.
// It is saved after a reconciliation and used during restore operations.
type InfrastructureState struct {
	// SavedProviderStatus contains the infrastructure's ProviderStatus.
	SavedProviderStatus *runtime.RawExtension `json:"savedProviderStatus,omitempty"`
	// TerraformState contains the state of the applied terraform config.
	TerraformState *runtime.RawExtension `json:"terraformState,omitempty"`
}

type actuator struct {
	common.RESTConfigContext
	disableProjectedTokenMount bool
}

// NewActuator creates a new infrastructure.Actuator.
func NewActuator(disableProjectedTokenMount bool) infrastructure.Actuator {
	return &actuator{
		disableProjectedTokenMount: disableProjectedTokenMount,
	}
}

func patchProviderStatusAndState(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, status *v1alpha1.InfrastructureStatus, state InfrastructureState, actuatorClient client.Client) error {
	patch := client.MergeFrom(infra.DeepCopy())
	infra.Status.ProviderStatus = &runtime.RawExtension{Object: status}
	infraStateBytes, err := json.Marshal(state)
	if err != nil {
		return err
	}
	infra.Status.State = &runtime.RawExtension{Raw: infraStateBytes}
	return actuatorClient.Status().Patch(ctx, infra, patch)
}
