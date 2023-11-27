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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

var _ PublicIP = &PublicIPClient{}

// PublicIPClient is an implementation of Network Public IP Address.
type PublicIPClient struct {
	client *armnetwork.PublicIPAddressesClient
}

// NewPublicIPClient creates a new PublicIPClient
func NewPublicIPClient(auth internal.ClientAuth, tc azcore.TokenCredential, opts *arm.ClientOptions) (*PublicIPClient, error) {
	client, err := armnetwork.NewPublicIPAddressesClient(auth.SubscriptionID, tc, opts)
	return &PublicIPClient{client}, err
}

// CreateOrUpdate indicates an expected call of Network Public IP CreateOrUpdate.
func (c *PublicIPClient) CreateOrUpdate(ctx context.Context, resourceGroupName, name string, parameters armnetwork.PublicIPAddress) (*armnetwork.PublicIPAddress, error) {
	future, err := c.client.BeginCreateOrUpdate(ctx, resourceGroupName, name, parameters, nil)
	if err != nil {
		return nil, err
	}
	res, err := future.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &res.PublicIPAddress, nil
}

// Get will get a network public IP Address
func (c *PublicIPClient) Get(ctx context.Context, resourceGroupName string, name string, opts *string) (*armnetwork.PublicIPAddress, error) {
	var getOpts *armnetwork.PublicIPAddressesClientGetOptions
	if opts != nil {
		getOpts = &armnetwork.PublicIPAddressesClientGetOptions{
			Expand: opts,
		}
	}
	npi, err := c.client.Get(ctx, resourceGroupName, name, getOpts)

	if err != nil {
		return nil, FilterNotFoundError(err)
	}
	return &npi.PublicIPAddress, nil
}

// List will get all network public IP Addresses
func (c *PublicIPClient) List(ctx context.Context, resourceGroupName string) ([]*armnetwork.PublicIPAddress, error) {
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
func (c *PublicIPClient) Delete(ctx context.Context, resourceGroupName, name string) error {
	future, err := c.client.BeginDelete(ctx, resourceGroupName, name, nil)
	if err != nil {
		return FilterNotFoundError(err)
	}
	_, err = future.PollUntilDone(ctx, nil)
	return err
}
