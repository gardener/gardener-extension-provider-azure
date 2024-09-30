// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkerConfig contains configuration settings for the worker nodes.
type WorkerConfig struct {
	metav1.TypeMeta
	// NodeTemplate contains resource information of the machine which is used by Cluster Autoscaler to generate nodeTemplate during scaling a nodeGroup from zero.
	NodeTemplate *extensionsv1alpha1.NodeTemplate

	// DiagnosticsProfile specifies boot diagnostic options.
	DiagnosticsProfile *DiagnosticsProfile

	// DataVolumes contains configuration for the additional disks attached to VMs.
	DataVolumes []DataVolume
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkerStatus contains information about created worker resources.
type WorkerStatus struct {
	metav1.TypeMeta

	// MachineImages is a list of machine images that have been used in this worker. Usually, the extension controller
	// gets the mapping from name/version to the provider-specific machine image data in its componentconfig. However, if
	// a version that is still in use gets removed from this componentconfig it cannot reconcile anymore existing `Worker`
	// resources that are still using this version. Hence, it stores the used versions in the provider status to ensure
	// reconciliation is possible.
	MachineImages []MachineImage

	// VmoDependencies is a list of external VirtualMachineScaleSet Orchestration Mode VM (VMO) dependencies.
	VmoDependencies []VmoDependency
}

// MachineImage is a mapping from logical names and versions to provider-specific machine image data.
type MachineImage struct {
	// Name is the logical name of the machine image.
	Name string
	// Version is the logical version of the machine image.
	Version string
	// AcceleratedNetworking is an indicator if the image supports Azure accelerated networking.
	AcceleratedNetworking *bool
	// Architecture is the CPU architecture of the machine image.
	Architecture *string
	// SkipMarketplaceAgreement skips the marketplace agreement check when enabled.
	SkipMarketplaceAgreement *bool
	// Image identifies the azure image.
	Image
}

// Image identifies the azure image.
type Image struct {
	// URN is the uniform resource name of the image, it has the format 'publisher:offer:sku:version'.
	URN *string
	// ID is the VM image ID.
	ID *string
	// CommunityGalleryImageID is the Community Image Gallery image id.
	CommunityGalleryImageID *string
	// SharedGalleryImageID is the Shared Image Gallery image id.
	SharedGalleryImageID *string
}

// VmoDependency is dependency reference for a workerpool to a VirtualMachineScaleSet Orchestration Mode VM (VMO).
type VmoDependency struct {
	// PoolName is the name of the worker pool to which the VMO belong to.
	PoolName string
	// ID is the id of the VMO resource on Azure.
	ID string
	// Name is the name of the VMO resource on Azure.
	Name string
}

// DiagnosticsProfile specifies boot diagnostic options.
type DiagnosticsProfile struct {
	// Enabled configures boot diagnostics to be stored or not.
	Enabled bool
	// StorageURI is the URI of the storage account to use for storing console output and screenshot.
	// If not specified azure managed storage will be used.
	StorageURI *string
}

// DataVolume contains configuration for data volumes attached to VMs.
type DataVolume struct {
	// Name is the name of the data volume this configuration applies to.
	Name string
	// ImageRef defines the dataVolume source image.
	ImageRef *Image
}
