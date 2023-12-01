// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-03-01/compute"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

var _ VirtualMachineImages = &VirtualMachineImageClient{}

// VirtualMachineImageClient is an implementation of Virtual Machine Image for a Virtual Machine Image k8sClient.
type VirtualMachineImageClient struct {
	client compute.VirtualMachineImagesClient
}

// NewVirtualMachineImagesClient creates a new VirtualMachineImagesClient client.
func NewVirtualMachineImagesClient(auth internal.ClientAuth) (*VirtualMachineImageClient, error) {
	client := compute.NewVirtualMachineImagesClient(auth.SubscriptionID)
	authorizer, err := getAuthorizer(auth.TenantID, auth.ClientID, auth.ClientSecret)
	client.Authorizer = authorizer
	return &VirtualMachineImageClient{client}, err
}

// ListSkus will a list of virtual machine image SKUs for the specified location, publisher, and offer.
func (c *VirtualMachineImageClient) ListSkus(ctx context.Context, location string, publisherName string, offer string) (*compute.ListVirtualMachineImageResource, error) {
	skus, err := c.client.ListSkus(ctx, location, publisherName, offer)
	if err != nil {
		return nil, err
	}
	return &skus, nil
}
