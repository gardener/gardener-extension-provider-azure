package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

// NewResourceGroupsClient creates a new ResourceGroupClient
func NewResourceGroupsClient(auth internal.ClientAuth) (*ResourceGroupClient, error) {
	cred, err := auth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}
	client, err := armresources.NewResourceGroupsClient(auth.SubscriptionID, cred, nil)
	return &ResourceGroupClient{client}, err
}

// Get gets a resource group
func (c ResourceGroupClient) Get(ctx context.Context, resourceGroupName string) (*armresources.ResourceGroup, error) {
	res, err := c.client.Get(ctx, resourceGroupName, nil)
	if err != nil {
		return nil, err
	}
	return &res.ResourceGroup, err
}

// CreateOrUpdate creates or updates a resource group
func (c ResourceGroupClient) CreateOrUpdate(ctx context.Context, resourceGroupName, location string) error {
	_, err := c.client.CreateOrUpdate(
		ctx,
		resourceGroupName,
		armresources.ResourceGroup{
			Location: to.Ptr(location),
		},
		nil)
	return err
}

// DeleteIfExists deletes a resource group if it exists.
func (c ResourceGroupClient) DeleteIfExists(ctx context.Context, resourceGroupName string) error {
	resourceGroupResp, err := c.client.BeginDelete(
		ctx,
		resourceGroupName,
		nil)
	if err != nil {
		if IsAzureAPINotFoundError(err) {
			return nil
		} else {
			return err
		}
	}
	_, err = resourceGroupResp.PollUntilDone(ctx, nil)
	if IsAzureAPINotFoundError(err) {
		return nil
	} // ignore if resource group is already deleted
	return err
}

// IsExisting checks if a resource group exists
func (c ResourceGroupClient) IsExisting(ctx context.Context, resourceGroupName string) (bool, error) {
	res, err := c.client.CheckExistence(ctx, resourceGroupName, nil)
	if err != nil {
		return false, err
	}
	return res.Success, err
}
