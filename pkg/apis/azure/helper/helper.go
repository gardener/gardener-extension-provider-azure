// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"k8s.io/utils/ptr"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

// NormalizeCapabilityDefinitions ensures that capability definitions always include at least
// the architecture capability. This allows all downstream code to assume capabilities are always present,
// eliminating the need for conditional logic based on whether capabilities are defined.
func NormalizeCapabilityDefinitions(capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) []gardencorev1beta1.CapabilityDefinition {
	if len(capabilityDefinitions) > 0 {
		return capabilityDefinitions
	}
	return []gardencorev1beta1.CapabilityDefinition{
		{
			Name:   v1beta1constants.ArchitectureName,
			Values: []string{v1beta1constants.ArchitectureAMD64, v1beta1constants.ArchitectureARM64},
		},
		{
			Name:   azure.CapabilityNetworkName,
			Values: []string{azure.CapabilityNetworkAccelerated, azure.CapabilityNetworkBasic},
		},
	}
}

// NormalizeMachineTypeCapabilities ensures that machine type capabilities include the architecture
// capability. This transforms the legacy architecture-based selection into capability-based selection.
// The architecture is determined in the following priority order:
// 1. If capabilities already has architecture, use it as-is
// 2. If capabilityDefinitions has exactly one architecture value, use that value
// 3. Otherwise, use workerArchitecture (defaulting to amd64)
func NormalizeMachineTypeCapabilities(capabilities gardencorev1beta1.Capabilities, workerArchitecture *string, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) gardencorev1beta1.Capabilities {
	if capabilities == nil {
		capabilities = make(gardencorev1beta1.Capabilities)
	}
	// If architecture capability is already present, return as-is
	if _, hasArch := capabilities[v1beta1constants.ArchitectureName]; hasArch {
		return capabilities
	}

	// Check if capabilityDefinitions has exactly one architecture value
	for _, def := range capabilityDefinitions {
		if def.Name == v1beta1constants.ArchitectureName && len(def.Values) == 1 {
			capabilities[v1beta1constants.ArchitectureName] = []string{def.Values[0]}
			return capabilities
		}
	}

	// Fall back to workerArchitecture or default
	arch := ptr.Deref(workerArchitecture, v1beta1constants.ArchitectureAMD64)
	capabilities[v1beta1constants.ArchitectureName] = []string{arch}
	return capabilities
}

// FindSubnetByPurposeAndZone takes a list of subnets and tries to find the first entry whose purpose matches with the given purpose.
// Optionally, if the zone argument is not nil, the Zone field of a candidate subnet must match that value.
// FindSubnetByPurposeAndZone returns the index of the subnet in the array and the subnet object.
// If no such entry is found then an error will be returned.
func FindSubnetByPurposeAndZone(subnets []api.Subnet, purpose api.Purpose, zone *string) (int, *api.Subnet, error) {
	for index, subnet := range subnets {
		if subnet.Purpose == purpose && (zone == nil || (subnet.Zone != nil && *subnet.Zone == *zone)) {
			return index, &subnet, nil
		}
	}

	errMsg := fmt.Sprintf("cannot find subnet with purpose %q", purpose)
	if zone != nil {
		errMsg += fmt.Sprintf(" and zone %q", *zone)
	}
	return 0, nil, fmt.Errorf("%s", errMsg)
}

// FindSecurityGroupByPurpose takes a list of security groups and tries to find the first entry
// whose purpose matches with the given purpose. If no such entry is found then an error will be
// returned.
func FindSecurityGroupByPurpose(securityGroups []api.SecurityGroup, purpose api.Purpose) (*api.SecurityGroup, error) {
	for _, securityGroup := range securityGroups {
		if securityGroup.Purpose == purpose {
			return &securityGroup, nil
		}
	}
	return nil, fmt.Errorf("cannot find security group with purpose %q", purpose)
}

// FindRouteTableByPurpose takes a list of route tables and tries to find the first entry
// whose purpose matches with the given purpose. If no such entry is found then an error will be
// returned.
func FindRouteTableByPurpose(routeTables []api.RouteTable, purpose api.Purpose) (*api.RouteTable, error) {
	for _, routeTable := range routeTables {
		if routeTable.Purpose == purpose {
			return &routeTable, nil
		}
	}
	return nil, fmt.Errorf("cannot find route table with purpose %q", purpose)
}

// FindDomainCountByRegion takes a region and the domain counts and finds the count for the given region.
func FindDomainCountByRegion(domainCounts []api.DomainCount, region string) (int32, error) {
	for _, domainCount := range domainCounts {
		if domainCount.Region == region {
			return domainCount.Count, nil
		}
	}
	return 0, fmt.Errorf("could not find a domain count for region %s", region)
}

// FindImageInCloudProfile takes a list of machine images and tries to find the first entry
// whose name, version and capabilities matches with the machineTypeCapabilities. If no such entry is
// found then an error will be returned.
// Note: capabilityDefinitions and machineTypeCapabilities are expected to be normalized
// by the caller using NormalizeCapabilityDefinitions() and NormalizeMachineTypeCapabilities()
func FindImageInCloudProfile(
	cloudProfileConfig *api.CloudProfileConfig,
	imageName, imageVersion string,
	machineTypeCapabilities gardencorev1beta1.Capabilities,
	capabilityDefinitions []gardencorev1beta1.CapabilityDefinition,
) (*api.MachineImageFlavor, error) {
	if cloudProfileConfig == nil {
		return nil, fmt.Errorf("cloud profile config is nil")
	}

	for _, machineImage := range cloudProfileConfig.MachineImages {
		if machineImage.Name != imageName {
			continue
		}

		// Collect all versions with matching version string (mixed format support)
		var matchingVersions []api.MachineImageVersion
		for _, version := range machineImage.Versions {
			if imageVersion == version.Version {
				matchingVersions = append(matchingVersions, version)
			}
		}

		if len(matchingVersions) == 0 {
			continue
		}

		// Convert old format (image with architecture) versions to capability flavors if required
		// as there may be multiple version entries for the same version with different architectures.
		capabilityFlavors := convertLegacyVersionsToCapabilityFlavors(matchingVersions)

		if len(capabilityFlavors) > 0 {
			bestMatch, err := worker.FindBestImageFlavor(capabilityFlavors, machineTypeCapabilities, capabilityDefinitions)
			if err != nil {
				return nil, fmt.Errorf("could not determine best flavor: %w", err)
			}
			return &bestMatch, nil
		}
	}

	return nil, fmt.Errorf("could not find an image for name %q and version %q that supports %v", imageName, imageVersion, machineTypeCapabilities)
}

// FindImageInWorkerStatus takes a list of machine images from the worker status and tries to find the first entry
// whose name, version, architecture, capabilities and zone matches with the given ones. If no such entry is
// found then an error will be returned.
func FindImageInWorkerStatus(machineImages []api.MachineImage, name string, version string, architecture *string, machineCapabilities gardencorev1beta1.Capabilities, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) (*api.MachineImage, error) {
	// If no capabilityDefinitions are specified, return the (legacy) architecture format field as no Capabilities are used.
	if len(capabilityDefinitions) == 0 {
		for _, statusMachineImage := range machineImages {
			if statusMachineImage.Architecture == nil {
				statusMachineImage.Architecture = ptr.To(v1beta1constants.ArchitectureAMD64)
			}
			if statusMachineImage.Name == name && statusMachineImage.Version == version && ptr.Equal(architecture, statusMachineImage.Architecture) {
				return &statusMachineImage, nil
			}
		}
		return nil, worker.ErrorMachineImageNotFound(name, version, *architecture)
	}

	// If capabilityDefinitions are specified, we need to find the best matching capability set.
	for _, statusMachineImage := range machineImages {
		if statusMachineImage.Name != name || statusMachineImage.Version != version {
			continue
		}

		// Normalize status image capabilities: if the status has Architecture but no Capabilities,
		// convert Architecture and AcceleratedNetworking to Capabilities for compatibility checking
		statusCapabilities := statusMachineImage.Capabilities
		if len(statusCapabilities) == 0 && statusMachineImage.Architecture != nil {
			statusCapabilities = gardencorev1beta1.Capabilities{
				v1beta1constants.ArchitectureName: []string{*statusMachineImage.Architecture},
			}
			// Convert AcceleratedNetworking to networking capability (same logic as convertLegacyVersionsToCapabilityFlavors)
			if ptr.Deref(statusMachineImage.AcceleratedNetworking, false) {
				statusCapabilities[azure.CapabilityNetworkName] = []string{azure.CapabilityNetworkBasic, azure.CapabilityNetworkAccelerated}
			} else {
				statusCapabilities[azure.CapabilityNetworkName] = []string{azure.CapabilityNetworkBasic}
			}
		}

		if gardencorev1beta1helper.AreCapabilitiesCompatible(statusCapabilities, machineCapabilities, capabilityDefinitions) {
			return &statusMachineImage, nil
		}
	}
	return nil, fmt.Errorf("no machine image found for image %q with version %q and capabilities %v", name, version, machineCapabilities)
}

// convertLegacyVersionsToCapabilityFlavors converts old format (image with architecture and acceleratedNetworking) versions
// to capability flavors for mixed format support.
func convertLegacyVersionsToCapabilityFlavors(versions []api.MachineImageVersion) []api.MachineImageFlavor {
	var capabilityFlavors []api.MachineImageFlavor
	for _, version := range versions {
		if len(version.CapabilityFlavors) == 0 {
			// Convert legacy format to capability flavors
			arch := ptr.Deref(version.Architecture, v1beta1constants.ArchitectureAMD64)
			capabilities := gardencorev1beta1.Capabilities{
				v1beta1constants.ArchitectureName: []string{arch},
			}

			// Convert AcceleratedNetworking to networking capability
			if ptr.Deref(version.AcceleratedNetworking, false) {
				capabilities[azure.CapabilityNetworkName] = []string{azure.CapabilityNetworkBasic, azure.CapabilityNetworkAccelerated}
			} else {
				capabilities[azure.CapabilityNetworkName] = []string{azure.CapabilityNetworkBasic}
			}

			capabilityFlavors = append(capabilityFlavors, api.MachineImageFlavor{
				Image:                    version.Image,
				Capabilities:             capabilities,
				SkipMarketplaceAgreement: version.SkipMarketplaceAgreement,
			})
		} else {
			capabilityFlavors = append(capabilityFlavors, version.CapabilityFlavors...)
		}
	}
	return capabilityFlavors
}

// GroupVersionsByVersionString groups all provider versions by their version string.
// This is needed because the old format may have multiple entries for the same version
// with different architectures (mixed format support).
func GroupVersionsByVersionString(versions []api.MachineImageVersion) map[string][]api.MachineImageVersion {
	result := make(map[string][]api.MachineImageVersion)
	for _, v := range versions {
		result[v.Version] = append(result[v.Version], v)
	}
	return result
}

// GroupV1alpha1VersionsByVersionString groups all v1alpha1 provider versions by their version string.
// This is needed because the old format may have multiple entries for the same version
// with different architectures (mixed format support).
func GroupV1alpha1VersionsByVersionString(versions []v1alpha1.MachineImageVersion) map[string][]v1alpha1.MachineImageVersion {
	result := make(map[string][]v1alpha1.MachineImageVersion)
	for _, v := range versions {
		result[v.Version] = append(result[v.Version], v)
	}
	return result
}

// IsVmoRequired determines if VMO is required. It is different from the condition in the infrastructure as this one depends on whether the infra controller
// has finished migrating the Availability sets.
func IsVmoRequired(infrastructureStatus *api.InfrastructureStatus) bool {
	return !infrastructureStatus.Zoned
}

// InfrastructureZoneToString translates the zone from the string format used in Gardener core objects to the int32 format used by the Azure provider extension.
func InfrastructureZoneToString(zone int32) string {
	return fmt.Sprintf("%d", zone)
}

// IsUsingSingleSubnetLayout returns true if the infrastructure configuration is using a network setup with a single subnet.
func IsUsingSingleSubnetLayout(config *api.InfrastructureConfig) bool {
	return len(config.Networks.Zones) == 0
}
