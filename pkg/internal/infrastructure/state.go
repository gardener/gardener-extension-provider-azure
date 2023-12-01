//  Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

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
