// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PublicIPAddress represents an Azure public IP address.
type PublicIPAddress struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PublicIPAddressSpec   `json:"spec,omitempty"`
	Status PublicIPAddressStatus `json:"status,omitempty"`
}

// PublicIPAddressSpec represents the spec of an Azure public IP address.
type PublicIPAddressSpec struct {
	// IPAddres is the actual IP address of the public IP address resource in Azure.
	IPAddress string `json:"ipAddress"`
}

// PublicIPAddressStatus represents the status of an Azure public IP address.
type PublicIPAddressStatus struct {
	// Exists specifies whether the public IP address resource exists or not.
	Exists bool `json:"exists"`
	// ID is the id of the public IP address resource in Azure.
	ID *string `json:"id,omitempty"`
	// Name is the name of the public IP address resource in Azure.
	Name *string `json:"name,omitempty"`
	// ProvisioningState is the provisioning state of the public IP address resource in Azure.
	ProvisioningState *string `json:"provisioningState,omitempty"`
	// FailedOperations is a list of all failed operations on the virtual machine resource in Azure.
	FailedOperations []FailedOperation `json:"failedOperations,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PublicIPAddressList contains a list of PublicIPAddress.
type PublicIPAddressList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []PublicIPAddress `json:"items"`
}
