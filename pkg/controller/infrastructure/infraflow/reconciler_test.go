package infraflow_test

import (
	"context"
	"encoding/base64"
	"io/ioutil"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

type ProviderSecret struct {
	Data internal.ClientAuth `yaml:"data"`
}

var _ = Describe("FlowReconciler", func() {
	Context("with resource group in cfg", func() {
		resourceGroupName := "t-i545428"
		client, err := client.NewResourceGroupsClient(readAuthFromFile("/Users/I545428/dev/azsecret.yaml"))
		It("should reconcile resource group", func() {
			Expect(err).To(BeNil())

			sut := infraflow.FlowReconciler{Client: client}
			cfg := &azure.InfrastructureConfig{
				ResourceGroup: &azure.ResourceGroup{Name: resourceGroupName},
			}
			infra := &v1alpha1.Infrastructure{Spec: v1alpha1.InfrastructureSpec{Region: "westeurope"}}
			sut.Reconcile(context.TODO(), infra, cfg)

			exists, err := client.IsExisting(context.TODO(), resourceGroupName)
			Expect(err).To(BeNil())
			Expect(exists).To(BeTrue())
		})
		AfterEach(func() {
			err := client.Delete(context.TODO(), resourceGroupName)
			Expect(err).NotTo(HaveOccurred())
		})

	})

})

func readAuthFromFile(fileName string) internal.ClientAuth {
	secret := ProviderSecret{}
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(data, &secret)
	if err != nil {
		panic(err)
	}
	secret.Data.ClientID = decodeString(secret.Data.ClientID)
	secret.Data.ClientSecret = decodeString(secret.Data.ClientSecret)
	secret.Data.SubscriptionID = decodeString(secret.Data.SubscriptionID)
	secret.Data.TenantID = decodeString(secret.Data.TenantID)
	return secret.Data
}

func decodeString(s string) string {
	res, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return string(res)
}
