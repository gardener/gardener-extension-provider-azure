// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure_test

import (
	"encoding/json"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	azuretypes "github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure"
	internalinfra "github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

var _ = Describe("ShouldUseFlow", func() {
	Context("without any flow annotation", func() {
		It("should not use FlowContext", func() {
			Expect(infrastructure.GetFlowAnnotationValue(&extensionsv1alpha1.Infrastructure{})).To(BeFalse())
		})
	})
	Context("with flow annotation in infrastruture", func() {
		infra := &extensionsv1alpha1.Infrastructure{}
		metav1.SetMetaDataAnnotation(&infra.ObjectMeta, azuretypes.AnnotationKeyUseFlow, "true")
		It("should use the FlowContext", func() {
			Expect(infrastructure.GetFlowAnnotationValue(infra)).To(BeTrue())
		})
	})
})

var _ = Describe("ReconcilationStrategy", func() {
	cluster := internalinfra.MakeCluster("11.0.0.0/16", "12.0.0.0/16", "europe", 1, 1)
	It("should use Flow if an annotation is found", func() {
		infra := &extensionsv1alpha1.Infrastructure{}
		metav1.SetMetaDataAnnotation(&infra.ObjectMeta, azuretypes.AnnotationKeyUseFlow, "true")

		sut := infrastructure.SelectorFunc(infrastructure.OnReconcile)
		useFlow, err := sut(infra, cluster)
		Expect(err).NotTo(HaveOccurred())
		Expect(useFlow).To(BeTrue())
	})
	It("should use Flow if resources were reconciled with Flow before, regardless of annotation", func() {
		state := newInfrastructureState()
		infra := &extensionsv1alpha1.Infrastructure{}
		infra.Status.State = &runtime.RawExtension{Object: state}

		sut := infrastructure.SelectorFunc(infrastructure.OnReconcile)
		useFlow, err := sut(infra, cluster)
		Expect(err).NotTo(HaveOccurred())
		Expect(useFlow).To(BeTrue())
	})

	It("should use Terraform if no flow state is found and there is no flow annotation", func() {
		infra := &extensionsv1alpha1.Infrastructure{}
		infra.Status.State = &runtime.RawExtension{Raw: getRawTerraformState(`{"provider": "terraform"}`)}

		sut := infrastructure.SelectorFunc(infrastructure.OnReconcile)
		useFlow, err := sut(infra, cluster)
		Expect(err).NotTo(HaveOccurred())
		Expect(useFlow).To(BeFalse())
	})

	It("should delete with Terraform if resources were reconciled with Terraform", func() {
		infra := &extensionsv1alpha1.Infrastructure{}
		infra.Status.State = &runtime.RawExtension{Raw: getRawTerraformState(`{"provider": "terraform"}`)}
		sut := infrastructure.SelectorFunc(infrastructure.OnDelete)
		useFlow, err := sut(infra, cluster)
		Expect(err).NotTo(HaveOccurred())
		Expect(useFlow).To(BeFalse())
	})
	It("should delete with Flow if resources were reconciled with Flow", func() {
		infra := &extensionsv1alpha1.Infrastructure{}
		state := newInfrastructureState()
		infra.Status.State = &runtime.RawExtension{Object: state}

		sut := infrastructure.SelectorFunc(infrastructure.OnDelete)
		useFlow, err := sut(infra, cluster)
		Expect(err).NotTo(HaveOccurred())
		Expect(useFlow).To(BeTrue())
	})

})

func getRawTerraformState(jsonContent string) []byte {
	state := internalinfra.InfrastructureState{
		TerraformState: &runtime.RawExtension{
			Raw: []byte(jsonContent),
		},
	}
	stateRaw, _ := json.Marshal(state)
	return stateRaw
}

func newInfrastructureState() *v1alpha1.InfrastructureState {
	return &v1alpha1.InfrastructureState{
		TypeMeta: helper.InfrastructureStateTypeMeta,
	}
}
