// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
