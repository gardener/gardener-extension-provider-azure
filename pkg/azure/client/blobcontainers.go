package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

var _ BlobContainers = &BlobContainersClient{}

// BlobContainersClient is the necessary client used to create the backupbucket
type BlobContainersClient struct {
	client *armstorage.BlobContainersClient
}

func NewBlobContainersClient(auth *internal.ClientAuth, tc azcore.TokenCredential, opts *policy.ClientOptions) (*BlobContainersClient, error) {
	client, err := armstorage.NewBlobContainersClient(auth.SubscriptionID, tc, opts)
	return &BlobContainersClient{client: client}, err
}

func (c *BlobContainersClient) GetContainer(ctx context.Context, resourceGroupName, accountName, containerName string) (armstorage.BlobContainersClientGetResponse, error) {
	return c.client.Get(ctx, resourceGroupName, accountName, containerName, nil)
}

// CreateContainer creates a container with the passed name
func (c *BlobContainersClient) CreateContainer(ctx context.Context, resourceGroupName, accountName, containerName string) (armstorage.BlobContainersClientCreateResponse, error) {
	return c.client.Create(ctx, resourceGroupName, accountName, containerName, armstorage.BlobContainer{}, nil)
}

func (c *BlobContainersClient) GetImmutabilityPolicy(ctx context.Context, resourceGroupName, accountName, containerName string) (*int32, bool, error) {
	immutabilityPolicyResponse, err := c.client.GetImmutabilityPolicy(ctx, resourceGroupName, accountName, containerName, nil)
	if err != nil || immutabilityPolicyResponse.Properties == nil || immutabilityPolicyResponse.Properties.State == nil {
		return nil, false, err
	}
	return immutabilityPolicyResponse.Properties.ImmutabilityPeriodSinceCreationInDays, *immutabilityPolicyResponse.Properties.State == armstorage.ImmutabilityPolicyStateLocked, nil
}

func (c *BlobContainersClient) CreateOrUpdateImmutabilityPolicy(ctx context.Context, resourceGroupName, accountName, containerName string, immutabilityPeriodSinceCreationInDays *int32) error {
	_, err := c.client.CreateOrUpdateImmutabilityPolicy(ctx, resourceGroupName, accountName, containerName, &armstorage.BlobContainersClientCreateOrUpdateImmutabilityPolicyOptions{
		Parameters: &armstorage.ImmutabilityPolicy{
			Properties: &armstorage.ImmutabilityPolicyProperty{
				AllowProtectedAppendWrites:            ptr.To(false),
				AllowProtectedAppendWritesAll:         ptr.To(false),
				ImmutabilityPeriodSinceCreationInDays: immutabilityPeriodSinceCreationInDays,
			},
		},
	})
	return err
}

func (c *BlobContainersClient) ExtendImmutabilityPolicy(ctx context.Context, resourceGroupName, accountName, containerName string, immutabilityPeriodSinceCreationInDays *int32) error {
	c.client.ExtendImmutabilityPolicy(ctx, resourceGroupName, accountName, containerName, "", &armstorage.BlobContainersClientExtendImmutabilityPolicyOptions{
		Parameters: &armstorage.ImmutabilityPolicy{
			Properties: &armstorage.ImmutabilityPolicyProperty{
				AllowProtectedAppendWrites:            ptr.To(false),
				AllowProtectedAppendWritesAll:         ptr.To(false),
				ImmutabilityPeriodSinceCreationInDays: immutabilityPeriodSinceCreationInDays,
			},
		},
	})
	return nil
}

func (c *BlobContainersClient) DeleteImmutabilityPolicy(ctx context.Context, resourceGroupName, accountName, containerName string) error {
	_, err := c.client.DeleteImmutabilityPolicy(ctx, resourceGroupName, accountName, containerName, "", nil)
	return err
}

func (c *BlobContainersClient) LockImmutabilityPolicy(ctx context.Context, resourceGroupName, accountName, containerName string) error {
	_, err := c.client.LockImmutabilityPolicy(ctx, resourceGroupName, accountName, containerName, "", nil)
	return err
}

func (c *BlobContainersClient) DeleteContainer(ctx context.Context, resourceGroupName, accountName, containerName string) error {
	_, err := c.client.Delete(ctx, resourceGroupName, accountName, containerName, nil)
	return err
}
