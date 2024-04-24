// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
type azureFactory struct {
	auth            *internal.ClientAuth
	tokenCredential azcore.TokenCredential
}

// NewAzureClientFactory creates a new Azure client factory with the passed secret reference.
func NewAzureClientFactory(ctx context.Context, client client.Client, secretRef corev1.SecretReference) (Factory, error) {
	auth, err := internal.GetClientAuthData(ctx, client, secretRef, false)
	if err != nil {
		return nil, err
	}
	return NewAzureClientFactoryWithAuth(auth)
}

// NewAzureClientFactoryWithDNSSecret creates a new Azure client factory with the passed secret reference using the DNS secret keys.
func NewAzureClientFactoryWithDNSSecret(ctx context.Context, client client.Client, secretRef corev1.SecretReference) (Factory, error) {
	auth, err := internal.GetClientAuthData(ctx, client, secretRef, true)
	if err != nil {
		return nil, err
	}
	return NewAzureClientFactoryWithAuth(auth)
}

// NewAzureClientFactoryWithAuth creates a new Azure client factory with the passed credentials.
func NewAzureClientFactoryWithAuth(auth *internal.ClientAuth) (Factory, error) {
	cred, err := auth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}
	return azureFactory{
		auth:            auth,
		tokenCredential: cred,
	}, nil
}

func (f azureFactory) Auth() *internal.ClientAuth {
	return f.auth
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

// Resource returns an Azure resource client.
func (f azureFactory) Resource() (Resource, error) {
	return NewResourceClient(f.auth, f.tokenCredential, f.clientOpts)
}

// Vmss returns an Azure virtual machine scale set client.
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

// NewBlobStorageClient reads the secret from the passed reference and return an Azure (blob) storage client.
func NewBlobStorageClient(ctx context.Context, c client.Client, secretRef corev1.SecretReference) (Storage, error) {
	serviceURL, err := newStorageClient(ctx, c, &secretRef)
	if err != nil {
		return nil, err
	}

	return &StorageClient{
		serviceURL: serviceURL,
	}, nil
}
