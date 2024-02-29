// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"
)

var _ = Describe("ValidateWorkerConfig", func() {
	Describe("#ValidateWorkerConfig", func() {
		var (
			nodeTemplate *extensionsv1alpha1.NodeTemplate

			worker  *apisazure.WorkerConfig
			fldPath = field.NewPath("config")
		)

		BeforeEach(func() {
			worker = &apisazure.WorkerConfig{
				NodeTemplate: nodeTemplate,
			}
		})

		It("should return no errors for a valid nodetemplate configuration", func() {
			worker.NodeTemplate = &extensionsv1alpha1.NodeTemplate{
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("50Gi"),
					"gpu":                 resource.MustParse("0"),
				},
			}
			Expect(ValidateWorkerConfig(worker, fldPath)).To(BeEmpty())
		})

		It("should return error when all resources not specified", func() {
			worker.NodeTemplate = &extensionsv1alpha1.NodeTemplate{
				Capacity: corev1.ResourceList{
					"gpu": resource.MustParse("0"),
				},
			}

			Expect(ValidateWorkerConfig(worker, fldPath)).To(ConsistOf(
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
			worker.NodeTemplate = &extensionsv1alpha1.NodeTemplate{
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("-50Gi"),
					"gpu":                 resource.MustParse("0"),
				},
			}

			Expect(ValidateWorkerConfig(worker, fldPath)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("config.nodeTemplate.capacity.memory"),
				})),
			))
		})
	})

})
