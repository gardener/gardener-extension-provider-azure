package infraflow_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	mockclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client/mock"
	"github.com/golang/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

type ProviderSecret struct {
	Data internal.ClientAuth `yaml:"data"`
}

var _ = Describe("FlowReconciler", func() {
	Context("with resource group, vnet, route table, security group, subnet, nat in cfg", func() {
		resourceGroupName := "t-i545428" // TODO what if resource group not given? by default Tf uses infra.Namespace
		location := "westeurope"
		vnetName := "vnet-i545428" // TODO test if not given default infra.Namespace

		cfg := &azure.InfrastructureConfig{
			ResourceGroup: &azure.ResourceGroup{Name: resourceGroupName},
			Networks: azure.NetworkConfig{
				VNet: azure.VNet{
					Name:          to.Ptr(vnetName),
					ResourceGroup: to.Ptr(resourceGroupName),
					CIDR:          to.Ptr("10.0.0.0/8"),
				},
				Workers:          to.Ptr("10.0.0.0/16"),
				ServiceEndpoints: []string{},
				Zones:            []azure.Zone{{Name: 1, CIDR: "10.0.0.0/16", NatGateway: &azure.ZonedNatGatewayConfig{Enabled: true}}, {Name: 2, CIDR: "10.1.0.0/16"}}, // subnets
			},
		}
		infra := &v1alpha1.Infrastructure{Spec: v1alpha1.InfrastructureSpec{Region: location}, ObjectMeta: metav1.ObjectMeta{Namespace: "test_cluster"}}

		cluster := infrastructure.MakeCluster("11.0.0.0/16", "12.0.0.0/16", infra.Spec.Region, 1, 1)
		var factory *mockclient.MockNewFactory
		BeforeEach(func() {
			mock := NewMockFactoryWrapper(resourceGroupName, location)
			createGroup := mock.assertResourceGroupCalled()
			mock.assertVnetCalledWith(vnetName).After(createGroup)
			createRoutes := mock.assertRouteTableCalledWith("worker_route_table").After(createGroup)
			createNats := mock.assertNatGatewayCalledWith("test_cluster-nat-gateway-z1").After(createGroup)
			createSgroup := mock.assertSecurityGroupCalledWith(infra.Namespace + "-workers").After(createGroup)
			// workaround: issue with arg order https://github.com/golang/mock/issues/653
			mock.assertSubnetCalledWith(vnetName, MatchAnyOfStrings([]string{"test_cluster-z2", "test_cluster-z1"})).After(createRoutes).After(createSgroup).After(createNats).Times(2)
			factory = mock.GetFactory()
		})
		It("should reconcile all resources", func() {
			sut := infraflow.FlowReconciler{Factory: factory}
			err := sut.Reconcile(context.TODO(), infra, cfg, cluster)
			Expect(err).To(BeNil())
		})
	})

})

type MatchAnyOfStrings ([]string)

func (m *MatchAnyOfStrings) Matches(x interface{}) bool {
	for _, v := range *m {
		if v == x.(string) {
			return true
		}
	}
	return false
}

func (m *MatchAnyOfStrings) String() string {
	return fmt.Sprintf("is one of %v", *m)
}

type MockFactoryWrapper struct {
	ctrl *gomock.Controller
	*mockclient.MockNewFactory
	resourceGroup string
	location      string
}

func (f *MockFactoryWrapper) GetFactory() *mockclient.MockNewFactory {
	return f.MockNewFactory
}

func NewMockFactoryWrapper(resourceGroup, location string) *MockFactoryWrapper {
	ctrl := gomock.NewController(GinkgoT())
	factory := mockclient.NewMockNewFactory(ctrl)
	return &MockFactoryWrapper{ctrl, factory, resourceGroup, location}
}

func (f *MockFactoryWrapper) assertResourceGroupCalled() *gomock.Call {
	rgroup := mockclient.NewMockResourceGroup(f.ctrl)
	f.EXPECT().ResourceGroup().Return(rgroup, nil)
	return rgroup.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, f.location).Return(nil)
}

func (f *MockFactoryWrapper) assertRouteTableCalledWith(name string) *gomock.Call {
	rt := mockclient.NewMockRouteTables(f.ctrl)
	f.EXPECT().RouteTables().Return(rt, nil)
	return rt.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, name, gomock.Any()).Return(armnetwork.RouteTablesClientCreateOrUpdateResponse{}, nil)
}

func (f *MockFactoryWrapper) assertSecurityGroupCalledWith(name string) *gomock.Call {
	sg := mockclient.NewMockSecurityGroups(f.ctrl)
	f.EXPECT().SecurityGroups().Return(sg, nil)
	return sg.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, name, gomock.Any()).Return(armnetwork.SecurityGroupsClientCreateOrUpdateResponse{}, nil)
}

func (f *MockFactoryWrapper) assertVnetCalledWith(name string) *gomock.Call {
	vnet := mockclient.NewMockVnet(f.ctrl)
	f.EXPECT().Vnet().Return(vnet, nil)
	return vnet.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, name, gomock.Any()).Return(nil)
}

func (f *MockFactoryWrapper) assertSubnetCalledWith(vnetName string, name interface{}) *gomock.Call {
	subnet := mockclient.NewMockSubnet(f.ctrl)
	f.EXPECT().Subnet().Return(subnet, nil)
	return subnet.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, vnetName, name, gomock.Any()).Return(nil)
}

func (f *MockFactoryWrapper) assertNatGatewayCalledWith(name string) *gomock.Call {
	nat := mockclient.NewMockNatGateway(f.ctrl)
	f.EXPECT().NatGateway().Return(nat, nil)
	return nat.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, name, gomock.Any()).Return(armnetwork.NatGatewaysClientCreateOrUpdateResponse{NatGateway: armnetwork.NatGateway{ID: to.Ptr("natId")}}, nil)
}

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
