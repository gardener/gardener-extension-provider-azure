// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

// DisksClient is an implementation of Disk for a disk k8sClient.
type DisksClient struct {
	client *armcompute.DisksClient
}

// NewDisksClient creates a new disk client
func NewDisksClient(auth internal.ClientAuth, tc azcore.TokenCredential, opts *arm.ClientOptions) (Disk, error) {
	client, err := armcompute.NewDisksClient(auth.SubscriptionID, tc, opts)
	return &DisksClient{client}, err
}

// Get will fetch a disk by given name in a given resource group.
func (c *DisksClient) Get(ctx context.Context, resourceGroupName string, name string) (*armcompute.Disk, error) {
	disk, err := c.client.Get(ctx, resourceGroupName, name, nil)
	if err != nil {
		return nil, FilterNotFoundError(err)
	}
	return &disk.Disk, nil
}

// CreateOrUpdate will create or update a disk.
func (c *DisksClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, diskName string, disk armcompute.Disk) (*armcompute.Disk, error) {
	future, err := c.client.BeginCreateOrUpdate(ctx, resourceGroupName, diskName, disk, nil)
	if err != nil {
		return nil, err
	}
	res, err := future.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &res.Disk, nil
}

// Delete will delete a disk.
func (c *DisksClient) Delete(ctx context.Context, resourceGroupName, name string) error {
	future, err := c.client.BeginDelete(ctx, resourceGroupName, name, nil)
	if err != nil {
		return err
	}
	_, err = future.PollUntilDone(ctx, nil)
	if err != nil {
		if IsAzureAPINotFoundError(err) {
			return nil
		}
	}
	return err
}
