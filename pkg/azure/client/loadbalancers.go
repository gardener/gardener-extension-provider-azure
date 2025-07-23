// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
)

// LoadBalancersClient implements the interface for the LoadBalancers client.
type LoadBalancersClient struct {
	client *armnetwork.LoadBalancersClient
}

// NewLoadBalancersClient creates a new client for the LoadBalancers API.
func NewLoadBalancersClient(auth ClientAuth, tc azcore.TokenCredential, opts *arm.ClientOptions) (*LoadBalancersClient, error) {
	client, err := armnetwork.NewLoadBalancersClient(auth.SubscriptionID, tc, opts)
	return &LoadBalancersClient{client}, err
}

// Get gets a given virtual load balancer by name
func (c *LoadBalancersClient) Get(ctx context.Context, resourceGroupName, name string) (*armnetwork.LoadBalancer, error) {
	res, err := c.client.Get(ctx, resourceGroupName, name, nil)
	if err != nil {
		return nil, FilterNotFoundError(err)
	}
	return &res.LoadBalancer, err
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

// CreateOrUpdate creates or updates a load balancer.
func (c *LoadBalancersClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, name string, parameters armnetwork.LoadBalancer) (*armnetwork.LoadBalancer, error) {
	poller, err := c.client.BeginCreateOrUpdate(ctx, resourceGroupName, name, parameters, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create loadbalancer: %v", err)
	}
	res, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create loadbalancer: %v", err)
	}
	return &res.LoadBalancer, err
}

// BackendAddressPoolClient is a client for managing backend address pools of Azure Load Balancers.
type BackendAddressPoolClient struct {
	client *armnetwork.LoadBalancerBackendAddressPoolsClient
}

// NewBackendAddressPoolClient creates a new BackendAddressPoolClient.
func NewBackendAddressPoolClient(auth ClientAuth, tc azcore.TokenCredential, opts *arm.ClientOptions) (*BackendAddressPoolClient, error) {
	client, err := armnetwork.NewLoadBalancerBackendAddressPoolsClient(auth.SubscriptionID, tc, opts)
	return &BackendAddressPoolClient{client}, err
}

// Get gets a given virtual load balancer by name
func (c *BackendAddressPoolClient) Get(ctx context.Context, resourceGroupName, lbName string, name string) (*armnetwork.BackendAddressPool, error) {
	res, err := c.client.Get(ctx, resourceGroupName, lbName, name, nil)
	if err != nil {
		return nil, FilterNotFoundError(err)
	}
	return &res.BackendAddressPool, err
}

// List lists all subnets of a given virtual network.
func (c *BackendAddressPoolClient) List(ctx context.Context, resourceGroupName, lbName string) ([]*armnetwork.BackendAddressPool, error) {
	pager := c.client.NewListPager(resourceGroupName, lbName, nil)
	var bp []*armnetwork.BackendAddressPool
	for pager.More() {
		page, err := pager.NextPage(ctx)
		bp = append(bp, page.Value...)
		if err != nil {
			return nil, err
		}
	}
	return bp, nil
}

// Delete deletes a subnet in a given virtual network.
func (c *BackendAddressPoolClient) Delete(ctx context.Context, resourceGroupName, loadBalancerName, name string) error {
	poller, err := c.client.BeginDelete(ctx, resourceGroupName, loadBalancerName, name, nil)
	if err != nil {
		return FilterNotFoundError(err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

// CreateOrUpdate creates or updates a load balancer.
func (c *BackendAddressPoolClient) CreateOrUpdate(ctx context.Context, resourceGroupName, lbName, name string, parameters armnetwork.BackendAddressPool) (*armnetwork.BackendAddressPool, error) {
	poller, err := c.client.BeginCreateOrUpdate(ctx, resourceGroupName, lbName, name, parameters, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create loadbalancer: %v", err)
	}
	res, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create loadbalancer: %v", err)
	}
	return &res.BackendAddressPool, err
}

// LoadBalancersFrontEndIPConfigurationClient is a client for managing frontend IP configurations of Azure Load Balancers.
type LoadBalancersFrontEndIPConfigurationClient struct {
	client *armnetwork.LoadBalancerFrontendIPConfigurationsClient
}

// NewLoadBalancersFrontEndIPConfiguration creates a new LoadBalancersFrontEndIPConfigurationClient.
func NewLoadBalancersFrontEndIPConfiguration(auth ClientAuth, tc azcore.TokenCredential, opts *arm.ClientOptions) (*LoadBalancersFrontEndIPConfigurationClient, error) {
	client, err := armnetwork.NewLoadBalancerFrontendIPConfigurationsClient(auth.SubscriptionID, tc, opts)
	return &LoadBalancersFrontEndIPConfigurationClient{client}, err
}

// Get gets a given virtual load balancer by name
func (c *LoadBalancersFrontEndIPConfigurationClient) Get(ctx context.Context, resourceGroupName, lbName string, name string) (*armnetwork.FrontendIPConfiguration, error) {
	res, err := c.client.Get(ctx, resourceGroupName, lbName, name, nil)
	if err != nil {
		return nil, FilterNotFoundError(err)
	}
	return &res.FrontendIPConfiguration, err
}

// List lists all subnets of a given virtual network.
func (c *LoadBalancersFrontEndIPConfigurationClient) List(ctx context.Context, resourceGroupName, lbName string) ([]*armnetwork.FrontendIPConfiguration, error) {
	pager := c.client.NewListPager(resourceGroupName, lbName, nil)
	var res []*armnetwork.FrontendIPConfiguration
	for pager.More() {
		page, err := pager.NextPage(ctx)
		res = append(res, page.Value...)
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}
