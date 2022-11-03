package client

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

func NewVnetClient(auth internal.ClientAuth) (*VnetClient, error) {
	cred, err := auth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}
	client, err := armnetwork.NewVirtualNetworksClient(auth.SubscriptionID, cred, nil)
	return &VnetClient{client}, err
}

// TODO create interface
// TODO ddos Protection plan id in caller .. (use json unmarshall?)
func (v VnetClient) Create(ctx context.Context, resourceGroupName string, name string, parameters armnetwork.VirtualNetwork) (err error) {
	poller, err := v.client.BeginCreateOrUpdate(ctx, resourceGroupName, name, parameters, nil)
	if err != nil {
		return fmt.Errorf("cannot create virtual network: %v", err)
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

// Delete a given an existing virtual network
func (v VnetClient) Delete(ctx context.Context, resourceGroup, vnetName string) (err error) {
	poller, err := v.client.BeginDelete(ctx, resourceGroup, vnetName, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

func (v VnetClient) Get(ctx context.Context, resourceGroupName, name string) (armnetwork.VirtualNetworksClientGetResponse, error) {
	res, err := v.client.Get(ctx, resourceGroupName, name, nil)
	return res, err
}
