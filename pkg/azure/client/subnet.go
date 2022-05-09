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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-05-01/network"
)

// Get will get a subnet in a given virtual network. If the requested subnet not exists nil will be returned.
func (c SubnetsClient) Get(ctx context.Context, resourceGroupName string, vnetName string, name string, expander string) (*network.Subnet, error) {
	subnet, err := c.client.Get(ctx, resourceGroupName, vnetName, name, expander)
	if err != nil {
		if IsAzureAPINotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}
	return &subnet, nil
}

// List lists all subnets of a given virtual network.
func (c SubnetsClient) List(ctx context.Context, resourceGroupName, vnetName string) ([]network.Subnet, error) {
	subnetPages, err := c.client.List(ctx, resourceGroupName, vnetName)
	if err != nil {
		return nil, err
	}

	subnetList := []network.Subnet{}
	for subnetPages.NotDone() {
		subnetList = append(subnetList, subnetPages.Values()...)
		if err := subnetPages.NextWithContext(ctx); err != nil {
			return nil, err
		}
	}

	return subnetList, nil
}

// Delete deletes a subnet in a given virtual network.
func (c SubnetsClient) Delete(ctx context.Context, resourceGroupName, vnetName, subnetName string) error {
	future, err := c.client.Delete(ctx, resourceGroupName, vnetName, subnetName)
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

	return fmt.Errorf("deletion of subnet %s in virtual network %s/%s failed. statuscode=%d", subnetName, resourceGroupName, vnetName, result.StatusCode)
}
