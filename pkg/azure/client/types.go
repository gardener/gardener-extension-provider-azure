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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-03-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/msi/mgmt/2018-11-30/msi"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

// Factory represents a factory to produce clients for various Azure services.
type Factory interface {
	Auth() *internal.ClientAuth

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
	Vnet() (VirtualNetwork, error)
	RouteTables() (RouteTables, error)
	NatGateway() (NatGateway, error)
	AvailabilitySet() (AvailabilitySet, error)
	ManagedUserIdentity() (ManagedUserIdentity, error)
	VirtualMachineImages() (VirtualMachineImages, error)
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
	GetFunc[msi.Identity]
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

// VirtualNetwork represents an Azure Virtual Network k8sClient.
type VirtualNetwork interface {
	CreateOrUpdateFunc[armnetwork.VirtualNetwork]
	GetFunc[armnetwork.VirtualNetwork]
	DeleteFunc[armnetwork.VirtualNetwork]
}

// StorageAccount represents an Azure storage account k8sClient.
type StorageAccount interface {
	CreateStorageAccount(context.Context, string, string, string) error
	ListStorageAccountKey(context.Context, string, string) (string, error)
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
	ListSkus(ctx context.Context, location string, publisherName string, offer string) (*compute.ListVirtualMachineImageResource, error)
}

// Storage represents an Azure (blob) storage k8sClient.
type Storage interface {
	DeleteObjectsWithPrefix(context.Context, string, string) error
	CreateContainerIfNotExists(context.Context, string) error
	DeleteContainerIfExists(context.Context, string) error
}
