package client

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

func NewRouteTablesClient(auth internal.ClientAuth) (*RouteTablesClient, error) {
	cred, err := auth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}
	client, err := armnetwork.NewRouteTablesClient(auth.SubscriptionID, cred, nil)
	return &RouteTablesClient{client}, err
}

func (c RouteTablesClient) CreateOrUpdate(ctx context.Context, resourceGroupName, routeTableName string, parameters armnetwork.RouteTable) (armnetwork.RouteTablesClientCreateOrUpdateResponse, error) {
	poller, err := c.client.BeginCreateOrUpdate(ctx, resourceGroupName, routeTableName, parameters, nil)
	if err != nil {
		return armnetwork.RouteTablesClientCreateOrUpdateResponse{}, fmt.Errorf("cannot create route table: %v", err)
	}
	return poller.PollUntilDone(ctx, nil)
}

func (c RouteTablesClient) Delete(ctx context.Context, resourceGroupName, name string) (err error) {
	poller, err := c.client.BeginDelete(ctx, resourceGroupName, name, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

func (c RouteTablesClient) Get(ctx context.Context, resourceGroupName, name string) (armnetwork.RouteTablesClientGetResponse, error) {
	return c.client.Get(ctx, resourceGroupName, name, nil)
}
