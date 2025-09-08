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

var _ NetworkInterface = &NetworkInterfaceClient{}

// NetworkInterfaceClient is an implementation of Network Interface.
type NetworkInterfaceClient struct {
	client *armnetwork.InterfacesClient
}

// NewNetworkInterfaceClient creates a new NetworkInterfaceClient
func NewNetworkInterfaceClient(auth ClientAuth, tc azcore.TokenCredential, opts *arm.ClientOptions) (*NetworkInterfaceClient, error) {
	client, err := armnetwork.NewInterfacesClient(auth.SubscriptionID, tc, opts)
	return &NetworkInterfaceClient{client}, err
}

// CreateOrUpdate indicates an expected call of Network interface CreateOrUpdate.
func (c *NetworkInterfaceClient) CreateOrUpdate(ctx context.Context, resourceGroupName, name string, parameters armnetwork.Interface) (*armnetwork.Interface, error) {
	future, err := c.client.BeginCreateOrUpdate(ctx, resourceGroupName, name, parameters, nil)
	if err != nil {
		return nil, err
	}
	res, err := future.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &res.Interface, nil
}

// Get will get a Network interface.
func (c *NetworkInterfaceClient) Get(ctx context.Context, resourceGroupName string, name string) (*armnetwork.Interface, error) {
	nic, err := c.client.Get(ctx, resourceGroupName, name, nil)
	if err != nil {
		return nil, FilterNotFoundError(err)
	}
	return &nic.Interface, nil
}

// Delete will delete a Network interface.
func (c *NetworkInterfaceClient) Delete(ctx context.Context, resourceGroupName, name string) error {
	future, err := c.client.BeginDelete(ctx, resourceGroupName, name, nil)
	if err != nil {
		return FilterNotFoundError(err)
	}
	_, err = future.PollUntilDone(ctx, nil)
	return err
}
