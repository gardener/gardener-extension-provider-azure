// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package helper

import (
	"fmt"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
)

// FindSubnetByPurpose takes a list of subnets and tries to find the first entry
// whose purpose matches with the given purpose. If no such entry is found then an error will be
// returned.
func FindSubnetByPurpose(subnets []api.Subnet, purpose api.Purpose) (*api.Subnet, error) {
	for _, subnet := range subnets {
		if subnet.Purpose == purpose {
			return &subnet, nil
		}
	}
	return nil, fmt.Errorf("cannot find subnet with purpose %q", purpose)
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
// whose name, version, and zone matches with the given name, version, and zone. If no such entry is
// found then an error will be returned.
func FindMachineImage(machineImages []api.MachineImage, name, version string) (*api.MachineImage, error) {
	for _, machineImage := range machineImages {
		if machineImage.Name == name && machineImage.Version == version {
			return &machineImage, nil
		}
	}
	return nil, fmt.Errorf("no machine image with name %q, version %q found", name, version)
}

// FindDomainCountByRegion takes a region and the domain counts and finds the count for the given region.
func FindDomainCountByRegion(domainCounts []api.DomainCount, region string) (int, error) {
	for _, domainCount := range domainCounts {
		if domainCount.Region == region {
			return domainCount.Count, nil
		}
	}
	return 0, fmt.Errorf("could not find a domain count for region %s", region)
}

// FindImageFromCloudProfile takes a list of machine images, and the desired image name and version. It tries
// to find the image with the given name and version. If it cannot be found then an error
// is returned.
func FindImageFromCloudProfile(cloudProfileConfig *api.CloudProfileConfig, imageName, imageVersion, region string) (*api.MachineImage, error) {
	if cloudProfileConfig != nil {
		for _, machineImage := range cloudProfileConfig.MachineImages {
			if machineImage.Name != imageName {
				continue
			}
			for _, version := range machineImage.Versions {
				if imageVersion == version.Version {
					imageID := findMachineImageIDForRegion(version, region)
					if imageID == nil && version.URN == nil {
						return nil, fmt.Errorf("image with name %q and version %q is not supported in region %q", imageName, imageVersion, region)
					}
					return &api.MachineImage{
						Name:    imageName,
						Version: version.Version,
						URN:     version.URN,
						ID:      imageID,
					}, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("could not find an image for name %q in version %q", imageName, imageVersion)
}

func findMachineImageIDForRegion(image api.MachineImageVersion, region string) *string {
	for _, imageRegion := range image.Regions {
		if imageRegion.Name == region {
			return &imageRegion.ID
		}
	}
	return nil
}
