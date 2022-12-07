package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

// NewNatGatewaysClient creates a new NatGateway client.
func NewNatGatewaysClient(auth internal.ClientAuth) (*NatGatewayClient, error) {
	cred, err := auth.GetAzClientCredentials()
	if err != nil {
		return nil, err
	}
	client, err := armnetwork.NewNatGatewaysClient(auth.SubscriptionID, cred, nil)
	return &NatGatewayClient{client}, err
}

// CreateOrUpdate creates or updates a NatGateway.
func (c NatGatewayClient) CreateOrUpdate(ctx context.Context, resourceGroupName, natGatewayName string, parameters armnetwork.NatGateway) (armnetwork.NatGatewaysClientCreateOrUpdateResponse, error) {

	poller, err := c.client.BeginCreateOrUpdate(ctx, resourceGroupName, natGatewayName, parameters, nil)
	if err != nil {
		return armnetwork.NatGatewaysClientCreateOrUpdateResponse{}, err
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	return resp, err
}

// Get returns a NatGateway by name or nil if it doesn't exis.
func (c NatGatewayClient) Get(ctx context.Context, resourceGroupName, natGatewayName string) (*armnetwork.NatGatewaysClientGetResponse, error) {
	natGateway, err := c.client.Get(ctx, resourceGroupName, natGatewayName, nil)
	if err != nil {
		if IsAzureAPINotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}
	return &natGateway, nil
}

// GetAll returns all NATGateways in the given resource group.
func (c NatGatewayClient) GetAll(ctx context.Context, resourceGroupName string) ([]*armnetwork.NatGateway, error) {
	pager := c.client.NewListPager(resourceGroupName, nil)
	var nats []*armnetwork.NatGateway
	for pager.More() {
		res, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		nats = append(nats, res.NatGatewayListResult.Value...)
	}
	return nats, nil
}

// Delete deletes the NatGateway with the given name.
func (c NatGatewayClient) Delete(ctx context.Context, resourceGroupName, natGatewayName string) error {
	poller, err := c.client.BeginDelete(ctx, resourceGroupName, natGatewayName, nil)
	if err != nil {
		return err
	}

	_, err = poller.PollUntilDone(ctx, nil)
	return err
}
