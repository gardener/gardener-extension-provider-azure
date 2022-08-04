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

package helper_test

import (
	"k8s.io/utils/pointer"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	profileURN              = "publisher:offer:sku:1.2.4"
	profileID               = "/subscription/image/id"
	profileCommunityImageId = "/CommunityGalleries/myGallery/Images/myImage/Versions/1.2.4"
	profileSharedImageId    = "/SharedGalleries/myGallery/Images/myImage/Versions/1.2.4"
)

var _ = Describe("Helper", func() {
	var (
		purpose          api.Purpose = "foo"
		purposeWrong     api.Purpose = "baz"
		urn              string      = "publisher:offer:sku:version"
		imageID          string      = "/image/id"
		communityImageId string      = "/CommunityGalleries/myGallery/Images/myImage/Versions/1.0.0"
		sharedImageId    string      = "/SharedGalleries/myGallery/Images/myImage/Versions/1.0.0"
		boolTrue                     = true
		boolFalse                    = false
		zone                         = "zone"
	)

	DescribeTable("#FindSubnetByPurposeAndZone",
		func(subnets []api.Subnet, purpose api.Purpose, zone *string, expectedSubnet *api.Subnet, expectErr bool) {
			_, subnet, err := FindSubnetByPurposeAndZone(subnets, purpose, zone)
			expectResults(subnet, expectedSubnet, err, expectErr)
		},

		Entry("list is nil", nil, purpose, nil, nil, true),
		Entry("empty list", []api.Subnet{}, purpose, nil, nil, true),
		Entry("entry not found", []api.Subnet{{Name: "bar", Purpose: purposeWrong}}, purpose, nil, nil, true),
		Entry("entry exists", []api.Subnet{{Name: "bar", Purpose: purpose}}, purpose, nil, &api.Subnet{Name: "bar", Purpose: purpose}, false),
		Entry("entry with zone", []api.Subnet{{Name: "bar", Purpose: purpose, Zone: &zone}}, purpose, &zone, &api.Subnet{Name: "bar", Purpose: purpose, Zone: &zone}, false),
		Entry("entry with zone not found", []api.Subnet{{Name: "bar", Purpose: purpose, Zone: &zone}}, purpose, pointer.String("badzone"), nil, true),
	)

	DescribeTable("#FindSecurityGroupByPurpose",
		func(securityGroups []api.SecurityGroup, purpose api.Purpose, expectedSecurityGroup *api.SecurityGroup, expectErr bool) {
			securityGroup, err := FindSecurityGroupByPurpose(securityGroups, purpose)
			expectResults(securityGroup, expectedSecurityGroup, err, expectErr)
		},

		Entry("list is nil", nil, purpose, nil, true),
		Entry("empty list", []api.SecurityGroup{}, purpose, nil, true),
		Entry("entry not found", []api.SecurityGroup{{Name: "bar", Purpose: purposeWrong}}, purpose, nil, true),
		Entry("entry exists", []api.SecurityGroup{{Name: "bar", Purpose: purpose}}, purpose, &api.SecurityGroup{Name: "bar", Purpose: purpose}, false),
	)

	DescribeTable("#FindRouteTableByPurpose",
		func(routeTables []api.RouteTable, purpose api.Purpose, expectedRouteTable *api.RouteTable, expectErr bool) {
			routeTable, err := FindRouteTableByPurpose(routeTables, purpose)
			expectResults(routeTable, expectedRouteTable, err, expectErr)
		},

		Entry("list is nil", nil, purpose, nil, true),
		Entry("empty list", []api.RouteTable{}, purpose, nil, true),
		Entry("entry not found", []api.RouteTable{{Name: "bar", Purpose: purposeWrong}}, purpose, nil, true),
		Entry("entry exists", []api.RouteTable{{Name: "bar", Purpose: purpose}}, purpose, &api.RouteTable{Name: "bar", Purpose: purpose}, false),
	)

	DescribeTable("#FindAvailabilitySetByPurpose",
		func(availabilitySets []api.AvailabilitySet, purpose api.Purpose, expectedAvailabilitySet *api.AvailabilitySet, expectErr bool) {
			availabilitySet, err := FindAvailabilitySetByPurpose(availabilitySets, purpose)
			expectResults(availabilitySet, expectedAvailabilitySet, err, expectErr)
		},

		Entry("list is nil", nil, purpose, nil, true),
		Entry("empty list", []api.AvailabilitySet{}, purpose, nil, true),
		Entry("entry not found", []api.AvailabilitySet{{ID: "bar", Purpose: purposeWrong}}, purpose, nil, true),
		Entry("entry exists", []api.AvailabilitySet{{ID: "bar", Purpose: purpose}}, purpose, &api.AvailabilitySet{ID: "bar", Purpose: purpose}, false),
	)

	DescribeTable("#FindMachineImage",
		func(machineImages []api.MachineImage, name, version string, architecture *string, expectedMachineImage *api.MachineImage, expectErr bool) {
			machineImage, err := FindMachineImage(machineImages, name, version, architecture)
			expectResults(machineImage, expectedMachineImage, err, expectErr)
		},

		Entry("list is nil", nil, "foo", "1.2.3", pointer.String("foo"), nil, true),
		Entry("empty list", []api.MachineImage{}, "foo", "1.2.3", pointer.String("foo"), nil, true),
		Entry("entry not found (no name)", []api.MachineImage{{Name: "bar", Version: "1.2.3", URN: &urn, Architecture: pointer.String("foo")}}, "foo", "1.2.3", pointer.String("foo"), nil, true),
		Entry("entry not found (no version)", []api.MachineImage{{Name: "bar", Version: "1.2.3", URN: &urn, Architecture: pointer.String("foo")}}, "bar", "1.2.4", pointer.String("foo"), nil, true),
		Entry("entry not found (no architecture)", []api.MachineImage{{Name: "bar", Version: "1.2.3", URN: &urn, Architecture: pointer.String("bar")}}, "bar", "1.2.3", pointer.String("foo"), nil, true),
		Entry("entry exists(urn)", []api.MachineImage{{Name: "bar", Version: "1.2.3", URN: &urn, Architecture: pointer.String("foo")}}, "bar", "1.2.3", pointer.String("foo"), &api.MachineImage{Name: "bar", Version: "1.2.3", URN: &urn, Architecture: pointer.String("foo")}, false),
		Entry("entry exists(id)", []api.MachineImage{{Name: "bar", Version: "1.2.3", ID: &imageID, Architecture: pointer.String("foo")}}, "bar", "1.2.3", pointer.String("foo"), &api.MachineImage{Name: "bar", Version: "1.2.3", ID: &imageID, Architecture: pointer.String("foo")}, false),
		Entry("entry exists(communityGalleryImageID)", []api.MachineImage{{Name: "bar", Version: "1.2.3", CommunityGalleryImageID: &communityImageId, Architecture: pointer.String("foo")}}, "bar", "1.2.3", pointer.String("foo"), &api.MachineImage{Name: "bar", Version: "1.2.3", CommunityGalleryImageID: &communityImageId, Architecture: pointer.String("foo")}, false),
		Entry("entry exists(sharedGalleryImageID)", []api.MachineImage{{Name: "bar", Version: "1.2.3", SharedGalleryImageID: &sharedImageId, Architecture: pointer.String("foo")}}, "bar", "1.2.3", pointer.String("foo"), &api.MachineImage{Name: "bar", Version: "1.2.3", SharedGalleryImageID: &sharedImageId, Architecture: pointer.String("foo")}, false),
		Entry("entry exists(accelerated networking active)", []api.MachineImage{{Name: "bar", Version: "1.2.3", URN: &urn, AcceleratedNetworking: &boolTrue, Architecture: pointer.String("foo")}}, "bar", "1.2.3", pointer.String("foo"), &api.MachineImage{Name: "bar", Version: "1.2.3", URN: &urn, AcceleratedNetworking: &boolTrue, Architecture: pointer.String("foo")}, false),
		Entry("entry exists(accelerated networking inactive)", []api.MachineImage{{Name: "bar", Version: "1.2.3", URN: &urn, AcceleratedNetworking: &boolFalse, Architecture: pointer.String("foo")}}, "bar", "1.2.3", pointer.String("foo"), &api.MachineImage{Name: "bar", Version: "1.2.3", URN: &urn, AcceleratedNetworking: &boolFalse, Architecture: pointer.String("foo")}, false),
	)

	DescribeTable("#FindDomainCountByRegion",
		func(domainCounts []api.DomainCount, region string, expectedCount int, expectErr bool) {
			count, err := FindDomainCountByRegion(domainCounts, region)
			expectResults(count, int32(expectedCount), err, expectErr)
		},

		Entry("list is nil", nil, "foo", 0, true),
		Entry("empty list", []api.DomainCount{}, "foo", 0, true),
		Entry("entry not found", []api.DomainCount{{Region: "bar", Count: int32(1)}}, "foo", 0, true),
		Entry("entry exists", []api.DomainCount{{Region: "bar", Count: int32(1)}}, "bar", 1, false),
	)

	DescribeTable("#FindImage",
		func(profileImages []api.MachineImages, imageName, version string, architecture *string, expectedImage *api.MachineImage) {
			cfg := &api.CloudProfileConfig{}
			cfg.MachineImages = profileImages
			image, err := FindImageFromCloudProfile(cfg, imageName, version, architecture)

			Expect(image).To(Equal(expectedImage))
			if expectedImage != nil {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		},

		Entry("list is nil", nil, "ubuntu", "1", pointer.String("foo"), nil),

		Entry("profile empty list", []api.MachineImages{}, "ubuntu", "1", pointer.String("foo"), nil),
		Entry("profile entry not found (image does not exist)", makeProfileMachineImages("debian", "1", "3", "5", "7", pointer.String("foo")), "ubuntu", "1", pointer.String("foo"), nil),
		Entry("profile entry not found (version does not exist)", makeProfileMachineImages("ubuntu", "2", "4", "6", "7", pointer.String("foo")), "ubuntu", "1", pointer.String("foo"), nil),
		Entry("profile entry not found (no architecture)", makeProfileMachineImages("ubuntu", "2", "4", "6", "7", pointer.String("bar")), "ubuntu", "2", pointer.String("foo"), nil),
		Entry("profile entry(urn)", makeProfileMachineImages("ubuntu", "1", "3", "5", "6", pointer.String("foo")), "ubuntu", "1", pointer.String("foo"), &api.MachineImage{Name: "ubuntu", Version: "1", URN: &profileURN, Architecture: pointer.String("foo")}),
		Entry("profile entry(id)", makeProfileMachineImages("ubuntu", "1", "3", "5", "6", pointer.String("foo")), "ubuntu", "3", pointer.String("foo"), &api.MachineImage{Name: "ubuntu", Version: "3", ID: &profileID, Architecture: pointer.String("foo")}),
		Entry("profile entry(communiyGalleryId)", makeProfileMachineImages("ubuntu", "1", "3", "5", "6", pointer.String("foo")), "ubuntu", "5", pointer.String("foo"), &api.MachineImage{Name: "ubuntu", Version: "5", CommunityGalleryImageID: &profileCommunityImageId, Architecture: pointer.String("foo")}),
		Entry("profile entry(sharedGalleryId)", makeProfileMachineImages("ubuntu", "1", "3", "5", "6", pointer.String("foo")), "ubuntu", "6", pointer.String("foo"), &api.MachineImage{Name: "ubuntu", Version: "6", SharedGalleryImageID: &profileSharedImageId, Architecture: pointer.String("foo")}),

		Entry("valid image reference, only urn", makeProfileMachineImageWithURNandIDandCommunityGalleryIDandSharedGalleryImageID("ubuntu", "1", &profileURN, nil, nil, nil, pointer.String("foo")),
			"ubuntu", "1", pointer.String("foo"), &api.MachineImage{Name: "ubuntu", Version: "1", URN: &profileURN, Architecture: pointer.String("foo")}),
		Entry("valid image reference, only id", makeProfileMachineImageWithURNandIDandCommunityGalleryIDandSharedGalleryImageID("ubuntu", "1", nil, &profileID, nil, nil, pointer.String("foo")),
			"ubuntu", "1", pointer.String("foo"), &api.MachineImage{Name: "ubuntu", Version: "1", ID: &profileID, Architecture: pointer.String("foo")}),
		Entry("valid image reference, only communityGalleryImageID", makeProfileMachineImageWithURNandIDandCommunityGalleryIDandSharedGalleryImageID("ubuntu", "1", nil, nil, &profileCommunityImageId, nil, pointer.String("foo")),
			"ubuntu", "1", pointer.String("foo"), &api.MachineImage{Name: "ubuntu", Version: "1", CommunityGalleryImageID: &profileCommunityImageId, Architecture: pointer.String("foo")}),
		Entry("valid image reference, only sharedGalleryImageID", makeProfileMachineImageWithURNandIDandCommunityGalleryIDandSharedGalleryImageID("ubuntu", "1", nil, nil, nil, &profileSharedImageId, pointer.String("foo")),
			"ubuntu", "1", pointer.String("foo"), &api.MachineImage{Name: "ubuntu", Version: "1", SharedGalleryImageID: &profileSharedImageId, Architecture: pointer.String("foo")}),
	)

	DescribeTable("#IsVmoRequired",
		func(zoned bool, availabilitySet *api.AvailabilitySet, expectedVmoRequired bool) {
			var infrastructureStatus = &api.InfrastructureStatus{
				Zoned: zoned,
			}
			if availabilitySet != nil {
				infrastructureStatus.AvailabilitySets = append(infrastructureStatus.AvailabilitySets, *availabilitySet)
			}

			Expect(IsVmoRequired(infrastructureStatus)).To(Equal(expectedVmoRequired))
		},
		Entry("should require a VMO", false, nil, true),
		Entry("should not require VMO for zoned cluster", true, nil, false),
		Entry("should not require VMO for a cluster with primary availabilityset (non zoned)", false, &api.AvailabilitySet{
			ID:      "/my/azure/availabilityset/id",
			Name:    "my-availabilityset",
			Purpose: api.PurposeNodes,
		}, false),
	)

	DescribeTable("#HasShootVmoAlphaAnnotation",
		func(hasVmoAnnotaion, hasCorrectVmoAnnotationValue, expectedResult bool) {
			var annotations = map[string]string{}
			if hasVmoAnnotaion {
				annotations[azure.ShootVmoUsageAnnotation] = "some-arbitrary-value"
			}
			if hasCorrectVmoAnnotationValue {
				annotations[azure.ShootVmoUsageAnnotation] = "true"
			}
			Expect(HasShootVmoAlphaAnnotation(annotations)).To(Equal(expectedResult))
		},
		Entry("should return true as shoot annotations contain vmo alpha annotation with value true", true, true, true),
		Entry("should return false as shoot annotations contain vmo alpha annotation with wrong value", true, false, false),
		Entry("should return false as shoot annotations do not contain vmo alpha annotation", false, false, false),
	)
})

func makeProfileMachineImages(name, urnVersion, idVersion, communityGalleryImageIdVersion string, sharedGalleryImageIdVersion string, architecture *string) []api.MachineImages {
	return []api.MachineImages{
		{
			Name: name,
			Versions: []api.MachineImageVersion{
				{
					Version:      urnVersion,
					URN:          &profileURN,
					Architecture: architecture,
				},
				{
					Version:      idVersion,
					ID:           &profileID,
					Architecture: architecture,
				},
				{
					Version:                 communityGalleryImageIdVersion,
					CommunityGalleryImageID: &profileCommunityImageId,
					Architecture:            architecture,
				},
				{
					Version:              sharedGalleryImageIdVersion,
					SharedGalleryImageID: &profileSharedImageId,
					Architecture:         architecture,
				},
			},
		},
	}
}

func makeProfileMachineImageWithURNandIDandCommunityGalleryIDandSharedGalleryImageID(name, version string, urn, id, communityGalleryImageID, sharedGalleryImageID, architecture *string) []api.MachineImages {
	return []api.MachineImages{
		{
			Name: name,
			Versions: []api.MachineImageVersion{
				{
					Version:                 version,
					URN:                     urn,
					ID:                      id,
					CommunityGalleryImageID: communityGalleryImageID,
					SharedGalleryImageID:    sharedGalleryImageID,
					Architecture:            architecture,
				},
			},
		},
	}
}

func expectResults(result, expected interface{}, err error, expectErr bool) {
	if !expectErr {
		Expect(result).To(Equal(expected))
		Expect(err).NotTo(HaveOccurred())
	} else {
		Expect(result).To(BeZero())
		Expect(err).To(HaveOccurred())
	}
}
