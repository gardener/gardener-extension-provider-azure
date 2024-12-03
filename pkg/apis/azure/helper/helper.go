// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"k8s.io/utils/ptr"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

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

// FindAvailabilitySetByPurpose takes a list of availability sets and tries to find the first entry
// whose purpose matches with the given purpose. If no such entry is found then an error will be
// returned.
func FindAvailabilitySetByPurpose(availabilitySets []api.AvailabilitySet, purpose api.Purpose) (*api.AvailabilitySet, error) {
	for _, availabilitySet := range availabilitySets {
		if availabilitySet.Purpose == purpose {
			return &availabilitySet, nil
		}
	}
	return nil, fmt.Errorf("cannot find availability set with purpose %q", purpose)
}

// FindMachineImage takes a list of machine images and tries to find the first entry
// whose name, version, architecture and zone matches with the given name, version, and zone. If no such entry is
// found then an error will be returned.
func FindMachineImage(machineImages []api.MachineImage, name, version string, architecture *string) (*api.MachineImage, error) {
	for _, machineImage := range machineImages {
		if machineImage.Architecture == nil {
			machineImage.Architecture = ptr.To(v1beta1constants.ArchitectureAMD64)
		}
		if machineImage.Name == name && machineImage.Version == version && ptr.Equal(architecture, machineImage.Architecture) {
			return &machineImage, nil
		}
	}
	return nil, fmt.Errorf("no machine image found with name %q, architecture %q and version %q", name, *architecture, version)
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

// FindImageFromCloudProfile takes a list of machine images, and the desired image name and version. It tries
// to find the image with the given name, architecture and version. If it cannot be found then an error
// is returned.
func FindImageFromCloudProfile(cloudProfileConfig *api.CloudProfileConfig, imageName, imageVersion string, architecture *string) (*api.MachineImage, error) {
	if cloudProfileConfig != nil {
		for _, machineImage := range cloudProfileConfig.MachineImages {
			if machineImage.Name != imageName {
				continue
			}
			for _, version := range machineImage.Versions {
				if imageVersion == version.Version && ptr.Equal(architecture, version.Architecture) {
					return &api.MachineImage{
						Name:                     imageName,
						Version:                  version.Version,
						AcceleratedNetworking:    version.AcceleratedNetworking,
						Architecture:             version.Architecture,
						SkipMarketplaceAgreement: version.SkipMarketplaceAgreement,
						Image: api.Image{
							URN:                     version.URN,
							ID:                      version.ID,
							SharedGalleryImageID:    version.SharedGalleryImageID,
							CommunityGalleryImageID: version.CommunityGalleryImageID,
						},
					}, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no machine image found with name %q, architecture %q and version %q", imageName, *architecture, imageVersion)
}

// IsVmoRequired determines if VMO is required. It is different from the condition in the infrastructure as this one depends on whether the infra controller
// has finished migrating the Availability sets.
func IsVmoRequired(infrastructureStatus *api.InfrastructureStatus) bool {
	return !infrastructureStatus.Zoned && (len(infrastructureStatus.AvailabilitySets) == 0 || infrastructureStatus.MigratingToVMO)
}

// HasShootVmoMigrationAnnotation determines if the passed Shoot annotations contain instruction to use VMO.
func HasShootVmoMigrationAnnotation(shootAnnotations map[string]string) bool {
	value, exists := shootAnnotations[azure.ShootVmoMigrationAnnotation]
	if exists && value == "true" {
		return true
	}
	return false
}

// HasShootVmoAlphaAnnotation determines if the passed Shoot annotations contain instruction to use VMO.
func HasShootVmoAlphaAnnotation(shootAnnotations map[string]string) bool {
	value, exists := shootAnnotations[azure.ShootVmoUsageAnnotation]
	if exists && value == "true" {
		return true
	}
	return false
}

// InfrastructureZoneToString translates the zone from the string format used in Gardener core objects to the int32 format used by the Azure provider extension.
func InfrastructureZoneToString(zone int32) string {
	return fmt.Sprintf("%d", zone)
}

// IsUsingSingleSubnetLayout returns true if the infrastructure configuration is using a network setup with a single subnet.
func IsUsingSingleSubnetLayout(config *api.InfrastructureConfig) bool {
	return len(config.Networks.Zones) == 0
}
