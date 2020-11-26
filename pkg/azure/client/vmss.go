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
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
)

// List will list vmss in a resource group.
func (c VmssClient) List(ctx context.Context, resourceGroupName string) ([]compute.VirtualMachineScaleSet, error) {
	pages, err := c.client.List(ctx, resourceGroupName)
	if err != nil {
		return nil, err
	}

	var vmoList []compute.VirtualMachineScaleSet
	for pages.NotDone() {
		vmoList = append(vmoList, pages.Values()...)
		if err := pages.NextWithContext(ctx); err != nil {
			return nil, err
		}
	}

	return vmoList, nil
}

// Get will fetch a vmss.
func (c VmssClient) Get(ctx context.Context, resourceGroupName, name string) (*compute.VirtualMachineScaleSet, error) {
	vmo, err := c.client.Get(ctx, resourceGroupName, name)
	if err != nil {
		if IsAzureAPINotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}
	return &vmo, nil
}

// Create will create a vmss.
func (c VmssClient) Create(ctx context.Context, resourceGroupName, name string, properties *compute.VirtualMachineScaleSet) (*compute.VirtualMachineScaleSet, error) {
	future, err := c.client.CreateOrUpdate(ctx, resourceGroupName, name, *properties)
	if err != nil {
		return nil, err
	}
	if err := future.WaitForCompletionRef(ctx, c.client.Client); err != nil {
		return nil, err
	}
	vmo, err := future.Result(c.client)
	if err != nil {
		return nil, err
	}
	return &vmo, nil
}

// Delete will delete a vmss.
func (c VmssClient) Delete(ctx context.Context, resourceGroupName, name string) error {
	future, err := c.client.Delete(ctx, resourceGroupName, name)
	if err != nil {
		return err
	}
	if err := future.WaitForCompletionRef(ctx, c.client.Client); err != nil {
		return err
	}
	result, err := future.Result(c.client)
	if err != nil {
		return err
	}
	if result.StatusCode == http.StatusOK || result.StatusCode == http.StatusAccepted || result.StatusCode == http.StatusNoContent {
		return nil
	}
	return fmt.Errorf("deletion of vmss %s failed. statuscode=%d", name, result.StatusCode)
}
