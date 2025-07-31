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

// VirtualMachinesClient is an implementation of Vm for a virtual machine k8sClient.
type VirtualMachinesClient struct {
	client *armcompute.VirtualMachinesClient
}

// NewVMClient creates a new VM client
func NewVMClient(auth ClientAuth, tc azcore.TokenCredential, opts *arm.ClientOptions) (*VirtualMachinesClient, error) {
	client, err := armcompute.NewVirtualMachinesClient(auth.SubscriptionID, tc, opts)
	return &VirtualMachinesClient{client}, err
}

// Get will get virtual machines in a resource group.
func (c *VirtualMachinesClient) Get(ctx context.Context, resourceGroupName string, resource string, opts *armcompute.InstanceViewTypes) (*armcompute.VirtualMachine, error) {
	var getOpts *armcompute.VirtualMachinesClientGetOptions
	if opts != nil {
		getOpts = &armcompute.VirtualMachinesClientGetOptions{
			Expand: opts,
		}
	}
	vm, err := c.client.Get(ctx, resourceGroupName, resource, getOpts)
	if err != nil {
		return nil, FilterNotFoundError(err)
	}
	return &vm.VirtualMachine, nil
}

// CreateOrUpdate will Create a virtual machine or update an existing one.
func (c *VirtualMachinesClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, name string, parameters armcompute.VirtualMachine) (*armcompute.VirtualMachine, error) {
	future, err := c.client.BeginCreateOrUpdate(ctx, resourceGroupName, name, parameters, nil)
	if err != nil {
		return nil, err
	}
	res, err := future.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &res.VirtualMachine, nil
}

// Delete will delete a virtual machine.
func (c *VirtualMachinesClient) Delete(ctx context.Context, resourceGroupName, name string, forceDeletion *bool) error {
	future, err := c.client.BeginDelete(ctx, resourceGroupName, name, &armcompute.VirtualMachinesClientBeginDeleteOptions{
		ForceDeletion: forceDeletion,
	})
	if err != nil {
		return FilterNotFoundError(err)
	}
	_, err = future.PollUntilDone(ctx, nil)
	return err
}
