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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	azurecompute "github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-03-01/compute"
	azuredns "github.com/Azure/azure-sdk-for-go/services/dns/mgmt/2018-05-01/dns"
	azurenetwork "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-05-01/network"
	azurestorage "github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2019-04-01/storage"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewAzureClientFactoryWithSecretReference(ctx context.Context, client client.Client, secretRef corev1.SecretReference) (Factory, error) {
	auth, err := internal.GetClientAuthData(ctx, client, secretRef, false)
	if err != nil {
		return nil, err
	}
	cred, err := auth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}

	return AzureFactory{
		cred: cred,
		auth: auth,
	}, nil
}

func NewAzureClientFactoryWithAuthAndClient(auth *internal.ClientAuth, client client.Client) (Factory, error) {
	cred, err := auth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}

	return AzureFactory{
		client: client, // TODO remove? only used for storage
		cred:   cred,
		auth:   auth,
	}, nil
}

// ResourceGroup gets a newer client for an Azure resource group.
func (f AzureFactory) Group() (ResourceGroup, error) {
	if f.auth == nil || f.cred == nil {
		panic("no secret ref for factory")
	}
	cred, err := f.auth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}
	client, err := armresources.NewResourceGroupsClient(f.auth.SubscriptionID, cred, nil)
	return ResourceGroupClient{client}, err
}

// Storage reads the secret from the passed reference and return an Azure (blob) storage client.
func (f AzureFactory) Storage(ctx context.Context, secretRef corev1.SecretReference) (Storage, error) {
	serviceURL, err := newStorageClient(ctx, f.client, &secretRef)
	if err != nil {
		return nil, err
	}

	return StorageClient{
		serviceURL: serviceURL,
	}, nil
}

// StorageAccount reads the secret from the passed reference and return an Azure storage account client.
func (f AzureFactory) StorageAccount() (StorageAccount, error) {
	authorizer, subscriptionID, err := internal.GetAuthorizerAndSubscriptionID(f.auth)
	if err != nil {
		return nil, err
	}
	storageAccountClient := azurestorage.NewAccountsClient(subscriptionID)
	storageAccountClient.Authorizer = authorizer

	return StorageAccountClient{
		client: storageAccountClient,
	}, nil
}

// Vmss reads the secret from the passed reference and return an Azure virtual machine scale set client.
func (f AzureFactory) Vmss() (Vmss, error) {
	authorizer, subscriptionID, err := internal.GetAuthorizerAndSubscriptionID(f.auth)
	if err != nil {
		return nil, err
	}
	vmssClient := azurecompute.NewVirtualMachineScaleSetsClient(subscriptionID)
	vmssClient.Authorizer = authorizer

	return VmssClient{
		client: vmssClient,
	}, nil
}

// VirtualMachine reads the secret from the passed reference and return an Azure virtual machine client.
func (f AzureFactory) VirtualMachine() (VirtualMachine, error) {
	authorizer, subscriptionID, err := internal.GetAuthorizerAndSubscriptionID(f.auth)
	if err != nil {
		return nil, err
	}
	virtualMachinesClient := azurecompute.NewVirtualMachinesClient(subscriptionID)
	virtualMachinesClient.Authorizer = authorizer

	return VirtualMachinesClient{
		client: virtualMachinesClient,
	}, nil
}

// DNSZone reads the secret from the passed reference and return an Azure DNS zone client.
func (f AzureFactory) DNSZone() (DNSZone, error) {
	authorizer, subscriptionID, err := internal.GetAuthorizerAndSubscriptionID(f.auth)
	if err != nil {
		return nil, err
	}
	zonesClient := azuredns.NewZonesClient(subscriptionID)
	zonesClient.Authorizer = authorizer

	return DNSZoneClient{
		client: zonesClient,
	}, nil
}

// DNSRecordSet reads the secret from the passed reference and return an Azure DNS record set client.
func (f AzureFactory) DNSRecordSet() (DNSRecordSet, error) {
	authorizer, subscriptionID, err := internal.GetAuthorizerAndSubscriptionID(f.auth)
	if err != nil {
		return nil, err
	}
	recordSetsClient := azuredns.NewRecordSetsClient(subscriptionID)
	recordSetsClient.Authorizer = authorizer

	return DNSRecordSetClient{
		client: recordSetsClient,
	}, nil
}

// NetworkSecurityGroup reads the secret from the passed reference and return an Azure network security group client.
func (f AzureFactory) NetworkSecurityGroup() (NetworkSecurityGroup, error) {
	authorizer, id, err := internal.GetAuthorizerAndSubscriptionID(f.auth)
	if err != nil {
		return nil, err
	}
	networkSecurityGroupClient := azurenetwork.NewSecurityGroupsClient(id)
	networkSecurityGroupClient.Authorizer = authorizer

	return NetworkSecurityGroupClient{
		client: networkSecurityGroupClient,
	}, nil
}

// PublicIP reads the secret from the passed reference and return an Azure network PublicIPClient.
func (f AzureFactory) PublicIP() (PublicIP, error) {
	authorizer, id, err := internal.GetAuthorizerAndSubscriptionID(f.auth)
	if err != nil {
		return nil, err
	}
	publicIPClient := azurenetwork.NewPublicIPAddressesClient(id)
	publicIPClient.Authorizer = authorizer

	return PublicIPClient{
		client: publicIPClient,
	}, nil
}

// NetworkInterface reads the secret from the passed reference and return an Azure network interface client.
func (f AzureFactory) NetworkInterface() (NetworkInterface, error) {
	authorizer, subscriptionID, err := internal.GetAuthorizerAndSubscriptionID(f.auth)
	if err != nil {
		return nil, err
	}
	networkInterfaceClient := azurenetwork.NewInterfacesClient(subscriptionID)
	networkInterfaceClient.Authorizer = authorizer

	return NetworkInterfaceClient{
		client: networkInterfaceClient,
	}, nil
}

// Disk reads the secret from the passed reference and return an Azure disk client.
func (f AzureFactory) Disk() (Disk, error) {
	authorizer, subscriptionID, err := internal.GetAuthorizerAndSubscriptionID(f.auth)
	if err != nil {
		return nil, err
	}
	disksClient := azurecompute.NewDisksClient(subscriptionID)
	disksClient.Authorizer = authorizer

	return DisksClient{
		client: disksClient,
	}, nil
}

// Vnet reads the secret from the passed reference and return an Azure Vnet client.
func (f AzureFactory) Vnet() (Vnet, error) {
	return NewVnetClient(*f.auth)
}

// Subnet reads the secret from the passed reference and return an Azure Subnet client.
func (f AzureFactory) Subnet() (Subnet, error) {
	if f.auth == nil || f.cred == nil {
		panic("factory not called with secretRef")
	}
	subnetsClient, err := armnetwork.NewSubnetsClient(f.auth.SubscriptionID, f.cred, nil)
	if err != nil {
		return nil, err
	}
	return SubnetsClient{
		client: subnetsClient,
	}, nil
}

// RouteTables reads the secret from the passed reference and return an Azure RouteTables client.
func (f AzureFactory) RouteTables() (RouteTables, error) {
	return NewRouteTablesClient(*f.auth)
}

func (f AzureFactory) NatGateway() (NatGateway, error) {
	return NewNatGatewaysClient(*f.auth)
}
func (f AzureFactory) AvailabilitySet() (AvailabilitySet, error) {
	return NewAvailabilitySetClient(*f.auth)
}
func (f AzureFactory) ManagedUserIdentity() (ManagedUserIdentity, error) {
	return NewManagedUserIdentityClient(*f.auth)
}
