// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

var _ ResourceGroup = &ResourceGroupClient{}

// ResourceGroupClient is a client for resource groups.
type ResourceGroupClient struct {
	client *armresources.ResourceGroupsClient
}

// NewResourceGroupsClient creates a new ResourceGroupClient
func NewResourceGroupsClient(auth *internal.ClientAuth, tc azcore.TokenCredential, opts *arm.ClientOptions) (*ResourceGroupClient, error) {
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
