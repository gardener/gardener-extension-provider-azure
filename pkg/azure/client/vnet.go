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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

var _ VirtualNetwork = &VnetClient{}

// VnetClient is an implmenetation of VirtualNetwork for a virtual network k8sClient.
type VnetClient struct {
	client *armnetwork.VirtualNetworksClient
}

// NewVnetClient creates a new VnetClient.
func NewVnetClient(auth internal.ClientAuth, tc azcore.TokenCredential, opts *arm.ClientOptions) (*VnetClient, error) {
	client, err := armnetwork.NewVirtualNetworksClient(auth.SubscriptionID, tc, opts)
	return &VnetClient{client}, err
}

// CreateOrUpdate creates or updates a virtual network.
func (v *VnetClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, name string, parameters armnetwork.VirtualNetwork) (*armnetwork.VirtualNetwork, error) {
	poller, err := v.client.BeginCreateOrUpdate(ctx, resourceGroupName, name, parameters, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create virtual network: %v", err)
	}
	res, err := poller.PollUntilDone(ctx, nil)
	return &res.VirtualNetwork, err
}

// Delete a given an existing virtual network.
func (v *VnetClient) Delete(ctx context.Context, resourceGroup, vnetName string) (err error) {
	poller, err := v.client.BeginDelete(ctx, resourceGroup, vnetName, nil)
	if err != nil {
		return FilterNotFoundError(err)
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

// Get gets a given virtual network by name
func (v *VnetClient) Get(ctx context.Context, resourceGroupName, name string) (*armnetwork.VirtualNetwork, error) {
	res, err := v.client.Get(ctx, resourceGroupName, name, nil)
	if err != nil {
		return nil, FilterNotFoundError(err)
	}
	return &res.VirtualNetwork, err
}
