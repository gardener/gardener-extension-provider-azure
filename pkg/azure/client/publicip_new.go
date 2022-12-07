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

// NewPublicIPClient returns a new client for Azure Public IP Addresses.
func NewPublicIPsClient(auth internal.ClientAuth) (*NewPublicIPClient, error) {
	cred, err := auth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}
	client, err := armnetwork.NewPublicIPAddressesClient(auth.SubscriptionID, cred, nil)
	return &NewPublicIPClient{client}, err
}

// CreateOrUpdate indicates an expected call of Network Public IP CreateOrUpdate.
func (c *NewPublicIPClient) CreateOrUpdate(ctx context.Context, resourceGroupName, name string, parameters armnetwork.PublicIPAddress) (a armnetwork.PublicIPAddressesClientCreateOrUpdateResponse, err error) {
	poller, err := c.client.BeginCreateOrUpdate(ctx, resourceGroupName, name, parameters, nil)
	if err != nil {
		return a, err
	}
	res, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return a, err
	}
	return res, nil
}

// Get will get a network public IP Address
func (c NewPublicIPClient) Get(ctx context.Context, resourceGroupName string, name string) (armnetwork.PublicIPAddressesClientGetResponse, error) {
	return c.client.Get(ctx, resourceGroupName, name, nil)
}

func (c NewPublicIPClient) GetAll(ctx context.Context, resourceGroupName string) ([]*armnetwork.PublicIPAddress, error) {
	pager := c.client.NewListPager(resourceGroupName, nil)
	var ips []*armnetwork.PublicIPAddress
	for pager.More() {
		res, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		ips = append(ips, res.PublicIPAddressListResult.Value...)
	}
	return ips, nil
}

// Delete will delete a network Public IP Address.
func (c NewPublicIPClient) Delete(ctx context.Context, resourceGroupName, name string) error {
	poller, err := c.client.BeginDelete(ctx, resourceGroupName, name, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}
