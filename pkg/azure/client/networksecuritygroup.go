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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-05-01/network"
)

// CreateOrUpdate indicates an expected call of Network Security Group CreateOrUpdate.
func (c NetworkSecurityGroupClient) CreateOrUpdate(ctx context.Context, resourceGroupName, name string, parameters network.SecurityGroup) (*network.SecurityGroup, error) {
	future, err := c.client.CreateOrUpdate(ctx, resourceGroupName, name, parameters)
	if err != nil {
		return nil, err
	}
	if err := future.WaitForCompletionRef(ctx, c.client.Client); err != nil {
		return nil, err
	}
	nsg, err := future.Result(c.client)
	if err != nil {
		return nil, err
	}
	return &nsg, nil
}

// Get will fetch a network security group.
func (c NetworkSecurityGroupClient) Get(ctx context.Context, resourceGroupName string, networkSecurityGroupName, name string) (*network.SecurityGroup, error) {
	nsg, err := c.client.Get(ctx, resourceGroupName, networkSecurityGroupName, name)
	if err != nil {
		return nil, err
	}
	return &nsg, nil
}

func (c NetworkSecurityGroupClient) Delete(ctx context.Context, resourceGroupName, name string) error {
	future, err := c.client.Delete(ctx, resourceGroupName, name)
	if err != nil {
		return err
	}
	if err := future.WaitForCompletionRef(ctx, c.client.Client); err != nil {
		return err
	}
	_, err = future.Result(c.client)
	return err
}

// Get will get a Security rule.
func (c SecurityRulesClient) Get(ctx context.Context, resourceGroupName string, networkSecurityGroupName string, name string) (*network.SecurityRule, error) {
	rules, err := c.client.Get(ctx, resourceGroupName, networkSecurityGroupName, name)
	if err != nil {
		return nil, err
	}
	return &rules, nil
}
