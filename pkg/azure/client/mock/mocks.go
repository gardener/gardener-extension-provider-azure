// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/gardener/gardener-extension-provider-azure/pkg/azure/client (interfaces: DNSZone,DNSRecordSet,Subnet,Factory,ResourceGroup,Vnet,RouteTables,NatGateway,PublicIP,AvailabilitySet,NetworkSecurityGroup,ManagedUserIdentity)

// Package client is a generated GoMock package.
package client

import (
	context "context"
	reflect "reflect"

	armcompute "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	armnetwork "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	armresources "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	msi "github.com/Azure/azure-sdk-for-go/services/msi/mgmt/2018-11-30/msi"
	client "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	gomock "github.com/golang/mock/gomock"
	v1 "k8s.io/api/core/v1"
)

// MockDNSZone is a mock of DNSZone interface.
type MockDNSZone struct {
	ctrl     *gomock.Controller
	recorder *MockDNSZoneMockRecorder
}

// MockDNSZoneMockRecorder is the mock recorder for MockDNSZone.
type MockDNSZoneMockRecorder struct {
	mock *MockDNSZone
}

// NewMockDNSZone creates a new mock instance.
func NewMockDNSZone(ctrl *gomock.Controller) *MockDNSZone {
	mock := &MockDNSZone{ctrl: ctrl}
	mock.recorder = &MockDNSZoneMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDNSZone) EXPECT() *MockDNSZoneMockRecorder {
	return m.recorder
}

// List mocks base method.
func (m *MockDNSZone) List(arg0 context.Context) (map[string]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List", arg0)
	ret0, _ := ret[0].(map[string]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockDNSZoneMockRecorder) List(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockDNSZone)(nil).List), arg0)
}

// MockDNSRecordSet is a mock of DNSRecordSet interface.
type MockDNSRecordSet struct {
	ctrl     *gomock.Controller
	recorder *MockDNSRecordSetMockRecorder
}

// MockDNSRecordSetMockRecorder is the mock recorder for MockDNSRecordSet.
type MockDNSRecordSetMockRecorder struct {
	mock *MockDNSRecordSet
}

// NewMockDNSRecordSet creates a new mock instance.
func NewMockDNSRecordSet(ctrl *gomock.Controller) *MockDNSRecordSet {
	mock := &MockDNSRecordSet{ctrl: ctrl}
	mock.recorder = &MockDNSRecordSetMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDNSRecordSet) EXPECT() *MockDNSRecordSetMockRecorder {
	return m.recorder
}

// CreateOrUpdate mocks base method.
func (m *MockDNSRecordSet) CreateOrUpdate(arg0 context.Context, arg1, arg2, arg3 string, arg4 []string, arg5 int64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateOrUpdate", arg0, arg1, arg2, arg3, arg4, arg5)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateOrUpdate indicates an expected call of CreateOrUpdate.
func (mr *MockDNSRecordSetMockRecorder) CreateOrUpdate(arg0, arg1, arg2, arg3, arg4, arg5 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateOrUpdate", reflect.TypeOf((*MockDNSRecordSet)(nil).CreateOrUpdate), arg0, arg1, arg2, arg3, arg4, arg5)
}

// Delete mocks base method.
func (m *MockDNSRecordSet) Delete(arg0 context.Context, arg1, arg2, arg3 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockDNSRecordSetMockRecorder) Delete(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockDNSRecordSet)(nil).Delete), arg0, arg1, arg2, arg3)
}

// MockSubnet is a mock of Subnet interface.
type MockSubnet struct {
	ctrl     *gomock.Controller
	recorder *MockSubnetMockRecorder
}

// MockSubnetMockRecorder is the mock recorder for MockSubnet.
type MockSubnetMockRecorder struct {
	mock *MockSubnet
}

// NewMockSubnet creates a new mock instance.
func NewMockSubnet(ctrl *gomock.Controller) *MockSubnet {
	mock := &MockSubnet{ctrl: ctrl}
	mock.recorder = &MockSubnetMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSubnet) EXPECT() *MockSubnetMockRecorder {
	return m.recorder
}

// CreateOrUpdate mocks base method.
func (m *MockSubnet) CreateOrUpdate(arg0 context.Context, arg1, arg2, arg3 string, arg4 armnetwork.Subnet) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateOrUpdate", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateOrUpdate indicates an expected call of CreateOrUpdate.
func (mr *MockSubnetMockRecorder) CreateOrUpdate(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateOrUpdate", reflect.TypeOf((*MockSubnet)(nil).CreateOrUpdate), arg0, arg1, arg2, arg3, arg4)
}

// Delete mocks base method.
func (m *MockSubnet) Delete(arg0 context.Context, arg1, arg2, arg3 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockSubnetMockRecorder) Delete(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockSubnet)(nil).Delete), arg0, arg1, arg2, arg3)
}

// Get mocks base method.
func (m *MockSubnet) Get(arg0 context.Context, arg1, arg2, arg3 string) (*armnetwork.SubnetsClientGetResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(*armnetwork.SubnetsClientGetResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockSubnetMockRecorder) Get(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockSubnet)(nil).Get), arg0, arg1, arg2, arg3)
}

// List mocks base method.
func (m *MockSubnet) List(arg0 context.Context, arg1, arg2 string) ([]*armnetwork.Subnet, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List", arg0, arg1, arg2)
	ret0, _ := ret[0].([]*armnetwork.Subnet)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockSubnetMockRecorder) List(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockSubnet)(nil).List), arg0, arg1, arg2)
}

// MockFactory is a mock of Factory interface.
type MockFactory struct {
	ctrl     *gomock.Controller
	recorder *MockFactoryMockRecorder
}

// MockFactoryMockRecorder is the mock recorder for MockFactory.
type MockFactoryMockRecorder struct {
	mock *MockFactory
}

// NewMockFactory creates a new mock instance.
func NewMockFactory(ctrl *gomock.Controller) *MockFactory {
	mock := &MockFactory{ctrl: ctrl}
	mock.recorder = &MockFactoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockFactory) EXPECT() *MockFactoryMockRecorder {
	return m.recorder
}

// AvailabilitySet mocks base method.
func (m *MockFactory) AvailabilitySet() (client.AvailabilitySet, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AvailabilitySet")
	ret0, _ := ret[0].(client.AvailabilitySet)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AvailabilitySet indicates an expected call of AvailabilitySet.
func (mr *MockFactoryMockRecorder) AvailabilitySet() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AvailabilitySet", reflect.TypeOf((*MockFactory)(nil).AvailabilitySet))
}

// DNSRecordSet mocks base method.
func (m *MockFactory) DNSRecordSet() (client.DNSRecordSet, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DNSRecordSet")
	ret0, _ := ret[0].(client.DNSRecordSet)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DNSRecordSet indicates an expected call of DNSRecordSet.
func (mr *MockFactoryMockRecorder) DNSRecordSet() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DNSRecordSet", reflect.TypeOf((*MockFactory)(nil).DNSRecordSet))
}

// DNSZone mocks base method.
func (m *MockFactory) DNSZone() (client.DNSZone, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DNSZone")
	ret0, _ := ret[0].(client.DNSZone)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DNSZone indicates an expected call of DNSZone.
func (mr *MockFactoryMockRecorder) DNSZone() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DNSZone", reflect.TypeOf((*MockFactory)(nil).DNSZone))
}

// Disk mocks base method.
func (m *MockFactory) Disk() (client.Disk, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Disk")
	ret0, _ := ret[0].(client.Disk)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Disk indicates an expected call of Disk.
func (mr *MockFactoryMockRecorder) Disk() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Disk", reflect.TypeOf((*MockFactory)(nil).Disk))
}

// Group mocks base method.
func (m *MockFactory) Group() (client.ResourceGroup, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Group")
	ret0, _ := ret[0].(client.ResourceGroup)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Group indicates an expected call of Group.
func (mr *MockFactoryMockRecorder) Group() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Group", reflect.TypeOf((*MockFactory)(nil).Group))
}

// ManagedUserIdentity mocks base method.
func (m *MockFactory) ManagedUserIdentity() (client.ManagedUserIdentity, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ManagedUserIdentity")
	ret0, _ := ret[0].(client.ManagedUserIdentity)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ManagedUserIdentity indicates an expected call of ManagedUserIdentity.
func (mr *MockFactoryMockRecorder) ManagedUserIdentity() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ManagedUserIdentity", reflect.TypeOf((*MockFactory)(nil).ManagedUserIdentity))
}

// NatGateway mocks base method.
func (m *MockFactory) NatGateway() (client.NatGateway, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NatGateway")
	ret0, _ := ret[0].(client.NatGateway)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NatGateway indicates an expected call of NatGateway.
func (mr *MockFactoryMockRecorder) NatGateway() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NatGateway", reflect.TypeOf((*MockFactory)(nil).NatGateway))
}

// NetworkInterface mocks base method.
func (m *MockFactory) NetworkInterface() (client.NetworkInterface, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NetworkInterface")
	ret0, _ := ret[0].(client.NetworkInterface)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NetworkInterface indicates an expected call of NetworkInterface.
func (mr *MockFactoryMockRecorder) NetworkInterface() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NetworkInterface", reflect.TypeOf((*MockFactory)(nil).NetworkInterface))
}

// NetworkSecurityGroup mocks base method.
func (m *MockFactory) NetworkSecurityGroup() (client.NetworkSecurityGroup, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NetworkSecurityGroup")
	ret0, _ := ret[0].(client.NetworkSecurityGroup)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NetworkSecurityGroup indicates an expected call of NetworkSecurityGroup.
func (mr *MockFactoryMockRecorder) NetworkSecurityGroup() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NetworkSecurityGroup", reflect.TypeOf((*MockFactory)(nil).NetworkSecurityGroup))
}

// PublicIP mocks base method.
func (m *MockFactory) PublicIP() (client.PublicIP, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PublicIP")
	ret0, _ := ret[0].(client.PublicIP)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PublicIP indicates an expected call of PublicIP.
func (mr *MockFactoryMockRecorder) PublicIP() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PublicIP", reflect.TypeOf((*MockFactory)(nil).PublicIP))
}

// RouteTables mocks base method.
func (m *MockFactory) RouteTables() (client.RouteTables, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RouteTables")
	ret0, _ := ret[0].(client.RouteTables)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RouteTables indicates an expected call of RouteTables.
func (mr *MockFactoryMockRecorder) RouteTables() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RouteTables", reflect.TypeOf((*MockFactory)(nil).RouteTables))
}

// Storage mocks base method.
func (m *MockFactory) Storage(arg0 context.Context, arg1 v1.SecretReference) (client.Storage, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Storage", arg0, arg1)
	ret0, _ := ret[0].(client.Storage)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Storage indicates an expected call of Storage.
func (mr *MockFactoryMockRecorder) Storage(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Storage", reflect.TypeOf((*MockFactory)(nil).Storage), arg0, arg1)
}

// StorageAccount mocks base method.
func (m *MockFactory) StorageAccount() (client.StorageAccount, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StorageAccount")
	ret0, _ := ret[0].(client.StorageAccount)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StorageAccount indicates an expected call of StorageAccount.
func (mr *MockFactoryMockRecorder) StorageAccount() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StorageAccount", reflect.TypeOf((*MockFactory)(nil).StorageAccount))
}

// Subnet mocks base method.
func (m *MockFactory) Subnet() (client.Subnet, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Subnet")
	ret0, _ := ret[0].(client.Subnet)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Subnet indicates an expected call of Subnet.
func (mr *MockFactoryMockRecorder) Subnet() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Subnet", reflect.TypeOf((*MockFactory)(nil).Subnet))
}

// VirtualMachine mocks base method.
func (m *MockFactory) VirtualMachine() (client.VirtualMachine, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "VirtualMachine")
	ret0, _ := ret[0].(client.VirtualMachine)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// VirtualMachine indicates an expected call of VirtualMachine.
func (mr *MockFactoryMockRecorder) VirtualMachine() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "VirtualMachine", reflect.TypeOf((*MockFactory)(nil).VirtualMachine))
}

// Vmss mocks base method.
func (m *MockFactory) Vmss() (client.Vmss, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Vmss")
	ret0, _ := ret[0].(client.Vmss)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Vmss indicates an expected call of Vmss.
func (mr *MockFactoryMockRecorder) Vmss() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Vmss", reflect.TypeOf((*MockFactory)(nil).Vmss))
}

// Vnet mocks base method.
func (m *MockFactory) Vnet() (client.Vnet, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Vnet")
	ret0, _ := ret[0].(client.Vnet)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Vnet indicates an expected call of Vnet.
func (mr *MockFactoryMockRecorder) Vnet() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Vnet", reflect.TypeOf((*MockFactory)(nil).Vnet))
}

// MockResourceGroup is a mock of ResourceGroup interface.
type MockResourceGroup struct {
	ctrl     *gomock.Controller
	recorder *MockResourceGroupMockRecorder
}

// MockResourceGroupMockRecorder is the mock recorder for MockResourceGroup.
type MockResourceGroupMockRecorder struct {
	mock *MockResourceGroup
}

// NewMockResourceGroup creates a new mock instance.
func NewMockResourceGroup(ctrl *gomock.Controller) *MockResourceGroup {
	mock := &MockResourceGroup{ctrl: ctrl}
	mock.recorder = &MockResourceGroupMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockResourceGroup) EXPECT() *MockResourceGroupMockRecorder {
	return m.recorder
}

// CreateOrUpdate mocks base method.
func (m *MockResourceGroup) CreateOrUpdate(arg0 context.Context, arg1, arg2 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateOrUpdate", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateOrUpdate indicates an expected call of CreateOrUpdate.
func (mr *MockResourceGroupMockRecorder) CreateOrUpdate(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateOrUpdate", reflect.TypeOf((*MockResourceGroup)(nil).CreateOrUpdate), arg0, arg1, arg2)
}

// DeleteIfExists mocks base method.
func (m *MockResourceGroup) DeleteIfExists(arg0 context.Context, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteIfExists", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteIfExists indicates an expected call of DeleteIfExists.
func (mr *MockResourceGroupMockRecorder) DeleteIfExists(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteIfExists", reflect.TypeOf((*MockResourceGroup)(nil).DeleteIfExists), arg0, arg1)
}

// Get mocks base method.
func (m *MockResourceGroup) Get(arg0 context.Context, arg1 string) (*armresources.ResourceGroup, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1)
	ret0, _ := ret[0].(*armresources.ResourceGroup)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockResourceGroupMockRecorder) Get(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockResourceGroup)(nil).Get), arg0, arg1)
}

// IsExisting mocks base method.
func (m *MockResourceGroup) IsExisting(arg0 context.Context, arg1 string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsExisting", arg0, arg1)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsExisting indicates an expected call of IsExisting.
func (mr *MockResourceGroupMockRecorder) IsExisting(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsExisting", reflect.TypeOf((*MockResourceGroup)(nil).IsExisting), arg0, arg1)
}

// MockVnet is a mock of Vnet interface.
type MockVnet struct {
	ctrl     *gomock.Controller
	recorder *MockVnetMockRecorder
}

// MockVnetMockRecorder is the mock recorder for MockVnet.
type MockVnetMockRecorder struct {
	mock *MockVnet
}

// NewMockVnet creates a new mock instance.
func NewMockVnet(ctrl *gomock.Controller) *MockVnet {
	mock := &MockVnet{ctrl: ctrl}
	mock.recorder = &MockVnetMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockVnet) EXPECT() *MockVnetMockRecorder {
	return m.recorder
}

// CreateOrUpdate mocks base method.
func (m *MockVnet) CreateOrUpdate(arg0 context.Context, arg1, arg2 string, arg3 armnetwork.VirtualNetwork) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateOrUpdate", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateOrUpdate indicates an expected call of CreateOrUpdate.
func (mr *MockVnetMockRecorder) CreateOrUpdate(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateOrUpdate", reflect.TypeOf((*MockVnet)(nil).CreateOrUpdate), arg0, arg1, arg2, arg3)
}

// Delete mocks base method.
func (m *MockVnet) Delete(arg0 context.Context, arg1, arg2 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockVnetMockRecorder) Delete(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockVnet)(nil).Delete), arg0, arg1, arg2)
}

// Get mocks base method.
func (m *MockVnet) Get(arg0 context.Context, arg1, arg2 string) (armnetwork.VirtualNetworksClientGetResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1, arg2)
	ret0, _ := ret[0].(armnetwork.VirtualNetworksClientGetResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockVnetMockRecorder) Get(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockVnet)(nil).Get), arg0, arg1, arg2)
}

// MockRouteTables is a mock of RouteTables interface.
type MockRouteTables struct {
	ctrl     *gomock.Controller
	recorder *MockRouteTablesMockRecorder
}

// MockRouteTablesMockRecorder is the mock recorder for MockRouteTables.
type MockRouteTablesMockRecorder struct {
	mock *MockRouteTables
}

// NewMockRouteTables creates a new mock instance.
func NewMockRouteTables(ctrl *gomock.Controller) *MockRouteTables {
	mock := &MockRouteTables{ctrl: ctrl}
	mock.recorder = &MockRouteTablesMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRouteTables) EXPECT() *MockRouteTablesMockRecorder {
	return m.recorder
}

// CreateOrUpdate mocks base method.
func (m *MockRouteTables) CreateOrUpdate(arg0 context.Context, arg1, arg2 string, arg3 armnetwork.RouteTable) (*armnetwork.RouteTable, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateOrUpdate", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(*armnetwork.RouteTable)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateOrUpdate indicates an expected call of CreateOrUpdate.
func (mr *MockRouteTablesMockRecorder) CreateOrUpdate(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateOrUpdate", reflect.TypeOf((*MockRouteTables)(nil).CreateOrUpdate), arg0, arg1, arg2, arg3)
}

// Delete mocks base method.
func (m *MockRouteTables) Delete(arg0 context.Context, arg1, arg2 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockRouteTablesMockRecorder) Delete(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockRouteTables)(nil).Delete), arg0, arg1, arg2)
}

// Get mocks base method.
func (m *MockRouteTables) Get(arg0 context.Context, arg1, arg2 string) (*armnetwork.RouteTable, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1, arg2)
	ret0, _ := ret[0].(*armnetwork.RouteTable)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockRouteTablesMockRecorder) Get(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockRouteTables)(nil).Get), arg0, arg1, arg2)
}

// MockNatGateway is a mock of NatGateway interface.
type MockNatGateway struct {
	ctrl     *gomock.Controller
	recorder *MockNatGatewayMockRecorder
}

// MockNatGatewayMockRecorder is the mock recorder for MockNatGateway.
type MockNatGatewayMockRecorder struct {
	mock *MockNatGateway
}

// NewMockNatGateway creates a new mock instance.
func NewMockNatGateway(ctrl *gomock.Controller) *MockNatGateway {
	mock := &MockNatGateway{ctrl: ctrl}
	mock.recorder = &MockNatGatewayMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockNatGateway) EXPECT() *MockNatGatewayMockRecorder {
	return m.recorder
}

// CreateOrUpdate mocks base method.
func (m *MockNatGateway) CreateOrUpdate(arg0 context.Context, arg1, arg2 string, arg3 armnetwork.NatGateway) (*armnetwork.NatGateway, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateOrUpdate", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(*armnetwork.NatGateway)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateOrUpdate indicates an expected call of CreateOrUpdate.
func (mr *MockNatGatewayMockRecorder) CreateOrUpdate(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateOrUpdate", reflect.TypeOf((*MockNatGateway)(nil).CreateOrUpdate), arg0, arg1, arg2, arg3)
}

// Delete mocks base method.
func (m *MockNatGateway) Delete(arg0 context.Context, arg1, arg2 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockNatGatewayMockRecorder) Delete(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockNatGateway)(nil).Delete), arg0, arg1, arg2)
}

// Get mocks base method.
func (m *MockNatGateway) Get(arg0 context.Context, arg1, arg2 string) (*armnetwork.NatGateway, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1, arg2)
	ret0, _ := ret[0].(*armnetwork.NatGateway)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockNatGatewayMockRecorder) Get(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockNatGateway)(nil).Get), arg0, arg1, arg2)
}

// List mocks base method.
func (m *MockNatGateway) List(arg0 context.Context, arg1 string) ([]*armnetwork.NatGateway, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List", arg0, arg1)
	ret0, _ := ret[0].([]*armnetwork.NatGateway)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockNatGatewayMockRecorder) List(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockNatGateway)(nil).List), arg0, arg1)
}

// MockPublicIP is a mock of PublicIP interface.
type MockPublicIP struct {
	ctrl     *gomock.Controller
	recorder *MockPublicIPMockRecorder
}

// MockPublicIPMockRecorder is the mock recorder for MockPublicIP.
type MockPublicIPMockRecorder struct {
	mock *MockPublicIP
}

// NewMockPublicIP creates a new mock instance.
func NewMockPublicIP(ctrl *gomock.Controller) *MockPublicIP {
	mock := &MockPublicIP{ctrl: ctrl}
	mock.recorder = &MockPublicIPMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPublicIP) EXPECT() *MockPublicIPMockRecorder {
	return m.recorder
}

// CreateOrUpdate mocks base method.
func (m *MockPublicIP) CreateOrUpdate(arg0 context.Context, arg1, arg2 string, arg3 armnetwork.PublicIPAddress) (*armnetwork.PublicIPAddress, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateOrUpdate", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(*armnetwork.PublicIPAddress)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateOrUpdate indicates an expected call of CreateOrUpdate.
func (mr *MockPublicIPMockRecorder) CreateOrUpdate(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateOrUpdate", reflect.TypeOf((*MockPublicIP)(nil).CreateOrUpdate), arg0, arg1, arg2, arg3)
}

// Delete mocks base method.
func (m *MockPublicIP) Delete(arg0 context.Context, arg1, arg2 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockPublicIPMockRecorder) Delete(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockPublicIP)(nil).Delete), arg0, arg1, arg2)
}

// Get mocks base method.
func (m *MockPublicIP) Get(arg0 context.Context, arg1, arg2 string) (*armnetwork.PublicIPAddress, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1, arg2)
	ret0, _ := ret[0].(*armnetwork.PublicIPAddress)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockPublicIPMockRecorder) Get(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockPublicIP)(nil).Get), arg0, arg1, arg2)
}

// List mocks base method.
func (m *MockPublicIP) List(arg0 context.Context, arg1 string) ([]*armnetwork.PublicIPAddress, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List", arg0, arg1)
	ret0, _ := ret[0].([]*armnetwork.PublicIPAddress)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockPublicIPMockRecorder) List(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockPublicIP)(nil).List), arg0, arg1)
}

// MockAvailabilitySet is a mock of AvailabilitySet interface.
type MockAvailabilitySet struct {
	ctrl     *gomock.Controller
	recorder *MockAvailabilitySetMockRecorder
}

// MockAvailabilitySetMockRecorder is the mock recorder for MockAvailabilitySet.
type MockAvailabilitySetMockRecorder struct {
	mock *MockAvailabilitySet
}

// NewMockAvailabilitySet creates a new mock instance.
func NewMockAvailabilitySet(ctrl *gomock.Controller) *MockAvailabilitySet {
	mock := &MockAvailabilitySet{ctrl: ctrl}
	mock.recorder = &MockAvailabilitySetMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAvailabilitySet) EXPECT() *MockAvailabilitySetMockRecorder {
	return m.recorder
}

// CreateOrUpdate mocks base method.
func (m *MockAvailabilitySet) CreateOrUpdate(arg0 context.Context, arg1, arg2 string, arg3 armcompute.AvailabilitySet) (*armcompute.AvailabilitySet, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateOrUpdate", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(*armcompute.AvailabilitySet)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateOrUpdate indicates an expected call of CreateOrUpdate.
func (mr *MockAvailabilitySetMockRecorder) CreateOrUpdate(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateOrUpdate", reflect.TypeOf((*MockAvailabilitySet)(nil).CreateOrUpdate), arg0, arg1, arg2, arg3)
}

// Delete mocks base method.
func (m *MockAvailabilitySet) Delete(arg0 context.Context, arg1, arg2 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockAvailabilitySetMockRecorder) Delete(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockAvailabilitySet)(nil).Delete), arg0, arg1, arg2)
}

// Get mocks base method.
func (m *MockAvailabilitySet) Get(arg0 context.Context, arg1, arg2 string) (*armcompute.AvailabilitySet, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1, arg2)
	ret0, _ := ret[0].(*armcompute.AvailabilitySet)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockAvailabilitySetMockRecorder) Get(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockAvailabilitySet)(nil).Get), arg0, arg1, arg2)
}

// MockNetworkSecurityGroup is a mock of NetworkSecurityGroup interface.
type MockNetworkSecurityGroup struct {
	ctrl     *gomock.Controller
	recorder *MockNetworkSecurityGroupMockRecorder
}

// MockNetworkSecurityGroupMockRecorder is the mock recorder for MockNetworkSecurityGroup.
type MockNetworkSecurityGroupMockRecorder struct {
	mock *MockNetworkSecurityGroup
}

// NewMockNetworkSecurityGroup creates a new mock instance.
func NewMockNetworkSecurityGroup(ctrl *gomock.Controller) *MockNetworkSecurityGroup {
	mock := &MockNetworkSecurityGroup{ctrl: ctrl}
	mock.recorder = &MockNetworkSecurityGroupMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockNetworkSecurityGroup) EXPECT() *MockNetworkSecurityGroupMockRecorder {
	return m.recorder
}

// CreateOrUpdate mocks base method.
func (m *MockNetworkSecurityGroup) CreateOrUpdate(arg0 context.Context, arg1, arg2 string, arg3 armnetwork.SecurityGroup) (*armnetwork.SecurityGroup, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateOrUpdate", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(*armnetwork.SecurityGroup)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateOrUpdate indicates an expected call of CreateOrUpdate.
func (mr *MockNetworkSecurityGroupMockRecorder) CreateOrUpdate(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateOrUpdate", reflect.TypeOf((*MockNetworkSecurityGroup)(nil).CreateOrUpdate), arg0, arg1, arg2, arg3)
}

// Delete mocks base method.
func (m *MockNetworkSecurityGroup) Delete(arg0 context.Context, arg1, arg2 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockNetworkSecurityGroupMockRecorder) Delete(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockNetworkSecurityGroup)(nil).Delete), arg0, arg1, arg2)
}

// Get mocks base method.
func (m *MockNetworkSecurityGroup) Get(arg0 context.Context, arg1, arg2 string) (*armnetwork.SecurityGroup, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1, arg2)
	ret0, _ := ret[0].(*armnetwork.SecurityGroup)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockNetworkSecurityGroupMockRecorder) Get(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockNetworkSecurityGroup)(nil).Get), arg0, arg1, arg2)
}

// MockManagedUserIdentity is a mock of ManagedUserIdentity interface.
type MockManagedUserIdentity struct {
	ctrl     *gomock.Controller
	recorder *MockManagedUserIdentityMockRecorder
}

// MockManagedUserIdentityMockRecorder is the mock recorder for MockManagedUserIdentity.
type MockManagedUserIdentityMockRecorder struct {
	mock *MockManagedUserIdentity
}

// NewMockManagedUserIdentity creates a new mock instance.
func NewMockManagedUserIdentity(ctrl *gomock.Controller) *MockManagedUserIdentity {
	mock := &MockManagedUserIdentity{ctrl: ctrl}
	mock.recorder = &MockManagedUserIdentityMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockManagedUserIdentity) EXPECT() *MockManagedUserIdentityMockRecorder {
	return m.recorder
}

// Get mocks base method.
func (m *MockManagedUserIdentity) Get(arg0 context.Context, arg1, arg2 string) (msi.Identity, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1, arg2)
	ret0, _ := ret[0].(msi.Identity)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockManagedUserIdentityMockRecorder) Get(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockManagedUserIdentity)(nil).Get), arg0, arg1, arg2)
}
