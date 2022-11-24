package client

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

func NewAvailabilitySetClient(auth internal.ClientAuth) (*AvailabilitySetClient, error) {
	cred, err := auth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}
	client, err := armcompute.NewAvailabilitySetsClient(auth.SubscriptionID, cred, nil)
	return &AvailabilitySetClient{client}, err
}

func (c AvailabilitySetClient) CreateOrUpdate(ctx context.Context, resourceGroupName, availabilitySetName string, parameters armcompute.AvailabilitySet) (res armcompute.AvailabilitySetsClientCreateOrUpdateResponse, err error) {
	res, err = c.client.CreateOrUpdate(ctx, resourceGroupName, availabilitySetName, parameters, nil)
	if err != nil {
		return res, fmt.Errorf("cannot create availability set: %v", err)
	}
	return res, nil
}

func (c AvailabilitySetClient) Get(ctx context.Context, resourceGroupName, availabilitySetName string) (res armcompute.AvailabilitySetsClientGetResponse, err error) {
	res, err = c.client.Get(ctx, resourceGroupName, availabilitySetName, nil)
	if err != nil {
		if IsAzureAPINotFoundError(err) {
			return res, nil
		}
	}
	return res, nil
}

func (c AvailabilitySetClient) Delete(ctx context.Context, resourceGroupName, availabilitySetName string) (armcompute.AvailabilitySetsClientDeleteResponse, error) {
	return c.client.Delete(ctx, resourceGroupName, availabilitySetName, nil)
}
