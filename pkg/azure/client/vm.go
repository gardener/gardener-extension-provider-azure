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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

// NewVMClient creates a new VM client
func NewVMClient(auth internal.ClientAuth) (VirtualMachine, error) {
	cred, err := auth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}
	client, err := armcompute.NewVirtualMachinesClient(auth.SubscriptionID, cred, nil)
	return &VirtualMachinesClient{client}, err
}

// Get will get virtual machines in a resource group.
func (c VirtualMachinesClient) Get(ctx context.Context, resourceGroupName string, name string, expander *armcompute.InstanceViewTypes) (*armcompute.VirtualMachine, error) {
	vm, err := c.client.Get(ctx, resourceGroupName, name, &armcompute.VirtualMachinesClientGetOptions{
		Expand: expander,
	})
	if err != nil {
		return nil, err
	}
	return &vm.VirtualMachine, nil
}

// Create will Create a virtual machine.
func (c VirtualMachinesClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, name string, parameters armcompute.VirtualMachine) (*armcompute.VirtualMachine, error) {
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
func (c VirtualMachinesClient) Delete(ctx context.Context, resourceGroupName, name string, forceDeletion *bool) error {
	future, err := c.client.BeginDelete(ctx, resourceGroupName, name, &armcompute.VirtualMachinesClientBeginDeleteOptions{
		ForceDeletion: forceDeletion,
	})
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
