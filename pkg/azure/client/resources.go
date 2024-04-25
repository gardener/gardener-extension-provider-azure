// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

var _ Resource = &ResourceClient{}

// ResourceClient is a client for resource groups.
type ResourceClient struct {
	client *armresources.Client
}

// NewResourceClient creates a new ResourceClient
func NewResourceClient(auth *internal.ClientAuth, tc azcore.TokenCredential, opts *arm.ClientOptions) (*ResourceClient, error) {
	client, err := armresources.NewClient(auth.SubscriptionID, tc, opts)
	return &ResourceClient{client: client}, err
}

// ListByResourceGroup fetches all resources of a resource group.
func (c *ResourceClient) ListByResourceGroup(ctx context.Context, resourceGroupName string, options *armresources.ClientListByResourceGroupOptions) ([]*armresources.GenericResourceExpanded, error) {
	var res []*armresources.GenericResourceExpanded
	pager := c.client.NewListByResourceGroupPager(resourceGroupName, options)
	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		res = append(res, nextResult.Value...)
	}
	return res, nil
}
