package infrastructure_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure"
	internalinfra "github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

var _ = Describe("Flowreconciler", func() {
	Context("with flow annotation in infrastruture", func() {
		infra := &extensionsv1alpha1.Infrastructure{}
		metav1.SetMetaDataAnnotation(&infra.ObjectMeta, infrastructure.AnnotationKeyUseFlow, "true")
		It("should use the FlowReconciler", func() {
			Expect(infrastructure.ShouldUseFlow(infra, nil)).To(BeTrue())
		})
	})
	Context("without flow annotation in infrastruture nor in shoot", func() {
		It("should not use FlowReconciler", func() {
			cluster := internalinfra.MakeCluster("11.0.0.0/16", "12.0.0.0/16", "europe", 1, 1)
			Expect(infrastructure.ShouldUseFlow(&extensionsv1alpha1.Infrastructure{}, cluster)).To(BeFalse())
		})
	})
	Context("with flow annotation in shoot", func() {
		cluster := internalinfra.MakeCluster("11.0.0.0/16", "12.0.0.0/16", "europe", 1, 1)
		metav1.SetMetaDataAnnotation(&cluster.Shoot.ObjectMeta, infrastructure.AnnotationKeyUseFlow, "true")
		It("should use the FlowReconciler", func() {
			Expect(infrastructure.ShouldUseFlow(&extensionsv1alpha1.Infrastructure{}, cluster)).To(BeTrue())
		})
	})
})
