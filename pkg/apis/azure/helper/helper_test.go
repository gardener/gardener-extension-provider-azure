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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var (
	profileURN = "publisher:offer:sku:1.2.4"
	profileID  = "/subscription/image/id"
)

var _ = Describe("Helper", func() {
	var (
		purpose      api.Purpose = "foo"
		purposeWrong api.Purpose = "baz"
		urn          string      = "publisher:offer:sku:version"
		imageID      string      = "/image/id"
		boolTrue                 = true
		boolFalse                = false
		zone                     = "zone"
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
		func(machineImages []api.MachineImage, name, version string, expectedMachineImage *api.MachineImage, expectErr bool) {
			machineImage, err := FindMachineImage(machineImages, name, version)
			expectResults(machineImage, expectedMachineImage, err, expectErr)
		},

		Entry("list is nil", nil, "foo", "1.2.3", nil, true),
		Entry("empty list", []api.MachineImage{}, "foo", "1.2.3", nil, true),
		Entry("entry not found (no name)", []api.MachineImage{{Name: "bar", Version: "1.2.3", URN: &urn}}, "foo", "1.2.3", nil, true),
		Entry("entry not found (no version)", []api.MachineImage{{Name: "bar", Version: "1.2.3", URN: &urn}}, "bar", "1.2.4", nil, true),
		Entry("entry exists(urn)", []api.MachineImage{{Name: "bar", Version: "1.2.3", URN: &urn}}, "bar", "1.2.3", &api.MachineImage{Name: "bar", Version: "1.2.3", URN: &urn}, false),
		Entry("entry exists(id)", []api.MachineImage{{Name: "bar", Version: "1.2.3", ID: &imageID}}, "bar", "1.2.3", &api.MachineImage{Name: "bar", Version: "1.2.3", ID: &imageID}, false),
		Entry("entry exists(accelerated networking active)", []api.MachineImage{{Name: "bar", Version: "1.2.3", URN: &urn, AcceleratedNetworking: &boolTrue}}, "bar", "1.2.3", &api.MachineImage{Name: "bar", Version: "1.2.3", URN: &urn, AcceleratedNetworking: &boolTrue}, false),
		Entry("entry exists(accelerated networking inactive)", []api.MachineImage{{Name: "bar", Version: "1.2.3", URN: &urn, AcceleratedNetworking: &boolFalse}}, "bar", "1.2.3", &api.MachineImage{Name: "bar", Version: "1.2.3", URN: &urn, AcceleratedNetworking: &boolFalse}, false),
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
		func(profileImages []api.MachineImages, imageName, version string, expectedImage *api.MachineImage) {
			cfg := &api.CloudProfileConfig{}
			cfg.MachineImages = profileImages
			image, err := FindImageFromCloudProfile(cfg, imageName, version)

			Expect(image).To(Equal(expectedImage))
			if expectedImage != nil {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		},

		Entry("list is nil", nil, "ubuntu", "1", nil),

		Entry("profile empty list", []api.MachineImages{}, "ubuntu", "1", nil),
		Entry("profile entry not found (image does not exist)", makeProfileMachineImages("debian", "1", "3"), "ubuntu", "1", nil),
		Entry("profile entry not found (version does not exist)", makeProfileMachineImages("ubuntu", "2", "4"), "ubuntu", "1", nil),
		Entry("profile entry(urn)", makeProfileMachineImages("ubuntu", "1", "3"), "ubuntu", "1", &api.MachineImage{Name: "ubuntu", Version: "1", URN: &profileURN}),
		Entry("profile entry(id)", makeProfileMachineImages("ubuntu", "1", "3"), "ubuntu", "3", &api.MachineImage{Name: "ubuntu", Version: "3", ID: &profileID}),

		Entry("valid image reference, only urn", makeProfileMachineImageWithIDandURN("ubuntu", "1", &profileURN, nil), "ubuntu", "1", &api.MachineImage{Name: "ubuntu", Version: "1", URN: &profileURN}),
		Entry("valid image reference, only id", makeProfileMachineImageWithIDandURN("ubuntu", "1", nil, &profileID), "ubuntu", "1", &api.MachineImage{Name: "ubuntu", Version: "1", ID: &profileID}),
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

func makeProfileMachineImages(name, urnVersion, idVersion string) []api.MachineImages {
	return []api.MachineImages{
		{
			Name: name,
			Versions: []api.MachineImageVersion{
				{
					Version: urnVersion,
					URN:     &profileURN,
				},
				{
					Version: idVersion,
					ID:      &profileID,
				},
			},
		},
	}
}

func makeProfileMachineImageWithIDandURN(name, version string, urn, id *string) []api.MachineImages {
	return []api.MachineImages{
		{
			Name: name,
			Versions: []api.MachineImageVersion{
				{
					Version: version,
					URN:     urn,
					ID:      id,
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
