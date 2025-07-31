// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

var _ ResourceGroup = &ResourceGroupClient{}

// ResourceGroupClient is a client for resource groups.
type ResourceGroupClient struct {
	client *armresources.ResourceGroupsClient
}

// NewResourceGroupsClient creates a new ResourceGroupClient
func NewResourceGroupsClient(auth *ClientAuth, tc azcore.TokenCredential, opts *arm.ClientOptions) (*ResourceGroupClient, error) {
	client, err := armresources.NewResourceGroupsClient(auth.SubscriptionID, tc, opts)
	return &ResourceGroupClient{client}, err
}

// Get gets a resource group.
func (c *ResourceGroupClient) Get(ctx context.Context, resourceGroupName string) (*armresources.ResourceGroup, error) {
	res, err := c.client.Get(ctx, resourceGroupName, nil)
	if err != nil {
		return nil, FilterNotFoundError(err)
	}
	return &res.ResourceGroup, err
}

// CreateOrUpdate creates or updates a resource group
func (c *ResourceGroupClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, resource armresources.ResourceGroup) (*armresources.ResourceGroup, error) {
	res, err := c.client.CreateOrUpdate(
		ctx,
		resourceGroupName,
		resource,
		nil)
	return &res.ResourceGroup, err
}

// Delete deletes a resource group if it exists.
func (c *ResourceGroupClient) Delete(ctx context.Context, resourceGroupName string) error {
	resourceGroupResp, err := c.client.BeginDelete(
		ctx,
		resourceGroupName,
		nil)
	if err != nil {
		return FilterNotFoundError(err)
	}
	_, err = resourceGroupResp.PollUntilDone(ctx, nil)
	return err
}

// CheckExistence checks if a resource group exists
func (c *ResourceGroupClient) CheckExistence(ctx context.Context, resourceGroupName string) (bool, error) {
	res, err := c.client.CheckExistence(ctx, resourceGroupName, nil)
	if err != nil {
		return false, err
	}
	return res.Success, err
}
