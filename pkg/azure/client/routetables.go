package client

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
)

// TODO interface + constructor

func (c RouteTableClient) CreateOrUpdate(ctx context.Context, resourceGroupName, routeTableName string, parameters armnetwork.RouteTable) (err error) {
	poller, err := c.client.BeginCreateOrUpdate(ctx, resourceGroupName, routeTableName, parameters, nil)
	if err != nil {
		return fmt.Errorf("cannot create route table: %v", err)
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

func (c RouteTableClient) Delete(ctx context.Context, resourceGroupName, name string) (err error) {
	poller, err := c.client.BeginDelete(ctx, resourceGroupName, name, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

func (c RouteTableClient) Get(ctx context.Context, resourceGroupName, name string) (armnetwork.RouteTablesClientGetResponse, error) {
	return c.client.Get(ctx, resourceGroupName, name, nil)
}
