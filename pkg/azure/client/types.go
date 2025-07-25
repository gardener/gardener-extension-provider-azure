// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
)

// Factory represents a factory to produce clients for various Azure services.
type Factory interface {
	StorageAccount() (StorageAccount, error)
	Vmss() (Vmss, error)
	DNSZone() (DNSZone, error)
	DNSRecordSet() (DNSRecordSet, error)
	VirtualMachine() (VirtualMachine, error)
	NetworkInterface() (NetworkInterface, error)
	Disk() (Disk, error)
	Group() (ResourceGroup, error)
	Resource() (Resource, error)
	NetworkSecurityGroup() (NetworkSecurityGroup, error)
	Subnet() (Subnet, error)
	LoadBalancer() (LoadBalancer, error)
	PublicIP() (PublicIP, error)
	Vnet() (VirtualNetwork, error)
	RouteTables() (RouteTables, error)
	NatGateway() (NatGateway, error)
	AvailabilitySet() (AvailabilitySet, error)
	ManagedUserIdentity() (ManagedUserIdentity, error)
	VirtualMachineImages() (VirtualMachineImages, error)
	BlobContainers() (BlobContainers, error)
	ManagementPolicies() (ManagementPolicies, error)
}

// ResourceGroup represents an Azure ResourceGroup k8sClient.
type ResourceGroup interface {
	ContainerCreateOrUpdateFunc[armresources.ResourceGroup]
	ContainerDeleteFunc[armresources.ResourceGroup]
	ContainerGetFunc[armresources.ResourceGroup]
	ContainerCheckExistenceFunc[armresources.ResourceGroup]
}

// AvailabilitySet is an interface for the Azure AvailabilitySet service.
type AvailabilitySet interface {
	GetFunc[armcompute.AvailabilitySet]
	CreateOrUpdateFunc[armcompute.AvailabilitySet]
	DeleteFunc[armcompute.AvailabilitySet]
}

// NatGateway is an interface for the Azure NatGateway service.
type NatGateway interface {
	CreateOrUpdateFunc[armnetwork.NatGateway]
	GetWithExpandFunc[armnetwork.NatGateway, *string]
	ListFunc[armnetwork.NatGateway]
	DeleteFunc[armnetwork.NatGateway]
}

// RouteTables is a k8sClient for the Azure RouteTable service.
type RouteTables interface {
	CreateOrUpdateFunc[armnetwork.RouteTable]
	DeleteFunc[armnetwork.RouteTable]
	GetFunc[armnetwork.RouteTable]
}

// ManagedUserIdentity is a k8sClient for the Azure Managed User Identity service.
type ManagedUserIdentity interface {
	GetFunc[armmsi.UserAssignedIdentitiesClientGetResponse]
}

// Vmss represents an Azure virtual machine scale set k8sClient.
type Vmss interface {
	ListFunc[armcompute.VirtualMachineScaleSet]
	GetWithExpandFunc[armcompute.VirtualMachineScaleSet, *armcompute.ExpandTypesForGetVMScaleSets]
	CreateOrUpdateFunc[armcompute.VirtualMachineScaleSet]
	DeleteWithOptsFunc[armcompute.VirtualMachineScaleSet, *bool]
}

// VirtualMachine represents an Azure virtual machine k8sClient.
type VirtualMachine interface {
	GetWithExpandFunc[armcompute.VirtualMachine, *armcompute.InstanceViewTypes]
	CreateOrUpdateFunc[armcompute.VirtualMachine]
	DeleteWithOptsFunc[armcompute.VirtualMachine, *bool]
}

// NetworkSecurityGroup represents an Azure Network security group k8sClient.
type NetworkSecurityGroup interface {
	GetFunc[armnetwork.SecurityGroup]
	CreateOrUpdateFunc[armnetwork.SecurityGroup]
	DeleteFunc[armnetwork.SecurityGroup]
}

// PublicIP represents an Azure Network Public IP k8sClient.
type PublicIP interface {
	GetWithExpandFunc[armnetwork.PublicIPAddress, *string]
	CreateOrUpdateFunc[armnetwork.PublicIPAddress]
	DeleteFunc[armnetwork.PublicIPAddress]
	ListFunc[armnetwork.PublicIPAddress]
}

// NetworkInterface represents an Azure Network Interface k8sClient.
type NetworkInterface interface {
	GetFunc[armnetwork.Interface]
	CreateOrUpdateFunc[armnetwork.Interface]
	DeleteFunc[armnetwork.Interface]
}

// Disk represents an Azure Disk k8sClient.
type Disk interface {
	GetFunc[armcompute.Disk]
	CreateOrUpdateFunc[armcompute.Disk]
	DeleteFunc[armcompute.Disk]
}

// Subnet represents an Azure Subnet k8sClient.
type Subnet interface {
	SubResourceCreateOrUpdateFunc[armnetwork.Subnet]
	SubResourceGetWithExpandFunc[armnetwork.Subnet, *string]
	SubResourceListFunc[armnetwork.Subnet]
	SubResourceDeleteFunc[armnetwork.Subnet]
}

// LoadBalancer represents an Azure LoadBalancer k8sClient.
type LoadBalancer interface {
	GetFunc[armnetwork.LoadBalancer]
	ListFunc[armnetwork.LoadBalancer]
	DeleteFunc[armnetwork.LoadBalancer]
}

// VirtualNetwork represents an Azure Virtual Network k8sClient.
type VirtualNetwork interface {
	CreateOrUpdateFunc[armnetwork.VirtualNetwork]
	GetFunc[armnetwork.VirtualNetwork]
	DeleteFunc[armnetwork.VirtualNetwork]
}

// StorageAccount represents an Azure storage account k8sClient.
type StorageAccount interface {
	CreateOrUpdateStorageAccount(context.Context, string, string, string, *int32) error
	ListStorageAccountKeys(context.Context, string, string) ([]*armstorage.AccountKey, error)
	RotateKey(context.Context, string, string, string) ([]*armstorage.AccountKey, error)
}

// DNSZone represents an Azure DNS zone k8sClient.
type DNSZone interface {
	List(context.Context) (map[string]string, error)
}

// DNSRecordSet represents an Azure DNS recordset k8sClient.
type DNSRecordSet interface {
	CreateOrUpdate(context.Context, string, string, string, []string, int64) error
	Delete(context.Context, string, string, string) error
}

// VirtualMachineImages represents an Azure Virtual Machine Image k8sClient.
type VirtualMachineImages interface {
	ListSkus(ctx context.Context, location string, publisherName string, offer string) (*armcompute.VirtualMachineImagesClientListSKUsResponse, error)
}

// BlobStorage represents an Azure blob storage k8sClient.
type BlobStorage interface {
	CleanupObjectsWithPrefix(context.Context, string) error
}

// Resource is an Azure resources client.
type Resource interface {
	ListByResourceGroup(ctx context.Context, resourceGroupName string, options *armresources.ClientListByResourceGroupOptions) ([]*armresources.GenericResourceExpanded, error)
}

// BlobContainers is an Azure Blob Container client.
type BlobContainers interface {
	GetContainer(context.Context, string, string, string) (armstorage.BlobContainersClientGetResponse, error)
	CreateContainer(context.Context, string, string, string) (armstorage.BlobContainersClientCreateResponse, error)
	GetImmutabilityPolicy(context.Context, string, string, string) (*int32, bool, *string, error)
	CreateOrUpdateImmutabilityPolicy(context.Context, string, string, string, *int32) (*string, error)
	DeleteImmutabilityPolicy(context.Context, string, string, string, *string) error
	ExtendImmutabilityPolicy(context.Context, string, string, string, *int32, *string) error
	LockImmutabilityPolicy(context.Context, string, string, string, *string) error
	DeleteContainer(context.Context, string, string, string) error
}

// ManagementPolicies is an Azure Blob Storage storage account lifecycle policy client
type ManagementPolicies interface {
	CreateOrUpdate(context.Context, string, string, int) error
}
