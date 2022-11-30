package infraflow_test

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("TfAdapter", func() {
	location := "westeurope"
	clusterName := "test_cluster"
	infra := &v1alpha1.Infrastructure{Spec: v1alpha1.InfrastructureSpec{Region: location}, ObjectMeta: metav1.ObjectMeta{Namespace: clusterName}}
	cluster := infrastructure.MakeCluster("11.0.0.0/16", "12.0.0.0/16", infra.Spec.Region, 1, 1)
	It("should return the Identity information", func() {
		cfg := newBasicConfig()
		sut, err := infraflow.NewTerraformAdapter(infra, cfg, cluster)
		Expect(err).ToNot(HaveOccurred())
		res := sut.Identity()
		Expect(res).To(BeNil())
	})
	It("should return NAT config for single subnet", func() {
		cfg := newBasicConfig()
		cfg.Networks.NatGateway = &azure.NatGatewayConfig{
			Zone:    to.Ptr(int32(1)),
			Enabled: true,
		}
		sut, err := infraflow.NewTerraformAdapter(infra, cfg, cluster)
		Expect(err).ToNot(HaveOccurred())
		res := sut.Nats()
		Expect(res).NotTo(BeEmpty())
	})
})
