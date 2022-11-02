package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

func NewResourceGroupsClient(auth internal.ClientAuth) (*ResourceGroupClient, error) {
	cred, err := auth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}
	client, err := armresources.NewResourceGroupsClient(auth.SubscriptionID, cred, nil)
	return &ResourceGroupClient{client}, err
}

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

func (c ResourceGroupClient) IsExisting(ctx context.Context, resourceGroupName string) (bool, error) {
	res, err := c.client.CheckExistence(ctx, resourceGroupName, nil)
	if err != nil {
		return false, err
	}
	return res.Success, err
}
