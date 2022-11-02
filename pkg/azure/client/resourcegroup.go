package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

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

func (c ResourceGroupClient) Delete(ctx context.Context, resourceGroupName string) error {
	resourceGroupResp, err := c.client.BeginDelete(
		ctx,
		resourceGroupName,
		nil)
	if err != nil {
		return err
	}
	_, err = resourceGroupResp.PollUntilDone(ctx, nil)
	return err
}
