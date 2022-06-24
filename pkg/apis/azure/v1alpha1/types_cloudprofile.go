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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudProfileConfig contains provider-specific configuration that is embedded into Gardener's `CloudProfile`
// resource.
type CloudProfileConfig struct {
	metav1.TypeMeta `json:",inline"`
	// CountUpdateDomains is list of update domain counts for each region.
	CountUpdateDomains []DomainCount `json:"countUpdateDomains"`
	// CountFaultDomains is list of fault domain counts for each region.
	CountFaultDomains []DomainCount `json:"countFaultDomains"`
	// MachineImages is the list of machine images that are understood by the controller. It maps
	// logical names and versions to provider-specific identifiers.
	MachineImages []MachineImages `json:"machineImages"`
	// MachineTypes is a list of machine types complete with provider specific information.
	// +optional
	MachineTypes []MachineType `json:"machineTypes,omitempty"`
}

// DomainCount defines the region and the count for this domain count value.
type DomainCount struct {
	// Region is a region.
	Region string `json:"region"`
	// Count is the count value for the respective domain count.
	Count int32 `json:"count"`
}

// MachineImages is a mapping from logical names and versions to provider-specific identifiers.
type MachineImages struct {
	// Name is the logical name of the machine image.
	Name string `json:"name"`
	// Versions contains versions and a provider-specific identifier.
	Versions []MachineImageVersion `json:"versions"`
}

// MachineImageVersion contains a version and a provider-specific identifier.
type MachineImageVersion struct {
	// Version is the version of the image.
	Version string `json:"version"`
	// URN is the uniform resource name of the image, it has the format 'publisher:offer:sku:version'.
	// +optional
	URN *string `json:"urn,omitempty"`
	// ID is the Shared Image Gallery image id.
	// +optional
	ID *string `json:"id,omitempty"`
	// CommunityGalleryImageID is the Community Image Gallery image id, it has the format '/CommunityGalleries/myGallery/Images/myImage/Versions/myVersion'
	// +optional
	CommunityGalleryImageID *string `json:"communityGalleryImageID,omitempty"`
	// AcceleratedNetworking is an indicator if the image supports Azure accelerated networking.
	// +optional
	AcceleratedNetworking *bool `json:"acceleratedNetworking,omitempty"`
}

// MachineType contains provider specific information to a machine type.
type MachineType struct {
	// Name is the name of the machine type.
	Name string `json:"name"`
	// AcceleratedNetworking is an indicator if the machine type supports Azure accelerated networking.
	// +optional
	AcceleratedNetworking *bool `json:"acceleratedNetworking,omitempty"`
}
