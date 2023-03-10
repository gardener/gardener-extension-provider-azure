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

package infraflow_test

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"

	mockclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client/mock"
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
	return aset.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, gomock.Any(), params).Return(&armcompute.AvailabilitySet{}, nil)
}

func (f *MockFactoryWrapper) assertRouteTableCalled(name string) *gomock.Call {
	rt := mockclient.NewMockRouteTables(f.ctrl)
	f.EXPECT().RouteTables().Return(rt, nil)
	return rt.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, name, gomock.Any()).Return(
		&armnetwork.RouteTable{
			ID: to.Ptr("routeId"),
		}, nil)
}

func (f *MockFactoryWrapper) assertSecurityGroupCalled(name string) *gomock.Call {
	sg := mockclient.NewMockNetworkSecurityGroup(f.ctrl)
	f.EXPECT().NetworkSecurityGroup().Return(sg, nil)
	return sg.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, name, gomock.Any()).Return(&armnetwork.SecurityGroup{
		ID: to.Ptr("sgId"),
	}, nil)
}

func (f *MockFactoryWrapper) assertVnetCalledWithParameters(name string, params interface{}) *gomock.Call {
	vnet := mockclient.NewMockVnet(f.ctrl)
	f.EXPECT().Vnet().Return(vnet, nil)
	return vnet.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, name, params).Return(nil, nil)
}

func (f *MockFactoryWrapper) VnetFactoryCalled() *gomock.Call {
	vnet := mockclient.NewMockVnet(f.ctrl)
	return f.EXPECT().Vnet().Return(vnet, nil)
}

func (f *MockFactoryWrapper) assertSubnetCalled(vnetName string, name interface{}) *gomock.Call {
	subnet := mockclient.NewMockSubnet(f.ctrl)
	f.EXPECT().Subnet().Return(subnet, nil)
	return subnet.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, vnetName, name, gomock.Any()).Return(nil, nil)
}

func (f *MockFactoryWrapper) assertNatGatewayCalledWithParameters(name string, params interface{}) *gomock.Call {
	nat := mockclient.NewMockNatGateway(f.ctrl)
	f.EXPECT().NatGateway().Return(nat, nil)
	nat.EXPECT().List(gomock.Any(), f.resourceGroup).Return([]*armnetwork.NatGateway{}, nil).AnyTimes() // simple fake (deletion not tested in mocks)
	return nat.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, name, params).Return(&armnetwork.NatGateway{ID: to.Ptr("natId")}, nil)
}

func (f *MockFactoryWrapper) assertPublicIPCalledWithoutCreation() *gomock.Call {
	ip := mockclient.NewMockPublicIP(f.ctrl)
	f.EXPECT().PublicIP().Return(ip, nil)
	return ip.EXPECT().List(gomock.Any(), f.resourceGroup).Return([]*armnetwork.PublicIPAddress{}, nil).AnyTimes() // simple fake (deletion not tested in mocks)
}

func (f *MockFactoryWrapper) assertPublicIPCalledWithParameters(name interface{}, params interface{}) *gomock.Call {
	ip := mockclient.NewMockPublicIP(f.ctrl)
	f.EXPECT().PublicIP().Return(ip, nil)
	ip.EXPECT().List(gomock.Any(), f.resourceGroup).Return([]*armnetwork.PublicIPAddress{}, nil).AnyTimes() // simple fake (deletion not tested in mocks)
	return ip.EXPECT().CreateOrUpdate(gomock.Any(), f.resourceGroup, name, params).Return(&armnetwork.PublicIPAddress{ID: to.Ptr("ipId")}, nil)
}

func (f *MockFactoryWrapper) assertPublicIPGet(resourceGroup, name interface{}) *gomock.Call {
	ip := mockclient.NewMockPublicIP(f.ctrl)
	f.EXPECT().PublicIP().Return(ip, nil)
	return ip.EXPECT().Get(gomock.Any(), resourceGroup, name).Return(&armnetwork.PublicIPAddress{ID: to.Ptr("my-id")}, nil)
}
