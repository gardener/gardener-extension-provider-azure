// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudProfileConfig contains provider-specific configuration that is embedded into Gardener's `CloudProfile`
// resource.
type CloudProfileConfig struct {
	metav1.TypeMeta
	// CountUpdateDomains is list of update domain counts for each region.
	// Deprecated: VMSS does not allow specifying update domain count. With the deprecation of ets, only CountFaultDomains is required.
	CountUpdateDomains []DomainCount
	// CountFaultDomains is list of fault domain counts for each region.
	CountFaultDomains []DomainCount
	// MachineImages is the list of machine images that are understood by the controller. It maps
	// logical names and versions to provider-specific identifiers.
	MachineImages []MachineImages
	// MachineTypes is a list of machine types complete with provider specific information.
	MachineTypes []MachineType
	// CloudConfiguration contains config that controls which cloud to connect to.
	CloudConfiguration *CloudConfiguration
}

// CloudConfiguration contains detailed config for the cloud to connect to. Currently we only support selection of well-
// known Azure-instances by name, but this could be extended in future to support private clouds.
type CloudConfiguration struct {
	// Name is the name of the cloud to connect to, e.g. "AzurePublic" or "AzureChina".
	Name string
}

// DomainCount defines the region and the count for this domain count value.
type DomainCount struct {
	// Region is a region.
	Region string
	// Count is the count value for the respective domain count.
	Count int32
}

// MachineImages is a mapping from logical names and versions to provider-specific identifiers.
type MachineImages struct {
	// Name is the logical name of the machine image.
	Name string
	// Versions contains versions and a provider-specific identifier.
	Versions []MachineImageVersion
}

// MachineImageVersion contains a version and a provider-specific identifier.
type MachineImageVersion struct {
	// Version is the version of the image.
	Version string
	// URN is the uniform resource name of the image, it has the format 'publisher:offer:sku:version'.
	URN *string
	// SkipMarketplaceAgreement skips the marketplace agreement check when enabled.
	SkipMarketplaceAgreement *bool
	// ID is the Shared Image Gallery image id.
	ID *string
	// CommunityGalleryImageID is the Community Image Gallery image id, it has the format '/CommunityGalleries/myGallery/Images/myImage/Versions/myVersion'
	CommunityGalleryImageID *string
	// SharedGalleryImageID is the Shared Image Gallery image id, it has the format '/SharedGalleries/sharedGalleryName/Images/sharedGalleryImageName/Versions/sharedGalleryImageVersionName'
	SharedGalleryImageID *string
	// AcceleratedNetworking is an indicator if the image supports Azure accelerated networking.
	AcceleratedNetworking *bool
	// Architecture is the CPU architecture of the machine image.
	Architecture *string
}

// MachineType contains provider specific information to a machine type.
type MachineType struct {
	// Name is the name of the machine type.
	Name string
	// AcceleratedNetworking is an indicator if the machine type supports Azure accelerated networking.
	AcceleratedNetworking *bool
}

// The (currently) supported values for the names of clouds to use in the CloudConfiguration.
const (
	AzureChinaCloudName  string = "AzureChina"
	AzureGovCloudName    string = "AzureGovernment"
	AzurePublicCloudName string = "AzurePublic"
)

// The known prefixes in of region names for the various instances.
var (
	AzureGovRegionPrefixes   = []string{"usgov", "usdod", "ussec"}
	AzureChinaRegionPrefixes = []string{"china"}
)
