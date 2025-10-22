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

var _ NetworkSecurityGroup = &NetworkSecurityGroupClient{}

// NetworkSecurityGroupClient is an implementation of Network Security Group for a network security group k8sClient.
type NetworkSecurityGroupClient struct {
	client *armnetwork.SecurityGroupsClient
}

// NewSecurityGroupClient creates a new SecurityGroupClient
func NewSecurityGroupClient(auth ClientAuth, tc azcore.TokenCredential, opts *arm.ClientOptions) (*NetworkSecurityGroupClient, error) {
	client, err := armnetwork.NewSecurityGroupsClient(auth.SubscriptionID, tc, opts)
	return &NetworkSecurityGroupClient{client}, err
}

// CreateOrUpdate indicates an expected call of Network Security Group CreateOrUpdate.
func (c *NetworkSecurityGroupClient) CreateOrUpdate(ctx context.Context, resourceGroupName, name string, parameters armnetwork.SecurityGroup) (*armnetwork.SecurityGroup, error) {
	future, err := c.client.BeginCreateOrUpdate(ctx, resourceGroupName, name, parameters, nil)
	if err != nil {
		return nil, err
	}
	nsg, err := future.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &nsg.SecurityGroup, nil
}

// Get will fetch a network security group.
func (c *NetworkSecurityGroupClient) Get(ctx context.Context, resourceGroupName string, networkSecurityGroupName string) (*armnetwork.SecurityGroup, error) {
	nsg, err := c.client.Get(ctx, resourceGroupName, networkSecurityGroupName, nil)
	if err != nil {
		return nil, FilterNotFoundError(err)
	}
	return &nsg.SecurityGroup, nil
}

// Delete deletes a network security group.
func (c *NetworkSecurityGroupClient) Delete(ctx context.Context, resourceGroupName, name string) error {
	future, err := c.client.BeginDelete(ctx, resourceGroupName, name, nil)
	if err != nil {
		return FilterNotFoundError(err)
	}
	if _, err := future.PollUntilDone(ctx, nil); err != nil {
		return err
	}
	return err
}
