// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

// NewRouteTablesClient creates a new RouteTables client.
func NewRouteTablesClient(auth internal.ClientAuth) (*RouteTablesClient, error) {
	cred, err := auth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}
	client, err := armnetwork.NewRouteTablesClient(auth.SubscriptionID, cred, nil)
	return &RouteTablesClient{client}, err
}

// CreateOrUpdate creates or updates a RouteTable.
func (c RouteTablesClient) CreateOrUpdate(ctx context.Context, resourceGroupName, routeTableName string, parameters armnetwork.RouteTable) (*armnetwork.RouteTable, error) {
	poller, err := c.client.BeginCreateOrUpdate(ctx, resourceGroupName, routeTableName, parameters, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create route table: %v", err)
	}
	res, err := poller.PollUntilDone(ctx, nil)
	return &res.RouteTable, err
}

// Delete deletes the RouteTable with the given name.
func (c RouteTablesClient) Delete(ctx context.Context, resourceGroupName, name string) (err error) {
	poller, err := c.client.BeginDelete(ctx, resourceGroupName, name, nil)
	if err != nil {
		return FilterNotFoundError(err)
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

// Get returns a RouteTable by name.
func (c RouteTablesClient) Get(ctx context.Context, resourceGroupName, name string) (*armnetwork.RouteTable, error) {
	res, err := c.client.Get(ctx, resourceGroupName, name, nil)
	if err != nil {
		return nil, FilterNotFoundError(err)
	}
	return &res.RouteTable, err
}
