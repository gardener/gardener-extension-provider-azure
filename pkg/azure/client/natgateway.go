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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

// NewNatGatewaysClient creates a new NatGateway client.
func NewNatGatewaysClient(auth internal.ClientAuth) (*NatGatewayClient, error) {
	cred, err := auth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}
	client, err := armnetwork.NewNatGatewaysClient(auth.SubscriptionID, cred, nil)
	return &NatGatewayClient{client}, err
}

// CreateOrUpdate creates or updates a NatGateway.
func (c NatGatewayClient) CreateOrUpdate(ctx context.Context, resourceGroupName, natGatewayName string, parameters armnetwork.NatGateway) (*armnetwork.NatGateway, error) {

	poller, err := c.client.BeginCreateOrUpdate(ctx, resourceGroupName, natGatewayName, parameters, nil)
	if err != nil {
		return nil, err
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	return &resp.NatGateway, err
}

// Get returns a NatGateway by name or nil if it doesn't exis.
func (c NatGatewayClient) Get(ctx context.Context, resourceGroupName, natGatewayName string) (*armnetwork.NatGateway, error) {
	res, err := c.client.Get(ctx, resourceGroupName, natGatewayName, nil)
	if err != nil {
		return nil, FilterNotFoundError(err)
	}
	return &res.NatGateway, nil
}

// List returns all NATGateways in the given resource group.
func (c NatGatewayClient) List(ctx context.Context, resourceGroupName string) ([]*armnetwork.NatGateway, error) {
	pager := c.client.NewListPager(resourceGroupName, nil)
	var nats []*armnetwork.NatGateway
	for pager.More() {
		res, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		nats = append(nats, res.NatGatewayListResult.Value...)
	}
	return nats, nil
}

// Delete deletes the NatGateway with the given name.
func (c NatGatewayClient) Delete(ctx context.Context, resourceGroupName, natGatewayName string) error {
	poller, err := c.client.BeginDelete(ctx, resourceGroupName, natGatewayName, nil)
	if err != nil {
		return FilterNotFoundError(err)
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}
