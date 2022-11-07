package infraflow_test

import (
	"context"
	"encoding/base64"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	mockclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client/mock"
	"github.com/golang/mock/gomock"

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
	Context("with resource group, vnet and route table in cfg", func() {
		resourceGroupName := "t-i545428"
		location := "westeurope"
		vnetName := "vnet-i545428"

		cfg := &azure.InfrastructureConfig{
			ResourceGroup: &azure.ResourceGroup{Name: resourceGroupName},
			Networks: azure.NetworkConfig{VNet: azure.VNet{Name: to.Ptr(vnetName),
				CIDR: to.Ptr("10.0.0.0/8")}},
		}
		infra := &v1alpha1.Infrastructure{Spec: v1alpha1.InfrastructureSpec{Region: location}}

		var factory *mockclient.MockNewFactory
		BeforeEach(func() {
			ctrl := gomock.NewController(GinkgoT())
			factory = mockclient.NewMockNewFactory(ctrl)

			rgroup := mockclient.NewMockResourceGroup(ctrl)
			createGroup := rgroup.EXPECT().CreateOrUpdate(gomock.Any(), resourceGroupName, location).Return(nil)
			factory.EXPECT().ResourceGroup().Return(rgroup, nil)
			vnet := mockclient.NewMockVnet(ctrl)
			createVnet := vnet.EXPECT().CreateOrUpdate(gomock.Any(), resourceGroupName, vnetName, gomock.Any()).Return(nil).After(createGroup) // TODO check location ?
			factory.EXPECT().Vnet().Return(vnet, nil)

			rt := mockclient.NewMockRouteTables(ctrl)
			factory.EXPECT().RouteTables().Return(rt, nil)
			rt.EXPECT().CreateOrUpdate(gomock.Any(), resourceGroupName, gomock.Any(), gomock.Any()).Return(nil).After(createVnet) // TODO check location ?
		})
		It("should reconcile all resources", func() {
			sut := infraflow.FlowReconciler{Factory: factory}
			err := sut.Reconcile(context.TODO(), infra, cfg)
			Expect(err).To(BeNil())
		})
	})

})

//var _ = Describe("FlowReconciler", func() {
//	Context("with resource group and vnet in cfg", func() {
//		resourceGroupName := "t-i545428"
//		vnetName := "vnet-i545428"
//		cfg := &azure.InfrastructureConfig{
//			ResourceGroup: &azure.ResourceGroup{Name: resourceGroupName},
//			Networks: azure.NetworkConfig{VNet: azure.VNet{Name: to.Ptr(vnetName),
//				CIDR: to.Ptr("10.0.0.0/8")}},
//		}
//		infra := &v1alpha1.Infrastructure{Spec: v1alpha1.InfrastructureSpec{Region: "westeurope"}}

//		auth := readAuthFromFile("/Users/I545428/dev/azsecret.yaml")
//		factory, err := client.NewAzureClientFactoryV2(auth)
//		Expect(err).To(BeNil())
//		rclient, err := factory.ResourceGroup()
//		Expect(err).To(BeNil())
//		vclient, err := factory.Vnet()
//		Expect(err).To(BeNil())
//		It("should reconcile all resources", func() {
//			sut := infraflow.FlowReconciler{Factory: factory}
//			err = sut.Reconcile(context.TODO(), infra, cfg)
//			Expect(err).To(BeNil())

//			exists, err := rclient.IsExisting(context.TODO(), resourceGroupName)
//			Expect(err).To(BeNil())
//			Expect(exists).To(BeTrue())

//			vnet, err := vclient.Get(context.TODO(), resourceGroupName, vnetName)
//			Expect(err).To(BeNil())
//			Expect(*vnet.Name).To(Equal(vnetName))
//		})
//		AfterEach(func() {
//			err := rclient.Delete(context.TODO(), resourceGroupName)
//			Expect(err).NotTo(HaveOccurred())
//		})

//	})

//})

func readAuthFromFile(fileName string) internal.ClientAuth {
	secret := ProviderSecret{}
	data, err := os.ReadFile(fileName)
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
