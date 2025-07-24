// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

var _ VirtualMachineImages = &VirtualMachineImageClient{}

// VirtualMachineImageClient is an implementation of Virtual Machine Image for a Virtual Machine Image k8sClient.
type VirtualMachineImageClient struct {
	client *armcompute.VirtualMachineImagesClient
}

// NewVirtualMachineImagesClient creates a new VirtualMachineImagesClient client.
func NewVirtualMachineImagesClient(auth *internal.ClientAuth, tc azcore.TokenCredential, opts *policy.ClientOptions) (*VirtualMachineImageClient, error) {
	client, err := armcompute.NewVirtualMachineImagesClient(auth.SubscriptionID, tc, opts)
	return &VirtualMachineImageClient{client}, err
}

// ListSkus will a list of virtual machine image SKUs for the specified location, publisher, and offer.
func (c *VirtualMachineImageClient) ListSkus(ctx context.Context, location string, publisherName string, offer string) (*armcompute.VirtualMachineImagesClientListSKUsResponse, error) {
	skus, err := c.client.ListSKUs(ctx, location, publisherName, offer, nil)
	if err != nil {
		return nil, err
	}
	return &skus, nil
}
