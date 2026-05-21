// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator

import (
	"context"
	"fmt"
	"slices"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

// NewCloudProfileMutator returns a new instance of a CloudProfile mutator.
func NewCloudProfileMutator(mgr manager.Manager) extensionswebhook.Mutator {
	return &cloudProfile{
		client:  mgr.GetClient(),
		decoder: serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
	}
}

type cloudProfile struct {
	client  client.Client
	decoder runtime.Decoder
}

// Mutate mutates the given CloudProfile object.
func (p *cloudProfile) Mutate(_ context.Context, newObj, _ client.Object) error {
	profile, ok := newObj.(*gardencorev1beta1.CloudProfile)
	if !ok {
		return fmt.Errorf("wrong object type %T", newObj)
	}

	// Skip mutation if CloudProfile is being deleted or when no capabilities used in that profile
	if profile.DeletionTimestamp != nil || profile.Spec.ProviderConfig == nil || len(profile.Spec.MachineCapabilities) == 0 {
		return nil
	}

	specConfig := &v1alpha1.CloudProfileConfig{}
	if _, _, err := p.decoder.Decode(profile.Spec.ProviderConfig.Raw, nil, specConfig); err != nil {
		return fmt.Errorf("could not decode providerConfig of cloudProfile for '%s': %w", profile.Name, err)
	}

	mutateMachineImageCapabilityFlavors(profile.Spec.MachineImages, specConfig)
	return nil
}

// mutateMachineImageCapabilityFlavors populates capabilityFlavors on the given machineImages
// slice from the provider config. Used by both CloudProfile and NamespacedCloudProfile mutators.
func mutateMachineImageCapabilityFlavors(machineImages []gardencorev1beta1.MachineImage, config *v1alpha1.CloudProfileConfig) {
	// Find the corresponding machine image in the CloudProfile
	for _, providerMachineImage := range config.MachineImages {
		imageIdx := slices.IndexFunc(machineImages, func(mi gardencorev1beta1.MachineImage) bool {
			return mi.Name == providerMachineImage.Name
		})
		if imageIdx == -1 {
			continue
		}

		// Group provider versions by version string (old format may have multiple entries per version)
		groupedVersions := helper.GroupV1alpha1VersionsByVersionString(providerMachineImage.Versions)

		for versionStr, providerVersions := range groupedVersions {
			// Find the corresponding version in the CloudProfile's machine image
			versionIdx := slices.IndexFunc(machineImages[imageIdx].Versions, func(miv gardencorev1beta1.MachineImageVersion) bool {
				return miv.Version == versionStr
			})
			if versionIdx == -1 {
				continue
			}

			// Check if any version entry uses new format (capabilityFlavors)
			// If so, use that; otherwise convert old format entries to capability flavors
			var capabilityFlavors []gardencorev1beta1.MachineImageFlavor
			for _, pv := range providerVersions {
				if len(pv.CapabilityFlavors) > 0 {
					// New format: use capabilityFlavors directly
					capabilityFlavors = convertCapabilityFlavors(pv.CapabilityFlavors)
					break
				}
			}

			if len(capabilityFlavors) == 0 {
				// Old format: convert all image+architecture entries to capability flavors
				capabilityFlavors = convertVersionsToCapabilityFlavors(providerVersions)
			}

			machineImages[imageIdx].Versions[versionIdx].CapabilityFlavors = capabilityFlavors
		}
	}
}

// convertVersionsToCapabilityFlavors converts old format (image with architecture and acceleratedNetworking) entries to capability flavors.
// It collects unique capability combinations from all version entries and creates a capability flavor for each.
// Note: A similar function exists in helper.go for internal API types that also preserves image references.
// This version only extracts unique capability combinations for CloudProfile spec mutation.
func convertVersionsToCapabilityFlavors(versions []v1alpha1.MachineImageVersion) []gardencorev1beta1.MachineImageFlavor {
	// capabilityKey is used to track unique capability combinations
	type capabilityKey struct {
		architecture          string
		acceleratedNetworking bool
	}

	// Collect unique capability combinations from all version entries
	capabilitySet := make(map[capabilityKey]struct{})
	for _, version := range versions {
		if version.Image != (v1alpha1.Image{}) {
			arch := ptr.Deref(version.Architecture, v1beta1constants.ArchitectureAMD64)
			// AcceleratedNetworking defaults to false if not specified to maintain backward compatibility
			acceleratedNetworking := ptr.Deref(version.AcceleratedNetworking, false)
			capabilitySet[capabilityKey{architecture: arch, acceleratedNetworking: acceleratedNetworking}] = struct{}{}
		}
	}

	// Create a capability flavor for each unique capability combination
	capabilityFlavors := make([]gardencorev1beta1.MachineImageFlavor, 0, len(capabilitySet))
	for key := range capabilitySet {
		capabilities := gardencorev1beta1.Capabilities{
			v1beta1constants.ArchitectureName: []string{key.architecture},
		}

		// Convert AcceleratedNetworking to networking capability
		if key.acceleratedNetworking {
			capabilities[azure.CapabilityNetworkName] = []string{azure.CapabilityNetworkBasic, azure.CapabilityNetworkAccelerated}
		} else {
			capabilities[azure.CapabilityNetworkName] = []string{azure.CapabilityNetworkBasic}
		}

		capabilityFlavors = append(capabilityFlavors, gardencorev1beta1.MachineImageFlavor{
			Capabilities: capabilities,
		})
	}

	return capabilityFlavors
}

// convertCapabilityFlavors converts provider capability flavors to CloudProfile capability flavors
func convertCapabilityFlavors(providerFlavors []v1alpha1.MachineImageFlavor) []gardencorev1beta1.MachineImageFlavor {
	capabilityFlavors := make([]gardencorev1beta1.MachineImageFlavor, 0, len(providerFlavors))
	for _, providerFlavor := range providerFlavors {
		capabilityFlavors = append(capabilityFlavors, gardencorev1beta1.MachineImageFlavor{
			Capabilities: providerFlavor.Capabilities,
		})
	}
	return capabilityFlavors
}
