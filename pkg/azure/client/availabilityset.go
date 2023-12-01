// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

var _ AvailabilitySet = &AvailabilitySetClient{}

// AvailabilitySetClient is an implementation of AvailabilitySet for an availability set k8sClient.
type AvailabilitySetClient struct {
	client *armcompute.AvailabilitySetsClient
}

// NewAvailabilitySetClient creates a new AvailabilitySetClient.
func NewAvailabilitySetClient(auth internal.ClientAuth) (*AvailabilitySetClient, error) {
	cred, err := auth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}
	client, err := armcompute.NewAvailabilitySetsClient(auth.SubscriptionID, cred, nil)
	return &AvailabilitySetClient{client}, err
}

// CreateOrUpdate creates or updates a new availability set.
func (c *AvailabilitySetClient) CreateOrUpdate(ctx context.Context, resourceGroupName, availabilitySetName string, parameters armcompute.AvailabilitySet) (*armcompute.AvailabilitySet, error) {
	res, err := c.client.CreateOrUpdate(ctx, resourceGroupName, availabilitySetName, parameters, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create availability set: %v", err)
	}
	return &res.AvailabilitySet, nil
}

// Get returns the availability set for the given resource group and availability set name.
func (c *AvailabilitySetClient) Get(ctx context.Context, resourceGroupName, availabilitySetName string) (*armcompute.AvailabilitySet, error) {
	res, err := c.client.Get(ctx, resourceGroupName, availabilitySetName, nil)
	if err != nil {
		return nil, FilterNotFoundError(err)
	}
	return &res.AvailabilitySet, err
}

// Delete deletes the availability set for the given resource group and availability set name.
func (c *AvailabilitySetClient) Delete(ctx context.Context, resourceGroupName, availabilitySetName string) error {
	_, err := c.client.Delete(ctx, resourceGroupName, availabilitySetName, nil)
	return FilterNotFoundError(err)
}
