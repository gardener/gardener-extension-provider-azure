package infraflow_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
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

var _ = Describe("FlowReconciler", func() {
	resourceGroupName := "t-i545428" // TODO what if resource group not given? by default Tf uses infra.Namespace
	location := "westeurope"
	vnetName := "vnet-i545428" // TODO test if not given default infra.Namespace
	clusterName := "test_cluster"
	infra := &v1alpha1.Infrastructure{Spec: v1alpha1.InfrastructureSpec{Region: location}, ObjectMeta: metav1.ObjectMeta{Namespace: clusterName}}
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
			Zones:            []azure.Zone{{Name: 1, CIDR: "10.0.0.0/16", NatGateway: &azure.ZonedNatGatewayConfig{Enabled: true, IPAddresses: []azure.ZonedPublicIPReference{{Name: "my-ip", ResourceGroup: resourceGroupName}}}}, {Name: 2, CIDR: "10.1.0.0/16"}}, // subnets
		},
	}
	cluster := infrastructure.MakeCluster("11.0.0.0/16", "12.0.0.0/16", infra.Spec.Region, 1, 1)

	var factory *mockclient.MockNewFactory
	Context("with resource group, vnet, route table, security group, subnet, nat and ip in cfg", func() {
		BeforeEach(func() {
			mock := NewMockFactoryWrapper(resourceGroupName, location)
			createGroup := mock.assertResourceGroupCalled()
			mock.assertVnetCalled(vnetName).After(createGroup)
			createRoutes := mock.assertRouteTableCalled("worker_route_table").After(createGroup)
			createSgroup := mock.assertSecurityGroupCalled(infra.Namespace + "-workers").After(createGroup)
			// workaround: issue with arg order https://github.com/golang/mock/issues/653
			createIps := mock.assertPublicIPCalled("my-ip").Times(2).After(createGroup)
			createNats := mock.assertNatGatewayCalledWith("test_cluster-nat-gateway-z1").After(createGroup).After(createIps)
			mock.assertSubnetCalled(vnetName, MatchAnyOfStrings([]string{"test_cluster-z2", "test_cluster-z1"})).After(createRoutes).After(createSgroup).After(createNats).Times(2)
			factory = mock.GetFactory()
		})
		It("should reconcile all resources", func() {
			cfg.Zoned = true // no availability set
			sut := infraflow.FlowReconciler{Factory: factory}
			err := sut.Reconcile(context.TODO(), infra, cfg, cluster)
			Expect(err).To(BeNil())
		})
	})
	Context("with resource group, vnet, route table, security group, subnet, nat, ip, availabilitySet in cfg", func() {
		BeforeEach(func() {
			mock := NewMockFactoryWrapper(resourceGroupName, location)
			createGroup := mock.assertResourceGroupCalled()
			mock.assertVnetCalled(vnetName).After(createGroup)
			createRoutes := mock.assertRouteTableCalled("worker_route_table").After(createGroup)
			createSgroup := mock.assertSecurityGroupCalled(infra.Namespace + "-workers").After(createGroup)
			// workaround: issue with arg order https://github.com/golang/mock/issues/653
			createIps := mock.assertPublicIPCalled("my-ip").Times(2).After(createGroup)
			createNats := mock.assertNatGatewayCalledWith("test_cluster-nat-gateway-z1").After(createGroup).After(createIps)
			mock.assertSubnetCalled(vnetName, MatchAnyOfStrings([]string{"test_cluster-z2", "test_cluster-z1"})).After(createRoutes).After(createSgroup).After(createNats).Times(2)
			mock.assertAvailabilitySetCalled(clusterName + "-avset-workers")
			factory = mock.GetFactory()
		})
		It("should reconcile all resources", func() {
			cfg.Zoned = false // for availability set
			sut := infraflow.FlowReconciler{Factory: factory}
			err := sut.Reconcile(context.TODO(), infra, cfg, cluster)
			Expect(err).To(BeNil())
		})
	})

})

type MatchAnyOfStrings ([]string)

func (m MatchAnyOfStrings) Matches(x interface{}) bool {
	for _, v := range m {
		if v == x.(string) {
			return true
		}
	}
	return false
}

func (m MatchAnyOfStrings) String() string {
	return fmt.Sprintf("is one of %v", []string(m))
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

func (f *MockFactoryWrapper) assertAvailabilitySetCalled(name string) *gomock.Call {
	aset := mockclient.NewMockAvailabilitySet(f.ctrl)
	f.EXPECT().AvailabilitySet().Return(aset, nil)
	return aset.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, gomock.Any(), gomock.Any()).Return(armcompute.AvailabilitySetsClientCreateOrUpdateResponse{}, nil)
}
func (f *MockFactoryWrapper) assertResourceGroupCalled() *gomock.Call {
	rgroup := mockclient.NewMockResourceGroup(f.ctrl)
	f.EXPECT().ResourceGroup().Return(rgroup, nil)
	return rgroup.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, f.location).Return(nil)
}

func (f *MockFactoryWrapper) assertRouteTableCalled(name string) *gomock.Call {
	rt := mockclient.NewMockRouteTables(f.ctrl)
	f.EXPECT().RouteTables().Return(rt, nil)
	return rt.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, name, gomock.Any()).Return(armnetwork.RouteTablesClientCreateOrUpdateResponse{}, nil)
}

func (f *MockFactoryWrapper) assertSecurityGroupCalled(name string) *gomock.Call {
	sg := mockclient.NewMockSecurityGroups(f.ctrl)
	f.EXPECT().SecurityGroups().Return(sg, nil)
	return sg.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, name, gomock.Any()).Return(armnetwork.SecurityGroupsClientCreateOrUpdateResponse{}, nil)
}

func (f *MockFactoryWrapper) assertVnetCalled(name string) *gomock.Call {
	vnet := mockclient.NewMockVnet(f.ctrl)
	f.EXPECT().Vnet().Return(vnet, nil)
	return vnet.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, name, gomock.Any()).Return(nil)
}

func (f *MockFactoryWrapper) assertSubnetCalled(vnetName string, name interface{}) *gomock.Call {
	subnet := mockclient.NewMockSubnet(f.ctrl)
	f.EXPECT().Subnet().Return(subnet, nil)
	return subnet.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, vnetName, name, gomock.Any()).Return(nil)
}

func (f *MockFactoryWrapper) assertNatGatewayCalledWith(name string) *gomock.Call {
	nat := mockclient.NewMockNatGateway(f.ctrl)
	f.EXPECT().NatGateway().Return(nat, nil)
	return nat.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, name, gomock.Any()).Return(armnetwork.NatGatewaysClientCreateOrUpdateResponse{NatGateway: armnetwork.NatGateway{ID: to.Ptr("natId")}}, nil)
}

func (f *MockFactoryWrapper) assertPublicIPCalled(name string) *gomock.Call {
	ip := mockclient.NewMockNewPublicIP(f.ctrl)
	f.EXPECT().PublicIP().Return(ip, nil)
	return ip.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, gomock.Any(), gomock.Any()).Return(armnetwork.PublicIPAddressesClientCreateOrUpdateResponse{PublicIPAddress: armnetwork.PublicIPAddress{ID: to.Ptr("ipId")}}, nil)
}

type ProviderSecret struct {
	Data internal.ClientAuth `yaml:"data"`
}

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
