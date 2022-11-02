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
	azurecompute "github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-03-01/compute"
	azuredns "github.com/Azure/azure-sdk-for-go/services/dns/mgmt/2018-05-01/dns"
	azurenetwork "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-05-01/network"
	azureresources "github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	azurestorage "github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2019-04-01/storage"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewAzureClientFactory returns a new factory to produce clients for various Azure services.
func NewAzureClientFactory(client client.Client) Factory {
	return AzureFactory{
		client: client,
	}
}

// Group reads the secret from the passed reference and return an Azure resource group client.
func (f AzureFactory) Group(ctx context.Context, secretRef corev1.SecretReference) (Group, error) {
	authorizer, subscriptionID, err := internal.GetAuthorizerAndSubscriptionID(ctx, f.client, secretRef, false)
	if err != nil {
		return nil, err
	}
	groupClient := azureresources.NewGroupsClient(subscriptionID)
	groupClient.Authorizer = authorizer

	return GroupClient{
		client: groupClient,
	}, nil
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
func (f AzureFactory) StorageAccount(ctx context.Context, secretRef corev1.SecretReference) (StorageAccount, error) {
	authorizer, subscriptionID, err := internal.GetAuthorizerAndSubscriptionID(ctx, f.client, secretRef, false)
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
func (f AzureFactory) Vmss(ctx context.Context, secretRef corev1.SecretReference) (Vmss, error) {
	authorizer, subscriptionID, err := internal.GetAuthorizerAndSubscriptionID(ctx, f.client, secretRef, false)
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
func (f AzureFactory) VirtualMachine(ctx context.Context, secretRef corev1.SecretReference) (VirtualMachine, error) {
	authorizer, subscriptionID, err := internal.GetAuthorizerAndSubscriptionID(ctx, f.client, secretRef, false)
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
func (f AzureFactory) DNSZone(ctx context.Context, secretRef corev1.SecretReference) (DNSZone, error) {
	authorizer, subscriptionID, err := internal.GetAuthorizerAndSubscriptionID(ctx, f.client, secretRef, true)
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
func (f AzureFactory) DNSRecordSet(ctx context.Context, secretRef corev1.SecretReference) (DNSRecordSet, error) {
	authorizer, subscriptionID, err := internal.GetAuthorizerAndSubscriptionID(ctx, f.client, secretRef, true)
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
func (f AzureFactory) NetworkSecurityGroup(ctx context.Context, secretRef corev1.SecretReference) (NetworkSecurityGroup, error) {
	authorizer, subscriptionID, err := internal.GetAuthorizerAndSubscriptionID(ctx, f.client, secretRef, false)
	if err != nil {
		return nil, err
	}
	networkSecurityGroupClient := azurenetwork.NewSecurityGroupsClient(subscriptionID)
	networkSecurityGroupClient.Authorizer = authorizer

	return NetworkSecurityGroupClient{
		client: networkSecurityGroupClient,
	}, nil
}

// PublicIP reads the secret from the passed reference and return an Azure network PublicIPClient.
func (f AzureFactory) PublicIP(ctx context.Context, secretRef corev1.SecretReference) (PublicIP, error) {
	authorizer, subscriptionID, err := internal.GetAuthorizerAndSubscriptionID(ctx, f.client, secretRef, false)
	if err != nil {
		return nil, err
	}
	publicIPClient := azurenetwork.NewPublicIPAddressesClient(subscriptionID)
	publicIPClient.Authorizer = authorizer

	return PublicIPClient{
		client: publicIPClient,
	}, nil
}

// NetworkInterface reads the secret from the passed reference and return an Azure network interface client.
func (f AzureFactory) NetworkInterface(ctx context.Context, secretRef corev1.SecretReference) (NetworkInterface, error) {
	authorizer, subscriptionID, err := internal.GetAuthorizerAndSubscriptionID(ctx, f.client, secretRef, false)
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
func (f AzureFactory) Disk(ctx context.Context, secretRef corev1.SecretReference) (Disk, error) {
	authorizer, subscriptionID, err := internal.GetAuthorizerAndSubscriptionID(ctx, f.client, secretRef, false)
	if err != nil {
		return nil, err
	}
	disksClient := azurecompute.NewDisksClient(subscriptionID)
	disksClient.Authorizer = authorizer

	return DisksClient{
		client: disksClient,
	}, nil
}

func (f AzureFactory) Vnet(ctx context.Context, secretRef corev1.SecretReference) (*VnetClient, error) {
	auth, err := internal.GetClientAuthData(ctx, f.client, secretRef, false)
	if err != nil {
		return nil, err
	}
	cred, err := auth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}
	vnetClient, err := armnetwork.NewVirtualNetworksClient(auth.SubscriptionID, cred, nil)
	if err != nil {
		return nil, err
	}
	return &VnetClient{
		client: vnetClient,
	}, nil

}

// Subnet reads the secret from the passed reference and return an Azure Subnet client.
func (f AzureFactory) Subnet(ctx context.Context, secretRef corev1.SecretReference) (Subnet, error) {
	auth, err := internal.GetClientAuthData(ctx, f.client, secretRef, false)
	if err != nil {
		return nil, err
	}
	cred, err := auth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}
	subnetsClient, err := armnetwork.NewSubnetsClient(auth.SubscriptionID, cred, nil)
	if err != nil {
		return nil, err
	}
	return SubnetsClient{
		client: subnetsClient,
	}, nil
}
