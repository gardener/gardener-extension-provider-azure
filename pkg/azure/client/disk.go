// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
