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

// NewSecurityGroupClient creates a new SecurityGroupClient
func NewSecurityGroupClient(auth internal.ClientAuth) (*NetworkSecurityGroupClient, error) {
	cred, err := auth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}
	client, err := armnetwork.NewSecurityGroupsClient(auth.SubscriptionID, cred, nil)
	return &NetworkSecurityGroupClient{client}, err
}

// CreateOrUpdate indicates an expected call of Network Security Group CreateOrUpdate.
func (c NetworkSecurityGroupClient) CreateOrUpdate(ctx context.Context, resourceGroupName, name string, parameters armnetwork.SecurityGroup) (*armnetwork.SecurityGroup, error) {
	future, err := c.client.BeginCreateOrUpdate(ctx, resourceGroupName, name, parameters, nil)
	if err != nil {
		return nil, err
	}
	nsg, err := future.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &nsg.SecurityGroup, nil
}

// Get will fetch a network security group.
func (c NetworkSecurityGroupClient) Get(ctx context.Context, resourceGroupName string, networkSecurityGroupName string) (*armnetwork.SecurityGroup, error) {
	nsg, err := c.client.Get(ctx, resourceGroupName, networkSecurityGroupName, nil)
	if err != nil {
		return nil, err
	}
	return &nsg.SecurityGroup, nil
}

// Delete deletes a network security group.
func (c NetworkSecurityGroupClient) Delete(ctx context.Context, resourceGroupName, name string) error {
	future, err := c.client.BeginDelete(ctx, resourceGroupName, name, nil)
	if err != nil {
		return FilterNotFoundError(err)
	}
	if _, err := future.PollUntilDone(ctx, nil); err != nil {
		return err
	}
	return err
}
