// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
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
		urn                          = "publisher:offer:sku:version"
		imageID                      = "/image/id"
		communityImageId             = "/CommunityGalleries/myGallery/Images/myImage/Versions/1.0.0"
		sharedImageId                = "/SharedGalleries/myGallery/Images/myImage/Versions/1.0.0"
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
		Entry("entry with zone not found", []api.Subnet{{Name: "bar", Purpose: purpose, Zone: &zone}}, purpose, ptr.To("badzone"), nil, true),
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

	DescribeTable("#FindMachineImage",
		func(machineImages []api.MachineImage, name, version string, architecture *string, expectedMachineImage *api.MachineImage, expectErr bool) {
			machineImage, err := FindMachineImage(machineImages, name, version, architecture)
			expectResults(machineImage, expectedMachineImage, err, expectErr)
		},

		Entry("list is nil", nil, "foo", "1.2.3", ptr.To("foo"), nil, true),
		Entry("empty list", []api.MachineImage{}, "foo", "1.2.3", ptr.To("foo"), nil, true),
		Entry("entry not found (no name)", []api.MachineImage{{Name: "bar", Version: "1.2.3", Image: api.Image{URN: &urn}, Architecture: ptr.To("foo")}}, "foo", "1.2.3", ptr.To("foo"), nil, true),
		Entry("entry not found (no version)", []api.MachineImage{{Name: "bar", Version: "1.2.3", Image: api.Image{URN: &urn}, Architecture: ptr.To("foo")}}, "bar", "1.2.4", ptr.To("foo"), nil, true),
		Entry("entry not found (no architecture)", []api.MachineImage{{Name: "bar", Version: "1.2.3", Image: api.Image{URN: &urn}, Architecture: ptr.To("bar")}}, "bar", "1.2.3", ptr.To("foo"), nil, true),
		Entry("entry exists(urn)", []api.MachineImage{{Name: "bar", Version: "1.2.3", Image: api.Image{URN: &urn}, Architecture: ptr.To("foo")}}, "bar", "1.2.3", ptr.To("foo"), &api.MachineImage{Name: "bar", Version: "1.2.3", Image: api.Image{URN: &urn}, Architecture: ptr.To("foo")}, false),
		Entry("entry exists(urn) if architecture is nil", []api.MachineImage{{Name: "bar", Version: "1.2.3", Image: api.Image{URN: &urn}}}, "bar", "1.2.3", ptr.To("amd64"), &api.MachineImage{Name: "bar", Version: "1.2.3", Image: api.Image{URN: &urn}, Architecture: ptr.To("amd64")}, false),
		Entry("entry exists(id)", []api.MachineImage{{Name: "bar", Version: "1.2.3", Image: api.Image{ID: &imageID}, Architecture: ptr.To("foo")}}, "bar", "1.2.3", ptr.To("foo"), &api.MachineImage{Name: "bar", Version: "1.2.3", Image: api.Image{ID: &imageID}, Architecture: ptr.To("foo")}, false),
		Entry("entry exists(communityGalleryImageID)", []api.MachineImage{{Name: "bar", Version: "1.2.3", Image: api.Image{CommunityGalleryImageID: &communityImageId}, Architecture: ptr.To("foo")}}, "bar", "1.2.3", ptr.To("foo"), &api.MachineImage{Name: "bar", Version: "1.2.3", Image: api.Image{CommunityGalleryImageID: &communityImageId}, Architecture: ptr.To("foo")}, false),
		Entry("entry exists(sharedGalleryImageID)", []api.MachineImage{{Name: "bar", Version: "1.2.3", Image: api.Image{SharedGalleryImageID: &sharedImageId}, Architecture: ptr.To("foo")}}, "bar", "1.2.3", ptr.To("foo"), &api.MachineImage{Name: "bar", Version: "1.2.3", Image: api.Image{SharedGalleryImageID: &sharedImageId}, Architecture: ptr.To("foo")}, false),
		Entry("entry exists(accelerated networking active)", []api.MachineImage{{Name: "bar", Version: "1.2.3", Image: api.Image{URN: &urn}, AcceleratedNetworking: &boolTrue, Architecture: ptr.To("foo")}}, "bar", "1.2.3", ptr.To("foo"), &api.MachineImage{Name: "bar", Version: "1.2.3", Image: api.Image{URN: &urn}, AcceleratedNetworking: &boolTrue, Architecture: ptr.To("foo")}, false),
		Entry("entry exists(accelerated networking inactive)", []api.MachineImage{{Name: "bar", Version: "1.2.3", Image: api.Image{URN: &urn}, AcceleratedNetworking: &boolFalse, Architecture: ptr.To("foo")}}, "bar", "1.2.3", ptr.To("foo"), &api.MachineImage{Name: "bar", Version: "1.2.3", Image: api.Image{URN: &urn}, AcceleratedNetworking: &boolFalse, Architecture: ptr.To("foo")}, false),
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

		Entry("list is nil", nil, "ubuntu", "1", ptr.To("foo"), nil),

		Entry("profile empty list", []api.MachineImages{}, "ubuntu", "1", ptr.To("foo"), nil),
		Entry("profile entry not found (image does not exist)", makeProfileMachineImages("debian", "1", "3", "5", "7", ptr.To("foo")), "ubuntu", "1", ptr.To("foo"), nil),
		Entry("profile entry not found (version does not exist)", makeProfileMachineImages("ubuntu", "2", "4", "6", "7", ptr.To("foo")), "ubuntu", "1", ptr.To("foo"), nil),
		Entry("profile entry not found (no architecture)", makeProfileMachineImages("ubuntu", "2", "4", "6", "7", ptr.To("bar")), "ubuntu", "2", ptr.To("foo"), nil),
		Entry("profile entry(urn)", makeProfileMachineImages("ubuntu", "1", "3", "5", "6", ptr.To("foo")), "ubuntu", "1", ptr.To("foo"), &api.MachineImage{Name: "ubuntu", Version: "1", Image: api.Image{URN: &profileURN}, Architecture: ptr.To("foo")}),
		Entry("profile entry(id)", makeProfileMachineImages("ubuntu", "1", "3", "5", "6", ptr.To("foo")), "ubuntu", "3", ptr.To("foo"), &api.MachineImage{Name: "ubuntu", Version: "3", Image: api.Image{ID: &profileID}, Architecture: ptr.To("foo")}),
		Entry("profile entry(communiyGalleryId)", makeProfileMachineImages("ubuntu", "1", "3", "5", "6", ptr.To("foo")), "ubuntu", "5", ptr.To("foo"), &api.MachineImage{Name: "ubuntu", Version: "5", Image: api.Image{CommunityGalleryImageID: &profileCommunityImageId}, Architecture: ptr.To("foo")}),
		Entry("profile entry(sharedGalleryId)", makeProfileMachineImages("ubuntu", "1", "3", "5", "6", ptr.To("foo")), "ubuntu", "6", ptr.To("foo"), &api.MachineImage{Name: "ubuntu", Version: "6", Image: api.Image{SharedGalleryImageID: &profileSharedImageId}, Architecture: ptr.To("foo")}),

		Entry("valid image reference, only urn", makeProfileMachineImageWithURNandIDandCommunityGalleryIDandSharedGalleryImageID("ubuntu", "1", &profileURN, nil, nil, nil, ptr.To("foo")),
			"ubuntu", "1", ptr.To("foo"), &api.MachineImage{Name: "ubuntu", Version: "1", Image: api.Image{URN: &profileURN}, Architecture: ptr.To("foo")}),
		Entry("valid image reference, only id", makeProfileMachineImageWithURNandIDandCommunityGalleryIDandSharedGalleryImageID("ubuntu", "1", nil, &profileID, nil, nil, ptr.To("foo")),
			"ubuntu", "1", ptr.To("foo"), &api.MachineImage{Name: "ubuntu", Version: "1", Image: api.Image{ID: &profileID}, Architecture: ptr.To("foo")}),
		Entry("valid image reference, only communityGalleryImageID", makeProfileMachineImageWithURNandIDandCommunityGalleryIDandSharedGalleryImageID("ubuntu", "1", nil, nil, &profileCommunityImageId, nil, ptr.To("foo")),
			"ubuntu", "1", ptr.To("foo"), &api.MachineImage{Name: "ubuntu", Version: "1", Image: api.Image{CommunityGalleryImageID: &profileCommunityImageId}, Architecture: ptr.To("foo")}),
		Entry("valid image reference, only sharedGalleryImageID", makeProfileMachineImageWithURNandIDandCommunityGalleryIDandSharedGalleryImageID("ubuntu", "1", nil, nil, nil, &profileSharedImageId, ptr.To("foo")),
			"ubuntu", "1", ptr.To("foo"), &api.MachineImage{Name: "ubuntu", Version: "1", Image: api.Image{SharedGalleryImageID: &profileSharedImageId}, Architecture: ptr.To("foo")}),
	)
})

func makeProfileMachineImages(name, urnVersion, idVersion, communityGalleryImageIdVersion string, sharedGalleryImageIdVersion string, architecture *string) []api.MachineImages {
	return []api.MachineImages{
		{
			Name: name,
			Versions: []api.MachineImageVersion{
				{
					Version:      urnVersion,
					Architecture: architecture,
					Image: api.Image{
						URN: &profileURN,
					},
				},
				{
					Version:      idVersion,
					Architecture: architecture,
					Image: api.Image{
						ID: &profileID,
					},
				},
				{
					Version:      communityGalleryImageIdVersion,
					Architecture: architecture,
					Image: api.Image{
						CommunityGalleryImageID: &profileCommunityImageId,
					},
				},
				{
					Version:      sharedGalleryImageIdVersion,
					Architecture: architecture,

					Image: api.Image{
						SharedGalleryImageID: &profileSharedImageId,
					},
				},
			},
		},
	}
}

// nolint :unparam
func makeProfileMachineImageWithURNandIDandCommunityGalleryIDandSharedGalleryImageID(name, version string, urn, id, communityGalleryImageID, sharedGalleryImageID, architecture *string) []api.MachineImages {
	return []api.MachineImages{
		{
			Name: name,
			Versions: []api.MachineImageVersion{
				{
					Version:      version,
					Architecture: architecture,
					Image: api.Image{
						URN:                     urn,
						ID:                      id,
						CommunityGalleryImageID: communityGalleryImageID,
						SharedGalleryImageID:    sharedGalleryImageID,
					},
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
