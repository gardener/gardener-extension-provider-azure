package client

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
)

func (c AvailabilitySetClient) CreateOrUpdate(ctx context.Context, resourceGroupName, availabilitySetName string, parameters armcompute.AvailabilitySet) error {
	_, err := c.client.CreateOrUpdate(ctx, resourceGroupName, availabilitySetName, parameters, nil)
	if err != nil {
		return fmt.Errorf("cannot create availability set: %v", err)
	}
	return err
}

func (c AvailabilitySetClient) Get(ctx context.Context, resourceGroupName, availabilitySetName string) (*armcompute.AvailabilitySet, error) {
	availabilitySet, err := c.client.Get(ctx, resourceGroupName, availabilitySetName, nil)
	if err != nil {
		if IsAzureAPINotFoundError(err) {
			return nil, nil
		}
	}
	return &availabilitySet.AvailabilitySet, nil
}

func (c AvailabilitySetClient) Delete(ctx context.Context, resourceGroupName, availabilitySetName string) error {
	_, err := c.client.Delete(ctx, resourceGroupName, availabilitySetName, nil)
	return err
}
