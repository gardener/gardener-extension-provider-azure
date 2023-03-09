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
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow/shared"
	imock "github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/mock"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure"
	internalinfra "github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

var _ = Describe("ShouldUseFlow", func() {
	Context("without any flow annotation", func() {
		It("should not use FlowReconciler", func() {
			cluster := internalinfra.MakeCluster("11.0.0.0/16", "12.0.0.0/16", "europe", 1, 1)
			Expect(infrastructure.HasFlowAnnotation(&extensionsv1alpha1.Infrastructure{}, cluster)).To(BeFalse())
		})
	})
	Context("with flow annotation in infrastruture", func() {
		infra := &extensionsv1alpha1.Infrastructure{}
		cluster := internalinfra.MakeCluster("11.0.0.0/16", "12.0.0.0/16", "europe", 1, 1)
		metav1.SetMetaDataAnnotation(&infra.ObjectMeta, infrastructure.AnnotationKeyUseFlow, "true")
		It("should use the FlowReconciler", func() {
			Expect(infrastructure.HasFlowAnnotation(infra, cluster)).To(BeTrue())
		})
	})
	Context("with flow annotation in shoot", func() {
		cluster := internalinfra.MakeCluster("11.0.0.0/16", "12.0.0.0/16", "europe", 1, 1)
		metav1.SetMetaDataAnnotation(&cluster.Shoot.ObjectMeta, infrastructure.AnnotationKeyUseFlow, "true")
		It("should use the FlowReconciler", func() {
			Expect(infrastructure.HasFlowAnnotation(&extensionsv1alpha1.Infrastructure{}, cluster)).To(BeTrue())
		})
	})
	Context("with flow annotation in seed", func() {
		cluster := internalinfra.MakeCluster("11.0.0.0/16", "12.0.0.0/16", "europe", 1, 1)
		cluster.Seed = &v1beta1.Seed{}
		metav1.SetMetaDataAnnotation(&cluster.Seed.ObjectMeta, infrastructure.AnnotationKeyUseFlow, "true")
		It("should use the FlowReconciler", func() {
			Expect(infrastructure.HasFlowAnnotation(&extensionsv1alpha1.Infrastructure{}, cluster)).To(BeTrue())
		})
	})
})

var _ = Describe("ReconcilationStrategy", func() {
	cluster := internalinfra.MakeCluster("11.0.0.0/16", "12.0.0.0/16", "europe", 1, 1)
	It("should use Flow if an annotation is found", func() {
		infra := &extensionsv1alpha1.Infrastructure{}
		metav1.SetMetaDataAnnotation(&infra.ObjectMeta, infrastructure.AnnotationKeyUseFlow, "true")

		sut := infrastructure.StrategySelector{}
		useFlow, err := sut.ShouldReconcileWithFlow(infra, cluster)
		Expect(err).NotTo(HaveOccurred())
		Expect(useFlow).To(BeTrue())
	})
	It("should use Flow if resources were reconciled with Flow before, regardless of annotation", func() {
		emptyState := shared.NewPersistentState()
		stateRaw, err := emptyState.ToJSON()
		Expect(err).NotTo(HaveOccurred())
		infra := &extensionsv1alpha1.Infrastructure{}
		infra.Status.State = &runtime.RawExtension{Raw: stateRaw}

		sut := infrastructure.StrategySelector{}
		useFlow, err := sut.ShouldReconcileWithFlow(infra, cluster)
		Expect(err).NotTo(HaveOccurred())
		Expect(useFlow).To(BeTrue())
	})
	It("should use Terraform if no flow state is found and there is no flow annotation", func() {
		infra := &extensionsv1alpha1.Infrastructure{}
		infra.Status.State = &runtime.RawExtension{Raw: getRawTerraformState(`{"provider": "terraform"}`)}

		sut := infrastructure.StrategySelector{}
		useFlow, err := sut.ShouldReconcileWithFlow(infra, cluster)
		Expect(err).NotTo(HaveOccurred())
		Expect(useFlow).To(BeFalse())
	})

	It("should delete with Terraform if resources were reconciled with Terraform", func() {
		useFlow := false
		stateRaw := getRawTerraformState(`{"provider": "terraform"}`)

		infra := &extensionsv1alpha1.Infrastructure{}
		ctrl := gomock.NewController(GinkgoT())
		mockClient, patchedInfra := expectStatusAndStatePatch(ctrl, infra, stateRaw)

		sut := infrastructure.StrategySelector{
			Factory: MockFactory{ctrl, stateRaw},
			Client:  mockClient,
		}
		err := sut.Reconcile(useFlow, context.TODO(), infra, &azure.InfrastructureConfig{}, cluster)
		Expect(err).NotTo(HaveOccurred())

		deleteWithFlow, err := sut.ShouldDeleteWithFlow(patchedInfra.Status)
		Expect(err).NotTo(HaveOccurred())

		Expect(deleteWithFlow).To(BeFalse())
	})
	It("should delete with Flow if resources were reconciled with Flow", func() {
		useFlow := true
		emptyState := shared.NewPersistentState()
		stateRaw, err := emptyState.ToJSON()
		Expect(err).NotTo(HaveOccurred())

		infra := &extensionsv1alpha1.Infrastructure{}
		ctrl := gomock.NewController(GinkgoT())
		mockClient, patchedInfra := expectStatusAndStatePatch(ctrl, infra, stateRaw)
		sut := infrastructure.StrategySelector{
			Factory: MockFactory{ctrl, stateRaw},
			Client:  mockClient,
		}

		err = sut.Reconcile(useFlow, context.TODO(), infra, &azure.InfrastructureConfig{}, cluster)
		Expect(err).NotTo(HaveOccurred())

		resFlow, err := sut.ShouldDeleteWithFlow(patchedInfra.Status)
		Expect(err).NotTo(HaveOccurred())

		Expect(resFlow).To(BeTrue())
	})

})

func getRawTerraformState(jsonContent string) []byte {
	state := infrastructure.InfrastructureState{
		TerraformState: &runtime.RawExtension{
			Raw: []byte(jsonContent),
		},
	}
	stateRaw, _ := json.Marshal(state)
	return stateRaw
}

func expectStatusAndStatePatch(ctrl *gomock.Controller, infra *extensionsv1alpha1.Infrastructure, expectedTfStateRaw []byte) (*mockclient.MockClient, *extensionsv1alpha1.Infrastructure) {
	mClient := mockclient.NewMockClient(ctrl)
	sw := mockclient.NewMockStatusWriter(ctrl)
	mClient.EXPECT().Status().Return(sw).AnyTimes()

	patchedInfra := infra.DeepCopy()
	patchedInfra.Status.State = &runtime.RawExtension{Raw: expectedTfStateRaw}
	patchedInfra.Status.ProviderStatus = &runtime.RawExtension{Object: &v1alpha1.InfrastructureStatus{}} // reconciler mock returns an empty status
	// expect patch with new State and Status
	sw.EXPECT().Patch(gomock.Any(), EqMatcher(patchedInfra), gomock.Any()).Return(nil)
	return mClient, patchedInfra
}

type MockFactory struct {
	*gomock.Controller
	tfState []byte
}

func (f MockFactory) Build(useFlow bool) (infrastructure.Reconciler, error) {
	reconciler := imock.NewMockReconciler(f.Controller)
	reconciler.EXPECT().Reconcile(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(&v1alpha1.InfrastructureStatus{}, nil)
	if useFlow {
		emptyState := shared.NewPersistentState()
		byteState, err := emptyState.ToJSON()
		if err != nil {
			panic(err)
		}
		reconciler.EXPECT().GetState(gomock.Any(), gomock.Any()).Return(byteState, nil).AnyTimes()
	} else {
		reconciler.EXPECT().GetState(gomock.Any(), gomock.Any()).Return(f.tfState, nil).AnyTimes()
	}
	return reconciler, nil
}

type eqMatcher struct {
	want interface{}
}

func EqMatcher(want interface{}) eqMatcher {
	return eqMatcher{
		want: want,
	}
}

func (eq eqMatcher) Matches(got interface{}) bool {
	return gomock.Eq(eq.want).Matches(got)
}

func (eq eqMatcher) Got(got interface{}) string {
	return fmt.Sprintf("%v (%T)\nDiff (-got +want):\n%s", got, got, strings.TrimSpace(cmp.Diff(got, eq.want)))
}

func (eq eqMatcher) String() string {
	return fmt.Sprintf("%v (%T)\n", eq.want, eq.want)
}
