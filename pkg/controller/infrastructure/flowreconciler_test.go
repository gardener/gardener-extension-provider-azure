// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package infrastructure_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure"
	internalinfra "github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

var _ = Describe("ShouldUseFlow", func() {
	Context("without any flow annotation", func() {
		It("should not use FlowReconciler", func() {
			cluster := internalinfra.MakeCluster("11.0.0.0/16", "12.0.0.0/16", "europe", 1, 1)
			Expect(infrastructure.ShouldUseFlow(&extensionsv1alpha1.Infrastructure{}, cluster)).To(BeFalse())
		})
	})
	Context("with flow annotation in infrastruture", func() {
		infra := &extensionsv1alpha1.Infrastructure{}
		cluster := internalinfra.MakeCluster("11.0.0.0/16", "12.0.0.0/16", "europe", 1, 1)
		metav1.SetMetaDataAnnotation(&infra.ObjectMeta, infrastructure.AnnotationKeyUseFlow, "true")
		It("should use the FlowReconciler", func() {
			Expect(infrastructure.ShouldUseFlow(infra, cluster)).To(BeTrue())
		})
	})
	Context("with flow annotation in shoot", func() {
		cluster := internalinfra.MakeCluster("11.0.0.0/16", "12.0.0.0/16", "europe", 1, 1)
		metav1.SetMetaDataAnnotation(&cluster.Shoot.ObjectMeta, infrastructure.AnnotationKeyUseFlow, "true")
		It("should use the FlowReconciler", func() {
			Expect(infrastructure.ShouldUseFlow(&extensionsv1alpha1.Infrastructure{}, cluster)).To(BeTrue())
		})
	})
	Context("with flow annotation in seed", func() {
		cluster := internalinfra.MakeCluster("11.0.0.0/16", "12.0.0.0/16", "europe", 1, 1)
		cluster.Seed = &v1beta1.Seed{}
		metav1.SetMetaDataAnnotation(&cluster.Seed.ObjectMeta, infrastructure.AnnotationKeyUseFlow, "true")
		It("should use the FlowReconciler", func() {
			Expect(infrastructure.ShouldUseFlow(&extensionsv1alpha1.Infrastructure{}, cluster)).To(BeTrue())
		})
	})
})
