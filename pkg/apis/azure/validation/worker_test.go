// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"github.com/gardener/gardener/pkg/apis/core"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
)

var _ = Describe("ValidateWorkerConfig", func() {
	var (
		fldPath   *field.Path
		workerCfg *apisazure.WorkerConfig
	)

	BeforeEach(func() {
		workerCfg = &apisazure.WorkerConfig{}
		fldPath = field.NewPath("config")
	})

	Describe("#DiagnosticsProfile", func() {
		It("should accept valid input", func() {
			workerCfg.DiagnosticsProfile = &apisazure.DiagnosticsProfile{
				Enabled:    true,
				StorageURI: ptr.To("https://mystorageaccount.blob.core.windows.net/mycontainer"),
			}
			Expect(ValidateWorkerConfig(workerCfg, nil, fldPath)).To(BeEmpty())
		})

		It("should reject invalid input", func() {
			workerCfg.DiagnosticsProfile = &apisazure.DiagnosticsProfile{
				Enabled:    true,
				StorageURI: ptr.To("foobar"),
			}
			errorList := ValidateWorkerConfig(workerCfg, nil, fldPath)
			Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras,
				Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("config.diagnosticsProfile.storageURI"),
					"Detail": ContainSubstring("does not match expected regex"),
				}))))
		})
	})

	Describe("NodeTemplate", func() {
		It("should return no errors for a valid nodetemplate configuration", func() {
			nodeTemplate := &extensionsv1alpha1.NodeTemplate{
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("50Gi"),
					"gpu":                 resource.MustParse("0"),
				},
			}
			Expect(validateNodeTemplate(nodeTemplate, fldPath)).To(BeEmpty())
		})

		It("should return error when invalid resources are specified", func() {
			nodeTemplate := &extensionsv1alpha1.NodeTemplate{
				Capacity: corev1.ResourceList{
					"foo":                 resource.MustParse("0"),
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("50Gi"),
					"gpu":                 resource.MustParse("0"),
				},
			}

			Expect(validateNodeTemplate(nodeTemplate, fldPath)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("config.capacity.foo"),
					"Detail": ContainSubstring("foo is an unsupported resource name."),
				})),
			))
		})

		It("should return error when resource value is negative", func() {
			nodeTemplate := &extensionsv1alpha1.NodeTemplate{
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("-50Gi"),
					"gpu":                 resource.MustParse("0"),
				},
			}

			Expect(validateNodeTemplate(nodeTemplate, fldPath)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("config.capacity.memory"),
				})),
			))
		})

	})

	Describe("DataVolumes", func() {
		It("should allow valid DataVolumes config", func() {
			dataVolumes := []core.DataVolume{{
				Name: "test-disk",
			}}
			dataVolumeConfigs := []apisazure.DataVolume{{
				Name: "test-disk",
				ImageRef: &apisazure.Image{
					URN: ptr.To("sap:gardenlinux:greatest:1312.0.0"),
				},
			}}

			Expect(validateDataVolumeConf(dataVolumeConfigs, dataVolumes, fldPath)).To(BeEmpty())
		})

		It("should forbid config of none existing DataVolume", func() {
			var dataVolumes []core.DataVolume
			dataVolumeConfigs := []apisazure.DataVolume{{
				Name: "does not exist",
				ImageRef: &apisazure.Image{
					URN: ptr.To("sap:gardenlinux:greatest:1312.0.0"),
				},
			}}

			Expect(validateDataVolumeConf(dataVolumeConfigs, dataVolumes, fldPath.Child("dataVolumes"))).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("config.dataVolumes[0].name"),
					"Detail": Equal("no dataVolume with this name exists"),
				})),
			))
		})

		It("should forbid empty DataVolume ImageRef", func() {
			dataVolumes := []core.DataVolume{{
				Name: "test-disk",
			}}
			dataVolumeConfigs := []apisazure.DataVolume{{
				Name:     "test-disk",
				ImageRef: &apisazure.Image{},
			}}

			Expect(validateDataVolumeConf(dataVolumeConfigs, dataVolumes, fldPath.Child("dataVolumes"))).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("config.dataVolumes[0].imageRef"),
					"Detail": Equal("imageRef is defined but empty"),
				})),
			))
		})

		It("should forbid invalid DataVolume ImageRef URN", func() {
			dataVolumes := []core.DataVolume{{
				Name: "test-disk",
			}}
			dataVolumeConfigs := []apisazure.DataVolume{{
				Name: "test-disk",
				ImageRef: &apisazure.Image{
					URN: ptr.To("invalid-urn"),
				},
			}}

			Expect(validateDataVolumeConf(dataVolumeConfigs, dataVolumes, fldPath.Child("dataVolumes"))).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("config.dataVolumes[0].imageRef.urn"),
				})),
			))
		})

		It("should allow DataVolume ImageRef URN", func() {
			dataVolumes := []core.DataVolume{{
				Name: "test-disk",
			}}
			dataVolumeConfigs := []apisazure.DataVolume{{
				Name: "test-disk",
				ImageRef: &apisazure.Image{
					URN: ptr.To("sap:gardenlinux:greatest:1312.0.0"),
				},
			}}

			Expect(validateDataVolumeConf(dataVolumeConfigs, dataVolumes, fldPath)).To(BeEmpty())
		})

		It("should forbid invalid DataVolume ImageRef with Community Gallery", func() {
			dataVolumes := []core.DataVolume{{
				Name: "test-disk",
			}}
			dataVolumeConfigs := []apisazure.DataVolume{{
				Name: "test-disk",
				ImageRef: &apisazure.Image{
					CommunityGalleryImageID: ptr.To("invalid-gallery-image-id"),
				},
			}}

			Expect(validateDataVolumeConf(dataVolumeConfigs, dataVolumes, fldPath.Child("dataVolumes"))).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("config.dataVolumes[0].imageRef.communityGalleryImageID"),
				})),
			))
		})

		It("should allow DataVolume ImageRef with Community Gallery", func() {
			dataVolumes := []core.DataVolume{{
				Name: "test-disk",
			}}
			dataVolumeConfigs := []apisazure.DataVolume{{
				Name: "test-disk",
				ImageRef: &apisazure.Image{
					CommunityGalleryImageID: ptr.To("/CommunityGalleries/gardenlinux-13e998fe-534d-4b0a-8a27-f16a73aef620/Images/gardenlinux/Versions/1443.15.0"),
				},
			}}

			Expect(validateDataVolumeConf(dataVolumeConfigs, dataVolumes, fldPath)).To(BeEmpty())
		})
	})
})
