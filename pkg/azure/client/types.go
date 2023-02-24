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
	"github.com/Azure/azure-sdk-for-go/services/msi/mgmt/2018-11-30/msi"
	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2019-04-01/storage"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Factory represents a factory to produce clients for various Azure services.
type Factory interface {
	Storage(context.Context, corev1.SecretReference) (Storage, error)
	StorageAccount() (StorageAccount, error)
	Vmss() (Vmss, error)
	DNSZone() (DNSZone, error)
	DNSRecordSet() (DNSRecordSet, error)
	VirtualMachine() (VirtualMachine, error)
	NetworkInterface() (NetworkInterface, error)
	Disk() (Disk, error)
	Group() (ResourceGroup, error)
	NetworkSecurityGroup() (NetworkSecurityGroup, error)
	Subnet() (Subnet, error)
	PublicIP() (PublicIP, error)
	Vnet() (Vnet, error)
	RouteTables() (RouteTables, error)
	NatGateway() (NatGateway, error)
	AvailabilitySet() (AvailabilitySet, error)
	ManagedUserIdentity() (ManagedUserIdentity, error)
}

// AvailabilitySet is an interface for the Azure AvailabilitySet service.
type AvailabilitySet interface {
	Get(ctx context.Context, resourceGroupName, availabilitySetName string) (result armcompute.AvailabilitySetsClientGetResponse, err error)
	CreateOrUpdate(ctx context.Context, resourceGroupName string, availabilitySetName string, parameters armcompute.AvailabilitySet) (result armcompute.AvailabilitySetsClientCreateOrUpdateResponse, err error)
	Delete(ctx context.Context, resourceGroupName string, availabilitySetName string) (result armcompute.AvailabilitySetsClientDeleteResponse, err error)
}

// NatGateway is an interface for the Azure NatGateway service.
type NatGateway interface {
	CreateOrUpdate(ctx context.Context, resourceGroupName, natGatewayName string, parameters armnetwork.NatGateway) (armnetwork.NatGatewaysClientCreateOrUpdateResponse, error)
	Get(ctx context.Context, resourceGroupName, natGatewayName string) (*armnetwork.NatGatewaysClientGetResponse, error)
	Delete(ctx context.Context, resourceGroupName, natGatewayName string) error
	GetAll(ctx context.Context, resourceGroupName string) ([]*armnetwork.NatGateway, error)
}

// RouteTables is a client for the Azure RouteTable service.
type RouteTables interface {
	CreateOrUpdate(ctx context.Context, resourceGroupName, routeTableName string, parameters armnetwork.RouteTable) (armnetwork.RouteTablesClientCreateOrUpdateResponse, error)
	Delete(ctx context.Context, resourceGroupName, name string) (err error)
	Get(ctx context.Context, resourceGroupName, name string) (armnetwork.RouteTablesClientGetResponse, error)
}

// ManagedUserIdentity is a client for the Azure Managed User Identity service.
type ManagedUserIdentity interface {
	Get(context.Context, string, string) (msi.Identity, error)
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
	List(context.Context, string) ([]*armcompute.VirtualMachineScaleSet, error)
	Get(context.Context, string, string, *armcompute.ExpandTypesForGetVMScaleSets) (*armcompute.VirtualMachineScaleSet, error)
	CreateOrUpdate(context.Context, string, string, armcompute.VirtualMachineScaleSet) (*armcompute.VirtualMachineScaleSet, error)
	Delete(context.Context, string, string, *bool) error
}

// VirtualMachine represents an Azure virtual machine client.
type VirtualMachine interface {
	Get(ctx context.Context, resourceGroupName string, name string, expander *armcompute.InstanceViewTypes) (*armcompute.VirtualMachine, error)
	CreateOrUpdate(ctx context.Context, resourceGroupName string, name string, parameters armcompute.VirtualMachine) (*armcompute.VirtualMachine, error)
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
	Get(ctx context.Context, resourceGroupName, networkSecurityGroupName string) (*armnetwork.SecurityGroup, error)
	CreateOrUpdate(ctx context.Context, resourceGroupName, name string, parameters armnetwork.SecurityGroup) (*armnetwork.SecurityGroup, error)
	Delete(ctx context.Context, resourceGroupName, name string) error
}

// PublicIP represents an Azure Network Public IP client.
type PublicIP interface {
	Get(ctx context.Context, resourceGroupName string, name string) (*armnetwork.PublicIPAddress, error)
	CreateOrUpdate(ctx context.Context, resourceGroupName, name string, parameters armnetwork.PublicIPAddress) (*armnetwork.PublicIPAddress, error)
	Delete(ctx context.Context, resourceGroupName, name string) error
	GetAll(ctx context.Context, resourceGroupName string) ([]*armnetwork.PublicIPAddress, error)
}

// NetworkInterface represents an Azure Network Interface client.
type NetworkInterface interface {
	Get(ctx context.Context, resourceGroupName string, name string) (*armnetwork.Interface, error)
	CreateOrUpdate(ctx context.Context, resourceGroupName, name string, parameters armnetwork.Interface) (*armnetwork.Interface, error)
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
	Get(ctx context.Context, resourceGroupName string, vnetName string, name string) (*armnetwork.SubnetsClientGetResponse, error)
	List(context.Context, string, string) ([]*armnetwork.Subnet, error)
	Delete(context.Context, string, string, string) error
}

// ResourceGroup represents an Azure ResourceGroup client.
type ResourceGroup interface {
	CreateOrUpdate(ctx context.Context, resourceGroupName, location string) error
	DeleteIfExists(ctx context.Context, resourceGroupName string) error
	IsExisting(ctx context.Context, resourceGroupName string) (bool, error)
	Get(ctx context.Context, resourceGroupName string) (*armresources.ResourceGroup, error)
}

// Vnet represents an Azure Virtual Network client.
type Vnet interface {
	CreateOrUpdate(ctx context.Context, resourceGroupName string, name string, parameters armnetwork.VirtualNetwork) (err error)
	Delete(ctx context.Context, resourceGroupName, name string) error
	Get(ctx context.Context, resourceGroupName, name string) (armnetwork.VirtualNetworksClientGetResponse, error)
}

// azureFactory is an implementation of Factory to produce clients for various Azure services.
type azureFactory struct {
	auth   *internal.ClientAuth
	client client.Client // TODO remove? only used for storage client secrets
}

// StorageClient is an implementation of Storage for a (blob) storage client.
type StorageClient struct {
	serviceURL *azblob.ServiceURL
}

// StorageAccountClient is an implementation of StorageAccount for storage account client.
type StorageAccountClient struct {
	client storage.AccountsClient
}

// VmssClient is an implementation of Vmss for a virtual machine scale set client.
type VmssClient struct {
	client *armcompute.VirtualMachineScaleSetsClient
}

// ResourceGroupClient is a newer client implementation of ResourceGroup.
type ResourceGroupClient struct {
	client *armresources.ResourceGroupsClient
}

// VnetClient is an implmenetation of Vnet for a virtual network client.
type VnetClient struct {
	client *armnetwork.VirtualNetworksClient
}

// VirtualMachinesClient is an implementation of Vm for a virtual machine client.
type VirtualMachinesClient struct {
	client *armcompute.VirtualMachinesClient
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
	client *armnetwork.SecurityGroupsClient
}

// PublicIPClient is an implementation of Network Public IP Address.
type PublicIPClient struct {
	client *armnetwork.PublicIPAddressesClient
}

// NetworkInterfaceClient is an implementation of Network Interface.
type NetworkInterfaceClient struct {
	client *armnetwork.InterfacesClient
}

// DisksClient is an implementation of Disk for a disk client.
type DisksClient struct {
	client compute.DisksClient
}

// SubnetsClient is an implementation of Subnet for a Subnet client.
type SubnetsClient struct {
	client *armnetwork.SubnetsClient
}

// RouteTablesClient is an implementation of RouteTables for a RouteTables client.
type RouteTablesClient struct {
	client *armnetwork.RouteTablesClient
}

// NatGatewayClient is an implementation of NatGateway for a Nat Gateway client.
type NatGatewayClient struct {
	client *armnetwork.NatGatewaysClient
}

// AvailabilitySetClient is an implementation of AvailabilitySet for an availability set client.
type AvailabilitySetClient struct {
	client *armcompute.AvailabilitySetsClient
}

// ManagedUserIdentityClient is an implementation of ManagedUserIdentity for a managed user identity client.
type ManagedUserIdentityClient struct {
	client msi.UserAssignedIdentitiesClient
}
