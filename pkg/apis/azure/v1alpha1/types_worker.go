// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

	// DiagnosticsProfile specifies boot diagnostic options.
	// +optional
	DiagnosticsProfile *DiagnosticsProfile `json:"diagnosticsProfile,omitempty"`

	// DataVolumes contains configuration for the additional disks attached to VMs.
	// +optional
	DataVolumes []DataVolume `json:"dataVolumes,omitempty"`
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
	// AcceleratedNetworking is an indicator if the image supports Azure accelerated networking.
	// +optional
	AcceleratedNetworking *bool `json:"acceleratedNetworking,omitempty"`
	// Architecture is the CPU architecture of the machine image.
	// +optional
	Architecture *string `json:"architecture,omitempty"`
	// SkipMarketplaceAgreement skips the marketplace agreement check when enabled.
	// +optional
	SkipMarketplaceAgreement *bool `json:"skipMarketplaceAgreement,omitempty"`
	// Image identifies the azure image.
	Image `json:",inline"`
}

// Image identifies the azure image.
type Image struct {
	// URN is the uniform resource name of the image, it has the format 'publisher:offer:sku:version'.
	// +optional
	URN *string `json:"urn,omitempty"`
	// ID is the VM image ID.
	// +optional
	ID *string `json:"id,omitempty"`
	// CommunityGalleryImageID is the Community Image Gallery image id.
	// +optional
	CommunityGalleryImageID *string `json:"communityGalleryImageID,omitempty"`
	// SharedGalleryImageID is the Shared Image Gallery image id.
	// +optional
	SharedGalleryImageID *string `json:"sharedGalleryImageID,omitempty"`
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

// DiagnosticsProfile specifies boot diagnostic options.
type DiagnosticsProfile struct {
	// Enabled configures boot diagnostics to be stored or not.
	Enabled bool `json:"enabled,omitempty"`
	// StorageURI is the URI of the storage account to use for storing console output and screenshot.
	// If not specified azure managed storage will be used.
	StorageURI *string `json:"storageURI,omitempty"`
}

// DataVolume contains configuration for data volumes attached to VMs.
type DataVolume struct {
	// Name is the name of the data volume this configuration applies to.
	Name string `json:"name"`
	// ImageRef defines the dataVolume source image.
	// +optional
	ImageRef *Image `json:"imageRef,omitempty"`
}
