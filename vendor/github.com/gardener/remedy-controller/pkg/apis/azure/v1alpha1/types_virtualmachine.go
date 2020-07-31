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

// VirtualMachine represents an Azure virtual machine.
type VirtualMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineSpec   `json:"spec,omitempty"`
	Status VirtualMachineStatus `json:"status,omitempty"`
}

// VirtualMachineSpec represents the spec of an Azure virtual machine.
type VirtualMachineSpec struct {
	// Hostname is the hostname of the Kubernetes node for this virtual machine.
	Hostname string `json:"hostname"`
	// ProviderID is the provider ID of the Kubernetes node for this virtual machine.
	ProviderID string `json:"providerID"`
	// NotReadyOrUnreachable is whether the Kubernetes node for this virtual machine is either not ready or unreachable.
	NotReadyOrUnreachable bool `json:"notReadyOrUnreachable"`
}

// VirtualMachineStatus represents the status of an Azure virtual machine.
type VirtualMachineStatus struct {
	// Exists specifies whether the virtual machine resource exists or not.
	Exists bool `json:"exists"`
	// ID is the id of the virtual machine resource in Azure.
	ID *string `json:"id,omitempty"`
	// Name is the name of the virtual machine resource in Azure.
	Name *string `json:"name,omitempty"`
	// ProvisioningState is the provisioning state of the virtual machine resource in Azure.
	ProvisioningState *string `json:"provisioningState,omitempty"`
	// FailedOperations is a list of all failed operations on the virtual machine resource in Azure.
	FailedOperations []FailedOperation `json:"failedOperations,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VirtualMachineList contains a list of VirtualMachine.
type VirtualMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []VirtualMachine `json:"items"`
}
