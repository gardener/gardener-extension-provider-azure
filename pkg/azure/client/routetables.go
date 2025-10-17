// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
)

var _ RouteTables = &RouteTablesClient{}

// RouteTablesClient is an implementation of RouteTables for a RouteTables k8sClient.
type RouteTablesClient struct {
	client *armnetwork.RouteTablesClient
}

// NewRouteTablesClient creates a new RouteTables client.
func NewRouteTablesClient(auth ClientAuth, tc azcore.TokenCredential, opts *arm.ClientOptions) (*RouteTablesClient, error) {
	client, err := armnetwork.NewRouteTablesClient(auth.SubscriptionID, tc, opts)
	return &RouteTablesClient{client}, err
}

// CreateOrUpdate creates or updates a RouteTable.
func (c *RouteTablesClient) CreateOrUpdate(ctx context.Context, resourceGroupName, routeTableName string, parameters armnetwork.RouteTable) (*armnetwork.RouteTable, error) {
	poller, err := c.client.BeginCreateOrUpdate(ctx, resourceGroupName, routeTableName, parameters, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create route table: %v", err)
	}
	res, err := poller.PollUntilDone(ctx, nil)
	return &res.RouteTable, err
}

// Delete deletes the RouteTable with the given name.
func (c *RouteTablesClient) Delete(ctx context.Context, resourceGroupName, name string) (err error) {
	poller, err := c.client.BeginDelete(ctx, resourceGroupName, name, nil)
	if err != nil {
		return FilterNotFoundError(err)
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

// Get returns a RouteTable by name.
func (c *RouteTablesClient) Get(ctx context.Context, resourceGroupName, name string) (*armnetwork.RouteTable, error) {
	res, err := c.client.Get(ctx, resourceGroupName, name, nil)
	if err != nil {
		return nil, FilterNotFoundError(err)
	}
	return &res.RouteTable, err
}
