package client

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

type NewFactory interface {
	ResourceGroup() (ResourceGroup, error)
	Vnet() (Vnet, error)
}

type newFactory struct {
	auth internal.ClientAuth
	cred *azidentity.ClientSecretCredential
}

func NewAzureClientFactoryV2(auth internal.ClientAuth) (newFactory, error) {
	cred, err := auth.GetAzClientCredentials()
	return newFactory{auth, cred}, err
}

func (f newFactory) ResourceGroup() (ResourceGroup, error) {
	c, err := armresources.NewResourceGroupsClient(f.auth.SubscriptionID, f.cred, nil)
	return ResourceGroupClient{c}, err
}

func (f newFactory) Vnet() (Vnet, error) {
	c, err := armnetwork.NewVirtualNetworksClient(f.auth.SubscriptionID, f.cred, nil)
	return VnetClient{c}, err
}
