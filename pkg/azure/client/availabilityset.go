// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

var _ AvailabilitySet = &AvailabilitySetClient{}

// AvailabilitySetClient is an implementation of AvailabilitySet for an availability set k8sClient.
type AvailabilitySetClient struct {
	client *armcompute.AvailabilitySetsClient
}

// NewAvailabilitySetClient creates a new AvailabilitySetClient.
func NewAvailabilitySetClient(auth internal.ClientAuth, tc azcore.TokenCredential, opts *policy.ClientOptions) (*AvailabilitySetClient, error) {
	client, err := armcompute.NewAvailabilitySetsClient(auth.SubscriptionID, tc, opts)
	return &AvailabilitySetClient{client}, err
}

// CreateOrUpdate creates or updates a new availability set.
func (c *AvailabilitySetClient) CreateOrUpdate(ctx context.Context, resourceGroupName, availabilitySetName string, parameters armcompute.AvailabilitySet) (*armcompute.AvailabilitySet, error) {
	res, err := c.client.CreateOrUpdate(ctx, resourceGroupName, availabilitySetName, parameters, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create availability set: %v", err)
	}
	return &res.AvailabilitySet, nil
}

// Get returns the availability set for the given resource group and availability set name.
func (c *AvailabilitySetClient) Get(ctx context.Context, resourceGroupName, availabilitySetName string) (*armcompute.AvailabilitySet, error) {
	res, err := c.client.Get(ctx, resourceGroupName, availabilitySetName, nil)
	if err != nil {
		return nil, FilterNotFoundError(err)
	}
	return &res.AvailabilitySet, err
}

// Delete deletes the availability set for the given resource group and availability set name.
func (c *AvailabilitySetClient) Delete(ctx context.Context, resourceGroupName, availabilitySetName string) error {
	_, err := c.client.Delete(ctx, resourceGroupName, availabilitySetName, nil)
	return FilterNotFoundError(err)
}
