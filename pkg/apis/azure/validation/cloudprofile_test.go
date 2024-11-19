// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"
)

var (
	imageName               = "ubuntu"
	urn                     = "Publisher:Offer:Sku:Version"
	id                      = "/subscription/id/image/id"
	communityGalleryImageID = "/CommunityGalleries/id/Images/myImageDefinition/versions/version"
	sharedGalleryImageID    = "/SharedGalleries/id/Images/myImageDefinition/versions/version"
)

var _ = Describe("CloudProfileConfig validation", func() {
	Describe("#ValidateCloudProfileConfig", func() {
		var (
			cloudProfileMachineImages []core.MachineImage
			cloudProfileConfig        *apisazure.CloudProfileConfig
			root                      = field.NewPath("root")
		)

		BeforeEach(func() {
			cloudProfileMachineImages = []core.MachineImage{
				{
					Name: imageName,
					Versions: []core.MachineImageVersion{
						{
							ExpirableVersion: core.ExpirableVersion{
								Version: "Version",
							},
							Architectures: []string{"amd64"},
						},
					},
				},
			}
			cloudProfileConfig = &apisazure.CloudProfileConfig{
				CountUpdateDomains: []apisazure.DomainCount{
					{
						Region: "westeurope",
						Count:  1,
					},
				},
				CountFaultDomains: []apisazure.DomainCount{
					{
						Region: "westeurope",
						Count:  1,
					},
				},
				MachineImages: []apisazure.MachineImages{
					{
						Name: imageName,
						Versions: []apisazure.MachineImageVersion{
							{
								Version:      "Version",
								URN:          &urn,
								Architecture: ptr.To("amd64"),
							},
						},
					},
				},
			}
		})

		Context("machine image validation", func() {
			It("should allow valid cloudProfileConfig", func() {
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, cloudProfileMachineImages, root)
				Expect(errorList).To(BeEmpty())
			})

			It("should enforce that at least one machine image has been defined", func() {
				cloudProfileConfig.MachineImages = []apisazure.MachineImages{}

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, cloudProfileMachineImages, root)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages"),
				}))))
			})

			It("should forbid unsupported machine image values", func() {
				cloudProfileConfig.MachineImages = []apisazure.MachineImages{{}}

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, cloudProfileMachineImages, root)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages"),
				}))))
			})

			It("should forbid unsupported machine image architecture", func() {
				cloudProfileConfig.MachineImages[0].Versions[0].Architecture = ptr.To("foo")

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, cloudProfileMachineImages, root)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages[0].versions"),
				}))))
			})

			DescribeTable("forbid unsupported machine image urn",
				func(urn string, matcher gomegatypes.GomegaMatcher) {
					cloudProfileConfig.MachineImages[0].Versions[0].URN = &urn

					errorList := ValidateCloudProfileConfig(cloudProfileConfig, cloudProfileMachineImages, root)

					Expect(errorList).To(matcher)
				},
				Entry("correct urn", "foo:bar:baz:ban", BeEmpty()),
				Entry("empty urn", "", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Type": Equal(field.ErrorTypeRequired), "Field": Equal("root.machineImages[0].versions[0].urn")})))),
				Entry("only one part", "foo", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Type": Equal(field.ErrorTypeInvalid), "Field": Equal("root.machineImages[0].versions[0].urn")})))),
				Entry("only two parts", "foo:bar", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Type": Equal(field.ErrorTypeInvalid), "Field": Equal("root.machineImages[0].versions[0].urn")})))),
				Entry("only three parts", "foo:bar:baz", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Type": Equal(field.ErrorTypeInvalid), "Field": Equal("root.machineImages[0].versions[0].urn")})))),
				Entry("more than four parts", "foo:bar:baz:ban:bam", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Type": Equal(field.ErrorTypeInvalid), "Field": Equal("root.machineImages[0].versions[0].urn")})))),
			)

			DescribeTable("forbid unsupported machine image ID",
				func(id string, matcher gomegatypes.GomegaMatcher) {
					cloudProfileConfig.MachineImages[0].Versions[0].URN = nil
					cloudProfileConfig.MachineImages[0].Versions[0].ID = &id

					errorList := ValidateCloudProfileConfig(cloudProfileConfig, cloudProfileMachineImages, root)

					Expect(errorList).To(matcher)
				},
				Entry("correct id", "/non/empty/id", BeEmpty()),
				Entry("empty id", "", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Type": Equal(field.ErrorTypeRequired), "Field": Equal("root.machineImages[0].versions[0].id")})))),
			)

			DescribeTable("forbid unsupported communityGalleryImageID",
				func(communityGalleryImageID string, matcher gomegatypes.GomegaMatcher) {
					cloudProfileConfig.MachineImages[0].Versions[0].URN = nil
					cloudProfileConfig.MachineImages[0].Versions[0].CommunityGalleryImageID = &communityGalleryImageID

					errorList := ValidateCloudProfileConfig(cloudProfileConfig, cloudProfileMachineImages, root)

					Expect(errorList).To(matcher)
				},
				Entry("correct id", communityGalleryImageID, BeEmpty()),
				Entry("incorrect number of parts id", "/too/little/parts/id", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Type": Equal(field.ErrorTypeInvalid), "Field": Equal("root.machineImages[0].versions[0].communityGalleryImageID")})))),
				Entry("incorrect number of parts id", "/there/are/way/too/many/parts/in/this/id", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Type": Equal(field.ErrorTypeInvalid), "Field": Equal("root.machineImages[0].versions[0].communityGalleryImageID")})))),
				Entry("does not start with correct prefix", "/somegallery/id/Images/myImageDefinition/versions/version", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Type": Equal(field.ErrorTypeInvalid), "Field": Equal("root.machineImages[0].versions[0].communityGalleryImageID")})))),
			)

			DescribeTable("forbid unsupported sharedGalleryImageID",
				func(sharedGalleryImageID string, matcher gomegatypes.GomegaMatcher) {
					cloudProfileConfig.MachineImages[0].Versions[0].URN = nil
					cloudProfileConfig.MachineImages[0].Versions[0].SharedGalleryImageID = &sharedGalleryImageID

					errorList := ValidateCloudProfileConfig(cloudProfileConfig, cloudProfileMachineImages, root)

					Expect(errorList).To(matcher)
				},
				Entry("correct id", sharedGalleryImageID, BeEmpty()),
				Entry("incorrect number of parts id", "/too/little/parts/id", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Type": Equal(field.ErrorTypeInvalid), "Field": Equal("root.machineImages[0].versions[0].sharedGalleryImageID")})))),
				Entry("incorrect number of parts id", "/there/are/way/too/many/parts/in/this/id", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Type": Equal(field.ErrorTypeInvalid), "Field": Equal("root.machineImages[0].versions[0].sharedGalleryImageID")})))),
				Entry("does not start with correct prefix", "/somegallery/id/Images/myImageDefinition/versions/version", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Type": Equal(field.ErrorTypeInvalid), "Field": Equal("root.machineImages[0].versions[0].sharedGalleryImageID")})))),
			)

			DescribeTable("forbid invalid image reference configuration",
				func(urn, id *string, communityGalleryImageID *string, sharedGalleryImageID *string, matcher gomegatypes.GomegaMatcher) {
					cloudProfileConfig.MachineImages[0].Versions[0].ID = id
					cloudProfileConfig.MachineImages[0].Versions[0].URN = urn
					cloudProfileConfig.MachineImages[0].Versions[0].SharedGalleryImageID = sharedGalleryImageID
					cloudProfileConfig.MachineImages[0].Versions[0].CommunityGalleryImageID = communityGalleryImageID

					errorList := ValidateCloudProfileConfig(cloudProfileConfig, cloudProfileMachineImages, root)

					Expect(errorList).To(matcher)
				},
				Entry("only valid URN", &urn, nil, nil, nil, BeEmpty()), // do P&C here for all combinations
				Entry("only valid ID", nil, &id, nil, nil, BeEmpty()),
				Entry("only valid CommunityGalleryImageID", nil, nil, &communityGalleryImageID, nil, BeEmpty()),
				Entry("only valid SharedGalleryImageID", nil, nil, nil, &sharedGalleryImageID, BeEmpty()),
				Entry("urn, id, communityGalleryImageID and sharedGalleryImageID are nil", nil, nil, nil, nil, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages[0].versions[0]")})))),
				Entry("urn and id are non-empty", &urn, &id, nil, nil, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages[0].versions[0]")})))),
				Entry("urn and communityGalleryImageID are non-empty", &urn, nil, &communityGalleryImageID, nil, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages[0].versions[0]")})))),
				Entry("id and communityGalleryImageID are non-empty", nil, &id, &communityGalleryImageID, nil, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages[0].versions[0]")})))),
				Entry("sharedGalleryImageID and communityGalleryImageID are non-empty", nil, nil, &communityGalleryImageID, &sharedGalleryImageID, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages[0].versions[0]")})))),
				Entry("urn and sharedGalleryImageID are non-empty", &urn, nil, nil, &sharedGalleryImageID, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages[0].versions[0]")})))),
				Entry("id and sharedGalleryImageID are non-empty", nil, &id, nil, &sharedGalleryImageID, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages[0].versions[0]")})))),
				Entry("urn, id and communityGalleryImageID are non-empty", &urn, &id, &communityGalleryImageID, nil, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages[0].versions[0]")})))),
				Entry("urn, id and sharedGalleryImageID are non-empty", &urn, &id, nil, &sharedGalleryImageID, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages[0].versions[0]")})))),
				Entry("urn, communityGalleryImageID and sharedGalleryImageID are non-empty", &urn, nil, &communityGalleryImageID, &sharedGalleryImageID, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages[0].versions[0]")})))),
				Entry("id, communityGalleryImageID and sharedGalleryImageID are non-empty", nil, &id, &communityGalleryImageID, &sharedGalleryImageID, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages[0].versions[0]")})))),
				Entry("urn, id, communityGalleryImageID and sharedGalleryImageID are empty", ptr.To(""), ptr.To(""), ptr.To(""), ptr.To(""), ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages[0].versions[0]"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages[0].versions[0].id"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages[0].versions[0].urn"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages[0].versions[0].communityGalleryImageID"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages[0].versions[0].sharedGalleryImageID"),
				})))))

			It("should forbid missing architecture in cpConfigMachineImages", func() {
				cloudProfileMachineImages[0].Versions[0].Architectures = []string{"amd64", "arm64"}

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, cloudProfileMachineImages, root)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.machineImages[0].versions"),
				}))))
			})
		})

		Context("fault domain count validation", func() {
			It("should enforce that at least one fault domain count has been defined", func() {
				cloudProfileConfig.CountFaultDomains = []apisazure.DomainCount{}

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, cloudProfileMachineImages, root)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.countFaultDomains"),
				}))))
			})

			It("should forbid fault domain count with unsupported format", func() {
				cloudProfileConfig.CountFaultDomains = []apisazure.DomainCount{
					{
						Region: "",
						Count:  -1,
					},
				}

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, cloudProfileMachineImages, root)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.countFaultDomains[0].region"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("root.countFaultDomains[0].count"),
				}))))
			})
		})

		Context("update domain count validation", func() {
			It("should enforce that at least one update domain count has been defined", func() {
				cloudProfileConfig.CountUpdateDomains = []apisazure.DomainCount{}

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, cloudProfileMachineImages, root)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.countUpdateDomains"),
				}))))
			})

			It("should forbid update domain count with unsupported format", func() {
				cloudProfileConfig.CountUpdateDomains = []apisazure.DomainCount{
					{
						Region: "",
						Count:  -1,
					},
				}

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, cloudProfileMachineImages, root)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("root.countUpdateDomains[0].region"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("root.countUpdateDomains[0].count"),
				}))))
			})
		})
	})
})
