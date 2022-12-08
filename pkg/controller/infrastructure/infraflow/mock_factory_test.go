package infraflow_test

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-05-01/network"
	mockclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client/mock"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
)

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
	*mockclient.MockFactory
	resourceGroup string
	location      string
}

func (f *MockFactoryWrapper) GetFactory() *mockclient.MockFactory {
	return f.MockFactory
}

func NewMockFactoryWrapper(resourceGroup, location string) *MockFactoryWrapper {
	ctrl := gomock.NewController(GinkgoT())
	factory := mockclient.NewMockFactory(ctrl)
	return &MockFactoryWrapper{ctrl, factory, resourceGroup, location}
}

func (f *MockFactoryWrapper) assertAvailabilitySetCalledWithParameters(name string, params interface{}) *gomock.Call {
	aset := mockclient.NewMockAvailabilitySet(f.ctrl)
	f.EXPECT().AvailabilitySet().Return(aset, nil)
	return aset.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, gomock.Any(), params).Return(armcompute.AvailabilitySetsClientCreateOrUpdateResponse{}, nil)
}
func (f *MockFactoryWrapper) assertResourceGroupCalled() *gomock.Call {
	rgroup := mockclient.NewMockResourceGroup(f.ctrl)
	f.EXPECT().Group().Return(rgroup, nil)
	return rgroup.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, f.location).Return(nil)
}

func (f *MockFactoryWrapper) assertRouteTableCalled(name string) *gomock.Call {
	rt := mockclient.NewMockRouteTables(f.ctrl)
	f.EXPECT().RouteTables().Return(rt, nil)
	return rt.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, name, gomock.Any()).Return(
		armnetwork.RouteTablesClientCreateOrUpdateResponse{
			RouteTable: armnetwork.RouteTable{
				ID: to.Ptr("routeId"),
			},
		}, nil)
}

func (f *MockFactoryWrapper) assertSecurityGroupCalled(name string) *gomock.Call {
	sg := mockclient.NewMockNetworkSecurityGroup(f.ctrl)
	f.EXPECT().NetworkSecurityGroup().Return(sg, nil)
	return sg.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, name, gomock.Any()).Return(&network.SecurityGroup{
		ID: to.Ptr("sgId"),
	}, nil)
}

func (f *MockFactoryWrapper) assertVnetCalled(name string) *gomock.Call {
	return f.assertVnetCalledWithParameters(name, gomock.Any())
}

func (f *MockFactoryWrapper) assertVnetCalledWithParameters(name string, params interface{}) *gomock.Call {
	vnet := mockclient.NewMockVnet(f.ctrl)
	f.EXPECT().Vnet().Return(vnet, nil)
	return vnet.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, name, params).Return(nil)
}

func (f *MockFactoryWrapper) VnetFactoryCalled() {
	vnet := mockclient.NewMockVnet(f.ctrl)
	f.EXPECT().Vnet().Return(vnet, nil)
	//return vnet
}

func (f *MockFactoryWrapper) assertSubnetCalled(vnetName string, name interface{}) *gomock.Call {
	subnet := mockclient.NewMockSubnet(f.ctrl)
	f.EXPECT().Subnet().Return(subnet, nil)
	return subnet.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, vnetName, name, gomock.Any()).Return(nil)
}

func (f *MockFactoryWrapper) assertNatGatewayCalledWithParameters(name string, params interface{}) *gomock.Call {
	nat := mockclient.NewMockNatGateway(f.ctrl)
	f.EXPECT().NatGateway().Return(nat, nil)
	nat.EXPECT().GetAll(gomock.Any(), f.resourceGroup).Return([]*armnetwork.NatGateway{}, nil).AnyTimes() // simple fake (deletion not tested in mocks)
	return nat.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, name, params).Return(armnetwork.NatGatewaysClientCreateOrUpdateResponse{NatGateway: armnetwork.NatGateway{ID: to.Ptr("natId")}}, nil)
}

func (f *MockFactoryWrapper) assertPublicIPCalledWithoutCreation() *gomock.Call {
	ip := mockclient.NewMockPublicIP(f.ctrl)
	f.EXPECT().PublicIP().Return(ip, nil)
	return ip.EXPECT().GetAll(gomock.Any(), f.resourceGroup).Return([]network.PublicIPAddress{}, nil).AnyTimes() // simple fake (deletion not tested in mocks)
}

func (f *MockFactoryWrapper) assertPublicIPCalledWithParameters(name interface{}, params interface{}) *gomock.Call {
	ip := mockclient.NewMockPublicIP(f.ctrl)
	f.EXPECT().PublicIP().Return(ip, nil)
	ip.EXPECT().GetAll(gomock.Any(), f.resourceGroup).Return([]network.PublicIPAddress{}, nil).AnyTimes() // simple fake (deletion not tested in mocks)
	return ip.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, name, params).Return(&network.PublicIPAddress{ID: to.Ptr("ipId")}, nil)
}

func (f *MockFactoryWrapper) assertPublicIPGet(resourceGroup, name interface{}) *gomock.Call {
	ip := mockclient.NewMockPublicIP(f.ctrl)
	f.EXPECT().PublicIP().Return(ip, nil)
	return ip.EXPECT().Get(gomock.Any(), resourceGroup, name, "").Return(&network.PublicIPAddress{ID: to.Ptr("my-id")}, nil)
}
