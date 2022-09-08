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

package v1alpha1

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkerConfig contains configuration settings for the worker nodes.
type WorkerConfig struct {
	metav1.TypeMeta `json:",inline"`

	// NodeTemplate contains resource information of the machine which is used by Cluster Autoscaler to generate nodeTemplate during scaling a nodeGroup from zero.
	// +optional
	NodeTemplate *extensionsv1alpha1.NodeTemplate `json:"nodeTemplate,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkerStatus contains information about created worker resources.
type WorkerStatus struct {
	metav1.TypeMeta `json:",inline"`

	// MachineImages is a list of machine images that have been used in this worker. Usually, the extension controller
	// gets the mapping from name/version to the provider-specific machine image data in its componentconfig. However, if
	// a version that is still in use gets removed from this componentconfig it cannot reconcile anymore existing `Worker`
	// resources that are still using this version. Hence, it stores the used versions in the provider status to ensure
	// reconciliation is possible.
	// +optional
	MachineImages []MachineImage `json:"machineImages,omitempty"`

	// VmoDependencies is a list of external VirtualMachineScaleSet Orchestration Mode VM (VMO) dependencies.
	// +optional
	VmoDependencies []VmoDependency `json:"vmoDependencies,omitempty"`
}

// MachineImage is a mapping from logical names and versions to provider-specific machine image data.
type MachineImage struct {
	// Name is the logical name of the machine image.
	Name string `json:"name"`
	// Version is the logical version of the machine image.
	Version string `json:"version"`
	// URN is the uniform resource name of the image, it has the format 'publisher:offer:sku:version'.
	// +optional
	URN *string `json:"urn,omitempty"`
	// ID is the VM image ID
	// +optional
	ID *string `json:"id,omitempty"`
	// CommunityGalleryImageID is the Community Image Gallery image id.
	// +optional
	CommunityGalleryImageID *string `json:"communityGalleryImageID,omitempty"`
	// SharedGalleryImageID is the Shared Image Gallery image id.
	// +optional
	SharedGalleryImageID *string `json:"sharedGalleryImageID,omitempty"`
	// AcceleratedNetworking is an indicator if the image supports Azure accelerated networking.
	// +optional
	AcceleratedNetworking *bool `json:"acceleratedNetworking,omitempty"`
	// Architecture is the CPU architecture of the machine image.
	// +optional
	Architecture *string `json:"architecture,omitempty"`
}

// VmoDependency is dependency reference for a workerpool to a VirtualMachineScaleSet Orchestration Mode VM (VMO).
type VmoDependency struct {
	// PoolName is the name of the worker pool to which the VMO belong to.
	PoolName string `json:"poolName"`
	// ID is the id of the VMO resource on Azure.
	ID string `json:"id"`
	// Name is the name of the VMO resource on Azure.
	Name string `json:"name"`
}
