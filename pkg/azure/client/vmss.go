// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
)

var _ Vmss = &VmssClient{}

// VmssClient is an implementation of Vmss for a virtual machine scale set k8sClient.
type VmssClient struct {
	client *armcompute.VirtualMachineScaleSetsClient
}

// NewVmssClient creates a new VmssClient
func NewVmssClient(auth ClientAuth, tc azcore.TokenCredential, opts *arm.ClientOptions) (Vmss, error) {
	client, err := armcompute.NewVirtualMachineScaleSetsClient(auth.SubscriptionID, tc, opts)
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
		ls = append(ls, res.Value...)
	}
	return ls, nil
}

// Get will fetch a vmss.
func (c VmssClient) Get(ctx context.Context, resourceGroupName, name string, expander *armcompute.ExpandTypesForGetVMScaleSets) (*armcompute.VirtualMachineScaleSet, error) {
	vmo, err := c.client.Get(ctx, resourceGroupName, name, &armcompute.VirtualMachineScaleSetsClientGetOptions{
		Expand: expander,
	})
	if err != nil {
		return nil, FilterNotFoundError(err)
	}
	return &vmo.VirtualMachineScaleSet, nil
}

// CreateOrUpdate will create a vmss or update an existing one.
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
