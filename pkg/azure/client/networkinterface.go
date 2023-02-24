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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

// NewNetworkInterfaceClient creates a new NetworkInterfaceClient
func NewNetworkInterfaceClient(auth internal.ClientAuth) (*NetworkInterfaceClient, error) {
	cred, err := auth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}
	client, err := armnetwork.NewInterfacesClient(auth.SubscriptionID, cred, nil)
	return &NetworkInterfaceClient{client}, err
}

// CreateOrUpdate indicates an expected call of Network interface CreateOrUpdate.
func (c NetworkInterfaceClient) CreateOrUpdate(ctx context.Context, resourceGroupName, name string, parameters armnetwork.Interface) (*armnetwork.Interface, error) {
	future, err := c.client.BeginCreateOrUpdate(ctx, resourceGroupName, name, parameters, nil)
	if err != nil {
		return nil, err
	}
	res, err := future.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &res.Interface, nil
}

// Get will get a Network interface.
func (c NetworkInterfaceClient) Get(ctx context.Context, resourceGroupName string, name string) (*armnetwork.Interface, error) {
	nic, err := c.client.Get(ctx, resourceGroupName, name, nil)
	if err != nil {
		return nil, err
	}
	return &nic.Interface, nil
}

// Delete will delete a Network interface.
func (c NetworkInterfaceClient) Delete(ctx context.Context, resourceGroupName, name string) error {
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
