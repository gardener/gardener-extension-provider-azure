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

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
)

// Get gets a resource group. If the requested resource group not exists nil will be returned.
func (c GroupClient) Get(ctx context.Context, resourceGroupName string) (*resources.Group, error) {
	resourceGroup, err := c.client.Get(ctx, resourceGroupName)
	if err != nil {
		if IsAzureAPINotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}

	return &resourceGroup, nil
}

// CreateOrUpdate creates a resource group or update an existing resource group.
func (c GroupClient) CreateOrUpdate(ctx context.Context, resourceGroupName, region string) error {
	if _, err := c.client.CreateOrUpdate(ctx, resourceGroupName, resources.Group{
		Location: &region,
	}); err != nil {
		return err
	}
	return nil
}

// DeleteIfExits deletes a resource group if it exits.
func (c GroupClient) DeleteIfExits(ctx context.Context, resourceGroupName string) error {
	_, err := c.client.Delete(ctx, resourceGroupName)
	if err != nil && IsAzureAPINotFoundError(err) {
		return nil
	}
	return err
}
