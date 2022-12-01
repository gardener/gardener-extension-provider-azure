// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

// NewSubnetsClient creates a new subnets client.
func NewSubnetsClient(auth internal.ClientAuth) (*SubnetsClient, error) {
	cred, err := auth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}
	client, err := armnetwork.NewSubnetsClient(auth.SubscriptionID, cred, nil)
	return &SubnetsClient{client}, err
}

// CreateOrUpdate creates or updates a subnet in a given virtual network.
func (c SubnetsClient) CreateOrUpdate(ctx context.Context, resourceGroupName, vnetName, subnetName string, parameters armnetwork.Subnet) error {
	poller, err := c.client.BeginCreateOrUpdate(ctx, resourceGroupName, vnetName, subnetName, parameters, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

// Get will get a subnet in a given virtual network. If the requested subnet not exists nil will be returned.
// TODO remove expander
func (c SubnetsClient) Get(ctx context.Context, resourceGroupName string, vnetName string, name string, expander string) (*armnetwork.SubnetsClientGetResponse, error) {
	subnet, err := c.client.Get(ctx, resourceGroupName, vnetName, name, nil)
	if err != nil {
		if IsAzureAPINotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}
	return &subnet, nil
}

// List lists all subnets of a given virtual network.
func (c SubnetsClient) List(ctx context.Context, resourceGroupName, vnetName string) ([]*armnetwork.Subnet, error) {
	pager := c.client.NewListPager(resourceGroupName, vnetName, nil)
	subnetList := []*armnetwork.Subnet{}
	for pager.More() {
		page, err := pager.NextPage(ctx)
		subnetList = append(subnetList, page.Value...)
		if err != nil {
			return nil, err
		}
	}
	return subnetList, nil
}

// Delete deletes a subnet in a given virtual network.
func (c SubnetsClient) Delete(ctx context.Context, resourceGroupName, vnetName, subnetName string) error {
	poller, err := c.client.BeginDelete(ctx, resourceGroupName, vnetName, subnetName, nil)
	if err != nil {
		return err
	}

	_, err = poller.PollUntilDone(ctx, nil)
	return err
}
