package client

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
)

func (c SecurityGroupClient) CreateOrUpdate(ctx context.Context, resourceGroupName, securityGroupName string, parameters armnetwork.SecurityGroup) (err error) {
	poller, err := c.client.BeginCreateOrUpdate(ctx, resourceGroupName, securityGroupName, parameters, nil)
	if err != nil {
		return fmt.Errorf("cannot create security group: %v", err)
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

func (c SecurityGroupClient) Delete(ctx context.Context, resourceGroupName, name string) (err error) {
	poller, err := c.client.BeginDelete(ctx, resourceGroupName, name, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

func (c SecurityGroupClient) Get(ctx context.Context, resourceGroupName string, name string) (armnetwork.SecurityGroupsClientGetResponse, error) {
	return c.client.Get(ctx, resourceGroupName, name, nil)
}
