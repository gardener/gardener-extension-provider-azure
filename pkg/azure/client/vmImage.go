// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"errors"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-03-01/compute"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

var _ VirtualMachineImages = &VirtualMachineImageClient{}

// VirtualMachineImageClient is an implementation of Virtual Machine Image for a Virtual Machine Image k8sClient.
type VirtualMachineImageClient struct {
	client compute.VirtualMachineImagesClient
}

// NewVirtualMachineImagesClient creates a new VirtualMachineImagesClient client.
func NewVirtualMachineImagesClient(auth internal.ClientAuth, opts *policy.ClientOptions) (*VirtualMachineImageClient, error) {
	var cloudConfiguration cloud.Configuration

	if opts == nil {
		cloudConfiguration = cloud.AzurePublic
	} else {
		cloudConfiguration = opts.Cloud
	}

	var resourceManagerEndpoint string
	activeDirectoryEndpoint := cloudConfiguration.ActiveDirectoryAuthorityHost
	if c, ok := cloudConfiguration.Services[cloud.ResourceManager]; ok {
		resourceManagerEndpoint = c.Endpoint
	} else {
		return nil, errors.New("unable to determine ResourceManager endpoint from given cloud configuration")
	}

	client := compute.NewVirtualMachineImagesClientWithBaseURI(resourceManagerEndpoint, auth.SubscriptionID)
	authorizer, err := getAuthorizer(auth.TenantID, auth.ClientID, auth.ClientSecret, activeDirectoryEndpoint, resourceManagerEndpoint)
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
