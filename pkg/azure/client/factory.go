// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

// AzureFactoryOption represents an option for the AzureFactory constructor.
type AzureFactoryOption func(*azureFactory)

// WithCloudConfiguration is the option that sets the cloud configuration on the factory
func WithCloudConfiguration(cloudConfiguration cloud.Configuration) AzureFactoryOption {
	return func(f *azureFactory) {
		f.clientOpts.Cloud = cloudConfiguration
	}
}

// AzureFactory is an implementation of Factory to produce clients for various Azure services.
type azureFactory struct {
	auth            *internal.ClientAuth
	tokenCredential azcore.TokenCredential
	clientOpts      *policy.ClientOptions
}

// NewAzureClientFactoryFromSecret builds the factory from the given secret (by ref).
func NewAzureClientFactoryFromSecret(
	ctx context.Context,
	client client.Client,
	secretRef corev1.SecretReference,
	isDNSSecret bool,
	options ...AzureFactoryOption,
) (Factory, error) {
	auth, secret, err := internal.GetClientAuthData(ctx, client, secretRef, isDNSSecret)
	if err != nil {
		return nil, err
	}
	if isDNSSecret {
		acc, err := cloudConfigurationFromSecret(secret)
		if err != nil {
			return nil, err
		}
		// prepend the cloud configuration from the secret in favor of the explicit ones that may be passed from options.
		options = append([]AzureFactoryOption{WithCloudConfiguration(acc)}, options...)
	}
	return NewAzureClientFactory(auth, options...)
}

// NewAzureClientFactory constructs a new factory using the provided Credentials and applying the provided options.
func NewAzureClientFactory(authCredentials *internal.ClientAuth, options ...AzureFactoryOption) (Factory, error) {
	// prepare tokenCredential for more convenient access later on
	cred, err := authCredentials.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}

	factory := &azureFactory{
		auth:            authCredentials,
		tokenCredential: cred,
		clientOpts:      DefaultAzureClientOpts(),
	}

	for _, option := range options {
		option(factory)
	}

	return *factory, nil
}

// StorageAccount returns an Azure storage account client.
func (f azureFactory) StorageAccount() (StorageAccount, error) {
	return NewStorageAccountClient(f.auth, f.tokenCredential, f.clientOpts)
}

// DNSZone returns an Azure DNS zone client.
func (f azureFactory) DNSZone() (DNSZone, error) {
	return NewDnsZoneClient(f.auth, f.tokenCredential, f.clientOpts)
}

// DNSRecordSet returns an Azure DNS record set client.
func (f azureFactory) DNSRecordSet() (DNSRecordSet, error) {
	return NewDnsRecordSetClient(f.auth, f.tokenCredential, f.clientOpts)
}

// Group returns an Azure resource group client.
func (f azureFactory) Group() (ResourceGroup, error) {
	return NewResourceGroupsClient(f.auth, f.tokenCredential, f.clientOpts)
}

// Resource returns an Azure resource client.
func (f azureFactory) Resource() (Resource, error) {
	return NewResourceClient(f.auth, f.tokenCredential, f.clientOpts)
}

// Vmss returns an Azure virtual machine scale set client.
func (f azureFactory) Vmss() (Vmss, error) {
	return NewVmssClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// VirtualMachine returns an Azure virtual machine client.
func (f azureFactory) VirtualMachine() (VirtualMachine, error) {
	return NewVMClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// NetworkSecurityGroup returns an Azure network security group client.
func (f azureFactory) NetworkSecurityGroup() (NetworkSecurityGroup, error) {
	return NewSecurityGroupClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// PublicIP returns an Azure network PublicIPClient.
func (f azureFactory) PublicIP() (PublicIP, error) {
	return NewPublicIPClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// NetworkInterface returns an Azure network interface client.
func (f azureFactory) NetworkInterface() (NetworkInterface, error) {
	return NewNetworkInterfaceClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// Disk returns an Azure disk client.
func (f azureFactory) Disk() (Disk, error) {
	return NewDisksClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// Vnet returns an Azure Vnet client.
func (f azureFactory) Vnet() (VirtualNetwork, error) {
	return NewVnetClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// Subnet returns an Azure Subnet client.
func (f azureFactory) Subnet() (Subnet, error) {
	return NewSubnetsClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// LoadBalancer returns an Azure LoadBalancer client.
func (f azureFactory) LoadBalancer() (LoadBalancer, error) {
	return NewLoadBalancersClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// RouteTables returns an Azure RouteTables client.
func (f azureFactory) RouteTables() (RouteTables, error) {
	return NewRouteTablesClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// NatGateway returns a NatGateway client.
func (f azureFactory) NatGateway() (NatGateway, error) {
	return NewNatGatewaysClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// AvailabilitySet returns an AvailabilitySet client.
func (f azureFactory) AvailabilitySet() (AvailabilitySet, error) {
	return NewAvailabilitySetClient(*f.auth, f.tokenCredential, f.clientOpts)
}

// ManagedUserIdentity returns a ManagedUserIdentity client.
func (f azureFactory) ManagedUserIdentity() (ManagedUserIdentity, error) {
	return NewManagedUserIdentityClient(f.auth, f.tokenCredential, f.clientOpts)
}

// VirtualMachineImages returns a VirtualMachineImages client.
func (f azureFactory) VirtualMachineImages() (VirtualMachineImages, error) {
	return NewVirtualMachineImagesClient(f.auth, f.tokenCredential, f.clientOpts)
}
