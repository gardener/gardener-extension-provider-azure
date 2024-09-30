// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
	Describe("#ValidateWorkerConfig", func() {
		var (
			fldPath = field.NewPath("config")
		)

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

			It("should return error when all resources not specified", func() {
				nodeTemplate := &extensionsv1alpha1.NodeTemplate{
					Capacity: corev1.ResourceList{
						"gpu": resource.MustParse("0"),
					},
				}

				Expect(validateNodeTemplate(nodeTemplate, fldPath)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("config.nodeTemplate.capacity"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("config.nodeTemplate.capacity"),
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
						"Field": Equal("config.nodeTemplate.capacity.memory"),
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

				Expect(validateDataVolumeConf(dataVolumeConfigs, dataVolumes, fldPath)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("config.dataVolumes.Name"),
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

				Expect(validateDataVolumeConf(dataVolumeConfigs, dataVolumes, fldPath)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("config.dataVolumes.ImageRef"),
					})),
				))
			})
		})
	})

})
