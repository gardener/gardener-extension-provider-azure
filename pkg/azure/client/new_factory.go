package client

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

type NewFactory interface {
	ResourceGroup() (ResourceGroup, error)
	Vnet() (Vnet, error)
	RouteTables() (RouteTables, error)
	SecurityGroups() (SecurityGroups, error)
	Subnet() (Subnet, error)
	NatGateway() (NatGateway, error)
	PublicIP() (NewPublicIP, error)
	AvailabilitySet() (AvailabilitySet, error)
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
	return NewVnetClient(f.auth)
}

func (f newFactory) RouteTables() (RouteTables, error) {
	return NewRouteTablesClient(f.auth)
}

func (f newFactory) SecurityGroups() (SecurityGroups, error) {
	return NewSecurityGroupClient(f.auth)
}

func (f newFactory) AvailabilitySet() (AvailabilitySet, error) {
	return NewAvailabilitySetClient(f.auth)
}

func (f newFactory) Subnet() (Subnet, error) {
	return NewSubnetsClient(f.auth)
}

func (f newFactory) NatGateway() (NatGateway, error) {
	return NewNatGatewaysClient(f.auth)
}

func (f newFactory) PublicIP() (NewPublicIP, error) {
	return NewPublicIPsClient(f.auth)
}
