// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	// CloudConfiguration contains config that controls which cloud to connect to.
	// +optional
	CloudConfiguration *CloudConfiguration `json:"cloudConfiguration,omitempty"`
}

// CloudConfiguration contains detailed config for the cloud to connect to. Currently we only support selection of well-
// known Azure-instances by name, but this could be extended in future to support private clouds.
type CloudConfiguration struct {
	// Name is the name of the cloud to connect to, e.g. "AzurePublic" or "AzureChina".
	Name string `json:"name,omitempty"`
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
	// SkipMarketplaceAgreement skips the marketplace agreement check when enabled.
	// +optional
	SkipMarketplaceAgreement *bool `json:"skipMarketplaceAgreement,omitempty"`
	// ID is the Shared Image Gallery image id.
	// +optional
	ID *string `json:"id,omitempty"`
	// CommunityGalleryImageID is the Community Image Gallery image id, it has the format '/CommunityGalleries/myGallery/Images/myImage/Versions/myVersion'
	// +optional
	CommunityGalleryImageID *string `json:"communityGalleryImageID,omitempty"`
	// SharedGalleryImageID is the Shared Image Gallery image id, it has the format '/SharedGalleries/sharedGalleryName/Images/sharedGalleryImageName/Versions/sharedGalleryImageVersionName'
	// +optional
	SharedGalleryImageID *string `json:"sharedGalleryImageID,omitempty"`
	// AcceleratedNetworking is an indicator if the image supports Azure accelerated networking.
	// +optional
	AcceleratedNetworking *bool `json:"acceleratedNetworking,omitempty"`
	// Architecture is the CPU architecture of the machine image.
	// +optional
	Architecture *string `json:"architecture,omitempty"`
}

// MachineType contains provider specific information to a machine type.
type MachineType struct {
	// Name is the name of the machine type.
	Name string `json:"name"`
	// AcceleratedNetworking is an indicator if the machine type supports Azure accelerated networking.
	// +optional
	AcceleratedNetworking *bool `json:"acceleratedNetworking,omitempty"`
}
