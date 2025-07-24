// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

var _ NatGateway = &NatGatewayClient{}

// NatGatewayClient is an implementation of NatGateway for a Nat Gateway k8sClient.
type NatGatewayClient struct {
	client *armnetwork.NatGatewaysClient
}

// NewNatGatewaysClient creates a new NatGateway client.
func NewNatGatewaysClient(auth internal.ClientAuth, tc azcore.TokenCredential, opts *arm.ClientOptions) (*NatGatewayClient, error) {
	client, err := armnetwork.NewNatGatewaysClient(auth.SubscriptionID, tc, opts)
	return &NatGatewayClient{client}, err
}

// CreateOrUpdate creates or updates a NatGateway.
func (c *NatGatewayClient) CreateOrUpdate(ctx context.Context, resourceGroupName, natGatewayName string, parameters armnetwork.NatGateway) (*armnetwork.NatGateway, error) {
	poller, err := c.client.BeginCreateOrUpdate(ctx, resourceGroupName, natGatewayName, parameters, nil)
	if err != nil {
		return nil, err
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	return &resp.NatGateway, err
}

// Get returns a NatGateway by name or nil if it doesn't exist.
func (c *NatGatewayClient) Get(ctx context.Context, resourceGroupName, natGatewayName string, expand *string) (*armnetwork.NatGateway, error) {
	var opts *armnetwork.NatGatewaysClientGetOptions
	if expand != nil {
		opts = &armnetwork.NatGatewaysClientGetOptions{
			Expand: expand,
		}
	}
	res, err := c.client.Get(ctx, resourceGroupName, natGatewayName, opts)
	if err != nil {
		return nil, FilterNotFoundError(err)
	}
	return &res.NatGateway, nil
}

// List returns all NATGateways in the given resource group.
func (c *NatGatewayClient) List(ctx context.Context, resourceGroupName string) ([]*armnetwork.NatGateway, error) {
	pager := c.client.NewListPager(resourceGroupName, nil)
	var nats []*armnetwork.NatGateway
	for pager.More() {
		res, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		nats = append(nats, res.Value...)
	}
	return nats, nil
}

// Delete deletes the NatGateway with the given name.
func (c *NatGatewayClient) Delete(ctx context.Context, resourceGroupName, natGatewayName string) error {
	poller, err := c.client.BeginDelete(ctx, resourceGroupName, natGatewayName, nil)
	if err != nil {
		return FilterNotFoundError(err)
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}
