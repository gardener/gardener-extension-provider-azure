package infraflow_test

import (
	"context"
	"encoding/base64"
	"io/ioutil"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
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
		vnetName := "vnet-i545428"
		cfg := &azure.InfrastructureConfig{
			ResourceGroup: &azure.ResourceGroup{Name: resourceGroupName},
			Networks: azure.NetworkConfig{VNet: azure.VNet{Name: to.Ptr(vnetName),
				CIDR: to.Ptr("10.0.0.0/8")}},
		}
		infra := &v1alpha1.Infrastructure{Spec: v1alpha1.InfrastructureSpec{Region: "westeurope"}}

		auth := readAuthFromFile("/Users/I545428/dev/azsecret.yaml")
		factory, err := client.NewAzureClientFactoryV2(auth)
		Expect(err).To(BeNil())
		rclient, err := factory.ResourceGroup()
		Expect(err).To(BeNil())
		vclient, err := factory.Vnet()
		Expect(err).To(BeNil())
		It("should reconcile resource group and vnet", func() {
			sut := infraflow.FlowReconciler{Factory: factory}
			err = sut.Reconcile(context.TODO(), infra, cfg)
			Expect(err).To(BeNil())

			exists, err := rclient.IsExisting(context.TODO(), resourceGroupName)
			Expect(err).To(BeNil())
			Expect(exists).To(BeTrue())

			vnet, err := vclient.Get(context.TODO(), resourceGroupName, vnetName)
			Expect(err).To(BeNil())
			Expect(*vnet.Name).To(Equal(vnetName))
		})
		AfterEach(func() {
			err := rclient.Delete(context.TODO(), resourceGroupName)
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
