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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	azuredns "github.com/Azure/azure-sdk-for-go/services/dns/mgmt/2018-05-01/dns"
	azurestorage "github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2019-04-01/storage"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

// AzureFactory is an implementation of Factory to produce clients for various Azure services.
// TODO(KA): client is still important because of how the storage client is constructed. Ideally we shouldn't need a k8s client.
type azureFactory struct {
	client          client.Client
	auth            *internal.ClientAuth
	tokenCredential azcore.TokenCredential
}

// NewAzureClientFactory creates a new Azure client factory with the passed secret reference.
func NewAzureClientFactory(ctx context.Context, client client.Client, secretRef corev1.SecretReference) (Factory, error) {
	auth, err := internal.GetClientAuthData(ctx, client, secretRef, false)
	if err != nil {
		return nil, err
	}
	return NewAzureClientFactoryWithAuth(auth, client)
}

// NewAzureClientFactoryWithDNSSecret creates a new Azure client factory with the passed secret reference using the DNS secret keys.
func NewAzureClientFactoryWithDNSSecret(ctx context.Context, client client.Client, secretRef corev1.SecretReference) (Factory, error) {
	auth, err := internal.GetClientAuthData(ctx, client, secretRef, true)
	if err != nil {
		return nil, err
	}
	return NewAzureClientFactoryWithAuth(auth, client)
}

// NewAzureClientFactoryWithAuth creates a new Azure client factory with the passed credentials.
func NewAzureClientFactoryWithAuth(auth *internal.ClientAuth, client client.Client) (Factory, error) {
	cred, err := auth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}
	return azureFactory{
		client:          client,
		auth:            auth,
		tokenCredential: cred,
	}, nil
}

func (f azureFactory) Auth() *internal.ClientAuth {
	return f.auth
}

// Storage reads the secret from the passed reference and return an Azure (blob) storage client.
func (f azureFactory) Storage(ctx context.Context, secretRef corev1.SecretReference) (Storage, error) {
	serviceURL, err := newStorageClient(ctx, f.client, &secretRef)
	if err != nil {
		return nil, err
	}

	return &StorageClient{
		serviceURL: serviceURL,
	}, nil
}

// StorageAccount reads the secret from the passed reference and return an Azure storage account client.
func (f azureFactory) StorageAccount() (StorageAccount, error) {
	authorizer, subscriptionID, err := internal.GetAuthorizerAndSubscriptionID(f.auth)
	if err != nil {
		return nil, err
	}
	storageAccountClient := azurestorage.NewAccountsClient(subscriptionID)
	storageAccountClient.Authorizer = authorizer

	return &StorageAccountClient{
		client: storageAccountClient,
	}, nil
}

// DNSZone reads the secret from the passed reference and return an Azure DNS zone client.
func (f azureFactory) DNSZone() (DNSZone, error) {
	authorizer, subscriptionID, err := internal.GetAuthorizerAndSubscriptionID(f.auth)
	if err != nil {
		return nil, err
	}
	zonesClient := azuredns.NewZonesClient(subscriptionID)
	zonesClient.Authorizer = authorizer

	return &DNSZoneClient{
		client: zonesClient,
	}, nil
}

// DNSRecordSet reads the secret from the passed reference and return an Azure DNS record set client.
func (f azureFactory) DNSRecordSet() (DNSRecordSet, error) {
	authorizer, subscriptionID, err := internal.GetAuthorizerAndSubscriptionID(f.auth)
	if err != nil {
		return nil, err
	}
	recordSetsClient := azuredns.NewRecordSetsClient(subscriptionID)
	recordSetsClient.Authorizer = authorizer

	return &DNSRecordSetClient{
		client: recordSetsClient,
	}, nil
}

// Group gets an Azure resource group client.
func (f azureFactory) Group() (ResourceGroup, error) {
	return NewResourceGroupsClient(f.auth, f.tokenCredential, DefaultAzureClientOpts())
}

// Vmss reads the secret from the passed reference and return an Azure virtual machine scale set client.
func (f azureFactory) Vmss() (Vmss, error) {
	return NewVmssClient(*f.auth, f.tokenCredential, DefaultAzureClientOpts())
}

// VirtualMachine reads the secret from the passed reference and return an Azure virtual machine client.
func (f azureFactory) VirtualMachine() (VirtualMachine, error) {
	return NewVMClient(*f.auth, f.tokenCredential, DefaultAzureClientOpts())
}

// NetworkSecurityGroup reads the secret from the passed reference and return an Azure network security group client.
func (f azureFactory) NetworkSecurityGroup() (NetworkSecurityGroup, error) {
	return NewSecurityGroupClient(*f.auth, f.tokenCredential, DefaultAzureClientOpts())
}

// PublicIP reads the secret from the passed reference and return an Azure network PublicIPClient.
func (f azureFactory) PublicIP() (PublicIP, error) {
	return NewPublicIPClient(*f.auth, f.tokenCredential, DefaultAzureClientOpts())

}

// NetworkInterface reads the secret from the passed reference and return an Azure network interface client.
func (f azureFactory) NetworkInterface() (NetworkInterface, error) {
	return NewNetworkInterfaceClient(*f.auth, f.tokenCredential, DefaultAzureClientOpts())
}

// Disk reads the secret from the passed reference and return an Azure disk client.
func (f azureFactory) Disk() (Disk, error) {
	return NewDisksClient(*f.auth, f.tokenCredential, DefaultAzureClientOpts())
}

// Vnet reads the secret from the passed reference and return an Azure Vnet client.
func (f azureFactory) Vnet() (VirtualNetwork, error) {
	return NewVnetClient(*f.auth, f.tokenCredential, DefaultAzureClientOpts())
}

// Subnet reads the secret from the passed reference and return an Azure Subnet client.
func (f azureFactory) Subnet() (Subnet, error) {
	return NewSubnetsClient(*f.auth, f.tokenCredential, DefaultAzureClientOpts())
}

// RouteTables reads the secret from the passed reference and return an Azure RouteTables client.
func (f azureFactory) RouteTables() (RouteTables, error) {
	return NewRouteTablesClient(*f.auth, f.tokenCredential, DefaultAzureClientOpts())
}

// NatGateway returns a NatGateway client.
func (f azureFactory) NatGateway() (NatGateway, error) {
	return NewNatGatewaysClient(*f.auth, f.tokenCredential, DefaultAzureClientOpts())
}

// AvailabilitySet returns an AvailabilitySet client.
func (f azureFactory) AvailabilitySet() (AvailabilitySet, error) {
	return NewAvailabilitySetClient(*f.auth)
}

// ManagedUserIdentity returns a ManagedUserIdentity client.
func (f azureFactory) ManagedUserIdentity() (ManagedUserIdentity, error) {
	return NewManagedUserIdentityClient(*f.auth)
}

// VirtualMachineImages returns a VirtualMachineImages client.
func (f azureFactory) VirtualMachineImages() (VirtualMachineImages, error) {
	return NewVirtualMachineImagesClient(*f.auth)
}
