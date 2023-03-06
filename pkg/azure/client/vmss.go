// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

// NewVmssClient creates a new VmssClient
func NewVmssClient(auth internal.ClientAuth) (Vmss, error) {
	cred, err := auth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}
	client, err := armcompute.NewVirtualMachineScaleSetsClient(auth.SubscriptionID, cred, nil)
	return &VmssClient{client}, err
}

// List will list vmss in a resource group.
func (c VmssClient) List(ctx context.Context, resourceGroupName string) ([]*armcompute.VirtualMachineScaleSet, error) {
	pager := c.client.NewListPager(resourceGroupName, nil)
	var ls []*armcompute.VirtualMachineScaleSet
	for pager.More() {
		res, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		ls = append(ls, res.VirtualMachineScaleSetListResult.Value...)
	}
	return ls, nil
}

// Get will fetch a vmss.
func (c VmssClient) Get(ctx context.Context, resourceGroupName, name string, expander *armcompute.ExpandTypesForGetVMScaleSets) (*armcompute.VirtualMachineScaleSet, error) {
	vmo, err := c.client.Get(ctx, resourceGroupName, name, &armcompute.VirtualMachineScaleSetsClientGetOptions{
		Expand: expander,
	})
	if err != nil {
		if IsAzureAPINotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}
	return &vmo.VirtualMachineScaleSet, nil
}

// Create will create a vmss.
func (c VmssClient) CreateOrUpdate(ctx context.Context, resourceGroupName, name string, properties armcompute.VirtualMachineScaleSet) (*armcompute.VirtualMachineScaleSet, error) {
	future, err := c.client.BeginCreateOrUpdate(ctx, resourceGroupName, name, properties, nil)
	if err != nil {
		return nil, err
	}
	res, err := future.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &res.VirtualMachineScaleSet, nil
}

// Delete will delete a vmss.
func (c VmssClient) Delete(ctx context.Context, resourceGroupName, name string, forceDeletion *bool) error {
	future, err := c.client.BeginDelete(ctx, resourceGroupName, name, &armcompute.VirtualMachineScaleSetsClientBeginDeleteOptions{
		ForceDeletion: forceDeletion,
	})
	if err != nil {
		return FilterNotFoundError(err)
	}
	_, err = future.PollUntilDone(ctx, nil)
	return err
}
