package client

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
)

// TODO create interface
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
