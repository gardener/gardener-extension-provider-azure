package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
)

func (c NatGatewayClient) CreateOrUpdate(ctx context.Context, resourceGroupName, natGatewayName string, parameters armnetwork.NatGateway) error {
	poller, err := c.client.BeginCreateOrUpdate(ctx, resourceGroupName, natGatewayName, parameters, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

func (c NatGatewayClient) Get(ctx context.Context, resourceGroupName, natGatewayName string) (*armnetwork.NatGatewaysClientGetResponse, error) {
	natGateway, err := c.client.Get(ctx, resourceGroupName, natGatewayName, nil)
	if err != nil {
		if IsAzureAPINotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}
	return &natGateway, nil
}

func (c NatGatewayClient) Delete(ctx context.Context, resourceGroupName, natGatewayName string) error {
	poller, err := c.client.BeginDelete(ctx, resourceGroupName, natGatewayName, nil)
	if err != nil {
		return err
	}

	_, err = poller.PollUntilDone(ctx, nil)
	return err
}
