package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

// TODO move to group.go?

func CreateOrUpdateResourceGroup(ctx context.Context, resourceGroupName, location string, clientAuth internal.ClientAuth) (*armresources.ResourceGroup, error) {
	cred, err := clientAuth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}
	resourceGroupClient, err := armresources.NewResourceGroupsClient(clientAuth.SubscriptionID, cred, nil)
	if err != nil {
		return nil, err
	}

	resourceGroupResp, err := resourceGroupClient.CreateOrUpdate(
		ctx,
		resourceGroupName,
		armresources.ResourceGroup{
			Location: to.Ptr(location),
		},
		nil)
	if err != nil {
		return nil, err
	}
	return &resourceGroupResp.ResourceGroup, nil
}

// sequential delete
func DeleteResourceGroup(ctx context.Context, resourceGroupName, location, subsriptionID string, cred azcore.TokenCredential) error {
	resourceGroupClient, err := armresources.NewResourceGroupsClient(subsriptionID, cred, nil)
	if err != nil {
		return err
	}

	resourceGroupResp, err := resourceGroupClient.BeginDelete(
		ctx,
		resourceGroupName,
		nil)
	if err != nil {
		return err
	}
	_, err = resourceGroupResp.PollUntilDone(ctx, nil)
	return err
}
