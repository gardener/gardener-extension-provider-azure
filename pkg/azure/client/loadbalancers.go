// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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

// LoadBalancersClient implements the interface for the LoadBalancers client.
type LoadBalancersClient struct {
	client *armnetwork.LoadBalancersClient
}

// NewLoadBalancersClient creates a new client for the LoadBalancers API.
func NewLoadBalancersClient(auth internal.ClientAuth, tc azcore.TokenCredential, opts *arm.ClientOptions) (*LoadBalancersClient, error) {
	client, err := armnetwork.NewLoadBalancersClient(auth.SubscriptionID, tc, opts)
	return &LoadBalancersClient{client}, err
}

// List lists all subnets of a given virtual network.
func (c *LoadBalancersClient) List(ctx context.Context, resourceGroupName string) ([]*armnetwork.LoadBalancer, error) {
	pager := c.client.NewListPager(resourceGroupName, nil)
	var loadBalancers []*armnetwork.LoadBalancer
	for pager.More() {
		page, err := pager.NextPage(ctx)
		loadBalancers = append(loadBalancers, page.Value...)
		if err != nil {
			return nil, err
		}
	}
	return loadBalancers, nil
}

// Delete deletes a subnet in a given virtual network.
func (c *LoadBalancersClient) Delete(ctx context.Context, resourceGroupName, loadBalancerName string) error {
	poller, err := c.client.BeginDelete(ctx, resourceGroupName, loadBalancerName, nil)
	if err != nil {
		return FilterNotFoundError(err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	return err
}
