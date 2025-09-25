// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
)

var _ PublicIP = &PublicIPClient{}

// PublicIPClient is an implementation of Network Public IP Address.
type PublicIPClient struct {
	client *armnetwork.PublicIPAddressesClient
}

// NewPublicIPClient creates a new PublicIPClient
func NewPublicIPClient(auth ClientAuth, tc azcore.TokenCredential, opts *arm.ClientOptions) (*PublicIPClient, error) {
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
		ips = append(ips, res.Value...)
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
