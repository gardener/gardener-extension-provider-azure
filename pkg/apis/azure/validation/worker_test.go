// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validation_test

import (
	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
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
