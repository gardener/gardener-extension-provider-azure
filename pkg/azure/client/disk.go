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
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-03-01/compute"
)

// Get will fetch a disk by given name in a given resource group.
func (c DisksClient) Get(ctx context.Context, resourceGroupName string, name string) (*compute.Disk, error) {
	disk, err := c.client.Get(ctx, resourceGroupName, name)
	if err != nil {
		return nil, err
	}
	return &disk, nil
}

// CreateOrUpdate will create or update a disk.
func (c DisksClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, diskName string, disk compute.Disk) (*compute.Disk, error) {
	future, err := c.client.CreateOrUpdate(ctx, resourceGroupName, diskName, disk)
	if err != nil {
		return nil, err
	}
	if err := future.WaitForCompletionRef(ctx, c.client.Client); err != nil {
		return nil, err
	}
	disk, err = future.Result(c.client)
	if err != nil {
		return nil, err
	}
	return &disk, nil
}

// Delete will delete a disk.
func (c DisksClient) Delete(ctx context.Context, resourceGroupName, name string) error {
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
	if result.StatusCode == http.StatusOK || result.StatusCode == http.StatusAccepted || result.StatusCode == http.StatusNoContent || result.StatusCode == http.StatusNotFound {
		return nil
	}
	return fmt.Errorf("deletion of disk %s failed. statuscode=%d", name, result.StatusCode)
}
