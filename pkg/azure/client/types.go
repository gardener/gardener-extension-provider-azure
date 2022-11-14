// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-03-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/dns/mgmt/2018-05-01/dns"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-05-01/network"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2019-04-01/storage"
	"github.com/Azure/azure-storage-blob-go/azblob"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Factory represents a factory to produce clients for various Azure services.
type Factory interface {
	Group(context.Context, corev1.SecretReference) (Group, error)
	Storage(context.Context, corev1.SecretReference) (Storage, error)
	StorageAccount(context.Context, corev1.SecretReference) (StorageAccount, error)
	Vmss(context.Context, corev1.SecretReference) (Vmss, error)
	DNSZone(context.Context, corev1.SecretReference) (DNSZone, error)
	DNSRecordSet(context.Context, corev1.SecretReference) (DNSRecordSet, error)
	VirtualMachine(ctx context.Context, secretRef corev1.SecretReference) (VirtualMachine, error)
	NetworkSecurityGroup(ctx context.Context, secretRef corev1.SecretReference) (NetworkSecurityGroup, error)
	PublicIP(ctx context.Context, secretRef corev1.SecretReference) (PublicIP, error)
	NetworkInterface(ctx context.Context, secretRef corev1.SecretReference) (NetworkInterface, error)
	Disk(ctx context.Context, secretRef corev1.SecretReference) (Disk, error)
	Subnet(ctx context.Context, secretRef corev1.SecretReference) (Subnet, error)
	ResourceGroup(ctx context.Context, secretRef corev1.SecretReference) (ResourceGroup, error) // #TODO replaces Group
	RouteTables(ctx context.Context, secretRef corev1.SecretReference) (RouteTables, error)
}

type AvailabilitySet interface {
	Get(ctx context.Context, resourceGroupName, availabilitySetName string) (result armcompute.AvailabilitySetsClientGetResponse, err error)
	CreateOrUpdate(ctx context.Context, resourceGroupName string, availabilitySetName string, parameters armcompute.AvailabilitySet) (result armcompute.AvailabilitySetsClientCreateOrUpdateResponse, err error)
	Delete(ctx context.Context, resourceGroupName string, availabilitySetName string) (result armcompute.AvailabilitySetsClientDeleteResponse, err error)
}
type NatGateway interface {
	CreateOrUpdate(ctx context.Context, resourceGroupName, natGatewayName string, parameters armnetwork.NatGateway) (armnetwork.NatGatewaysClientCreateOrUpdateResponse, error)
	Get(ctx context.Context, resourceGroupName, natGatewayName string) (*armnetwork.NatGatewaysClientGetResponse, error)
	Delete(ctx context.Context, resourceGroupName, natGatewayName string) error
}

// #TODO replaces NetworkSecurityGroup
type SecurityGroups interface {
	Get(ctx context.Context, resourceGroupName, networkSecurityGroupName string) (armnetwork.SecurityGroupsClientGetResponse, error)
	CreateOrUpdate(ctx context.Context, resourceGroupName string, networkSecurityGroupName string, parameters armnetwork.SecurityGroup) (armnetwork.SecurityGroupsClientCreateOrUpdateResponse, error)
	Delete(ctx context.Context, resourceGroupName, networkSecurityGroupName string) error
}

// RouteTables is a client for the Azure RouteTable service.
type RouteTables interface {
	CreateOrUpdate(ctx context.Context, resourceGroupName, routeTableName string, parameters armnetwork.RouteTable) (armnetwork.RouteTablesClientCreateOrUpdateResponse, error)
	Delete(ctx context.Context, resourceGroupName, name string) (err error)
	Get(ctx context.Context, resourceGroupName, name string) (armnetwork.RouteTablesClientGetResponse, error)
}

// Group represents an Azure group client.
type Group interface {
	Get(context.Context, string) (*resources.Group, error)
	CreateOrUpdate(context.Context, string, string) error
	DeleteIfExits(context.Context, string) error
}

// Storage represents an Azure (blob) storage client.
type Storage interface {
	DeleteObjectsWithPrefix(context.Context, string, string) error
	CreateContainerIfNotExists(context.Context, string) error
	DeleteContainerIfExists(context.Context, string) error
}

// StorageAccount represents an Azure storage account client.
type StorageAccount interface {
	CreateStorageAccount(context.Context, string, string, string) error
	ListStorageAccountKey(context.Context, string, string) (string, error)
}

// Vmss represents an Azure virtual machine scale set client.
type Vmss interface {
	List(context.Context, string) ([]compute.VirtualMachineScaleSet, error)
	Get(context.Context, string, string, compute.ExpandTypesForGetVMScaleSets) (*compute.VirtualMachineScaleSet, error)
	Create(context.Context, string, string, *compute.VirtualMachineScaleSet) (*compute.VirtualMachineScaleSet, error)
	Delete(context.Context, string, string, *bool) error
}

// VirtualMachine represents an Azure virtual machine client.
type VirtualMachine interface {
	Get(ctx context.Context, resourceGroupName string, name string, instanceViewTypes compute.InstanceViewTypes) (*compute.VirtualMachine, error)
	Create(ctx context.Context, resourceGroupName string, name string, parameters *compute.VirtualMachine) (*compute.VirtualMachine, error)
	Delete(ctx context.Context, resourceGroupName string, name string, forceDeletion *bool) error
}

// DNSZone represents an Azure DNS zone client.
type DNSZone interface {
	GetAll(context.Context) (map[string]string, error)
}

// DNSRecordSet represents an Azure DNS recordset client.
type DNSRecordSet interface {
	CreateOrUpdate(context.Context, string, string, string, []string, int64) error
	Delete(context.Context, string, string, string) error
}

// NetworkSecurityGroup represents an Azure Network security group client.
type NetworkSecurityGroup interface {
	Get(ctx context.Context, resourceGroupName string, networkSecurityGroupName, name string) (*network.SecurityGroup, error)
	CreateOrUpdate(ctx context.Context, resourceGroupName, name string, parameters network.SecurityGroup) (*network.SecurityGroup, error)
}

// PublicIP represents an Azure Network PUblic IP client.
type PublicIP interface {
	Get(ctx context.Context, resourceGroupName string, name string, expander string) (*network.PublicIPAddress, error)
	CreateOrUpdate(ctx context.Context, resourceGroupName, name string, parameters network.PublicIPAddress) (*network.PublicIPAddress, error)
	Delete(ctx context.Context, resourceGroupName, name string) error
}

type NewPublicIP interface {
	Get(ctx context.Context, resourceGroupName string, name string) (armnetwork.PublicIPAddressesClientGetResponse, error)
	CreateOrUpdate(ctx context.Context, resourceGroupName, name string, parameters armnetwork.PublicIPAddress) (armnetwork.PublicIPAddressesClientCreateOrUpdateResponse, error)
	Delete(ctx context.Context, resourceGroupName, name string) error
}

// NetworkInterface represents an Azure Network Interface client.
type NetworkInterface interface {
	Get(ctx context.Context, resourceGroupName string, name string, expander string) (*network.Interface, error)
	CreateOrUpdate(ctx context.Context, resourceGroupName, name string, parameters network.Interface) (*network.Interface, error)
	Delete(ctx context.Context, resourceGroupName, name string) error
}

// Disk represents an Azure Disk client.
type Disk interface {
	Get(ctx context.Context, resourceGroupName string, name string) (*compute.Disk, error)
	CreateOrUpdate(ctx context.Context, resourceGroupName string, diskName string, disk compute.Disk) (*compute.Disk, error)
	Delete(ctx context.Context, resourceGroupName, name string) error
}

// Subnet represents an Azure Subnet client.
type Subnet interface {
	CreateOrUpdate(ctx context.Context, resourceGroupName, vnetName, subnetName string, parameters armnetwork.Subnet) error
	Get(ctx context.Context, resourceGroupName string, vnetName string, name string, expander string) (*armnetwork.SubnetsClientGetResponse, error)
	List(context.Context, string, string) ([]*armnetwork.Subnet, error)
	Delete(context.Context, string, string, string) error
}

// ResourceGroup represents an Azure ResourceGroup client.
type ResourceGroup interface {
	CreateOrUpdate(ctx context.Context, resourceGroupName, location string) error
	Delete(ctx context.Context, resourceGroupName string) error
	IsExisting(ctx context.Context, resourceGroupName string) (bool, error)
}

// Vnet represents an Azure Virtual Network client.
type Vnet interface {
	CreateOrUpdate(ctx context.Context, resourceGroupName string, name string, parameters armnetwork.VirtualNetwork) (err error)
	Delete(ctx context.Context, resourceGroupName, name string) error
	Get(ctx context.Context, resourceGroupName, name string) (armnetwork.VirtualNetworksClientGetResponse, error)
}

// AzureFactory is an implementation of Factory to produce clients for various Azure services.
type AzureFactory struct {
	client client.Client
}

// StorageClient is an implementation of Storage for a (blob) storage client.
type StorageClient struct {
	serviceURL *azblob.ServiceURL
}

// StorageAccountClient is an implementation of StorageAccount for storage account client.
type StorageAccountClient struct {
	client storage.AccountsClient
}

// GroupClient is an implementation of Group for a resource group client.
type GroupClient struct {
	client resources.GroupsClient
}

// VmssClient is an implementation of Vmss for a virtual machine scale set client.
type VmssClient struct {
	client compute.VirtualMachineScaleSetsClient
}

type ResourceGroupClient struct {
	client *armresources.ResourceGroupsClient
}

// VnetClient is an implmenetation of Vnet for a virtual network client.
type VnetClient struct {
	client *armnetwork.VirtualNetworksClient
}

// VirtualMachinesClient is an implementation of Vm for a virtual machine client.
type VirtualMachinesClient struct {
	client compute.VirtualMachinesClient
}

// DNSZoneClient is an implementation of DNSZone for a DNS zone client.
type DNSZoneClient struct {
	client dns.ZonesClient
}

// DNSRecordSetClient is an implementation of DNSRecordSet for a DNS recordset client.
type DNSRecordSetClient struct {
	client dns.RecordSetsClient
}

// NetworkSecurityGroupClient is an implementation of Network Security Group for a network security group client.
type NetworkSecurityGroupClient struct {
	client network.SecurityGroupsClient
}

// PublicIPClient is an implementation of Network Public IP Address.
type PublicIPClient struct {
	client network.PublicIPAddressesClient
}

// TODO replace old PublicIPClient is an implementation of Network Public IP Address.
type NewPublicIPClient struct {
	client *armnetwork.PublicIPAddressesClient
}

// NetworkInterfaceClient is an implementation of Network Interface.
type NetworkInterfaceClient struct {
	client network.InterfacesClient
}

// SecurityRulesClient is an implementation of Network Security Groups rules.
type SecurityRulesClient struct {
	client network.SecurityRulesClient
}

// DisksClient is an implementation of Disk for a disk client.
type DisksClient struct {
	client compute.DisksClient
}

// SubnetsClient is an implementation of Subnet for a Subnet client.
type SubnetsClient struct {
	client *armnetwork.SubnetsClient
}

type RouteTablesClient struct {
	client *armnetwork.RouteTablesClient
}

type SecurityGroupClient struct {
	client *armnetwork.SecurityGroupsClient
}

type NatGatewayClient struct {
	client *armnetwork.NatGatewaysClient
}

type AvailabilitySetClient struct {
	client *armcompute.AvailabilitySetsClient
}
