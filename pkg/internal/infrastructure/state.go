// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/runtime"
)

// InfrastructureState represents the last known State of an Infrastructure resource.
// It is saved after a reconciliation and used during restore operations.
type InfrastructureState struct {
	// SavedProviderStatus contains the infrastructure's ProviderStatus.
	SavedProviderStatus *runtime.RawExtension `json:"savedProviderStatus,omitempty"`
	// TerraformState contains the state of the last applied terraform config.
	TerraformState *runtime.RawExtension `json:"terraformState,omitempty"`
	// // FlowState contains the state of the last applied Flow reconciliation.
	// FlowState *runtime.RawExtension `json:"flowState,omitempty"`
}

// ToRawExtension marshalls the struct and returns a runtime.RawExtension.
func (i *InfrastructureState) ToRawExtension() (*runtime.RawExtension, error) {
	j, err := json.Marshal(i)
	if err != nil {
		return nil, err
	}

	return &runtime.RawExtension{Raw: j}, nil
}
