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

// NewBlobContainersClient creates a blob container client
func NewBlobContainersClient(auth *internal.ClientAuth, tc azcore.TokenCredential, opts *policy.ClientOptions) (*BlobContainersClient, error) {
	client, err := armstorage.NewBlobContainersClient(auth.SubscriptionID, tc, opts)
	return &BlobContainersClient{client: client}, err
}

// GetContainer gets the metadata of the container with the name <containerName>, in the storage account <accountName>, and resource group <resourceGroupName>.
func (c *BlobContainersClient) GetContainer(ctx context.Context, resourceGroupName, accountName, containerName string) (armstorage.BlobContainersClientGetResponse, error) {
	return c.client.Get(ctx, resourceGroupName, accountName, containerName, nil)
}

// CreateContainer creates a container with the name <containerName>, in the storage account <accountName>, and resource group <resourceGroupName>.
func (c *BlobContainersClient) CreateContainer(ctx context.Context, resourceGroupName, accountName, containerName string) (armstorage.BlobContainersClientCreateResponse, error) {
	return c.client.Create(ctx, resourceGroupName, accountName, containerName, armstorage.BlobContainer{}, nil)
}

// GetImmutabilityPolicy gets the immutability policy of the container with the name <containerName>, in the storage account <storageAccountName>, and resource group <resourceGroupName>.
func (c *BlobContainersClient) GetImmutabilityPolicy(ctx context.Context, resourceGroupName, accountName, containerName string) (*int32, bool, *string, error) {
	immutabilityPolicyResponse, err := c.client.GetImmutabilityPolicy(ctx, resourceGroupName, accountName, containerName, nil)
	if err != nil || immutabilityPolicyResponse.Properties == nil || immutabilityPolicyResponse.Properties.State == nil {
		return nil, false, nil, err
	}
	// return resp.Etag, not resp.ETag
	return immutabilityPolicyResponse.Properties.ImmutabilityPeriodSinceCreationInDays, *immutabilityPolicyResponse.Properties.State == armstorage.ImmutabilityPolicyStateLocked, immutabilityPolicyResponse.Etag, nil
}

// CreateOrUpdateImmutabilityPolicy creates, or updates the immutability policy set at the container level on the container with the name <containerName>, in the storage account <accountName>, and resource group <resourceGroupName>. This method can be called on containers without an immutability policy, or with an unlocked immutability policy.
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

// ExtendImmutabilityPolicy extends the locked immutability policy of the container with the name <containerName>, in storage account <accountName>, and resource group <resourceGroupName>. This method is to be called only on containers with locked immutability policies.
func (c *BlobContainersClient) ExtendImmutabilityPolicy(ctx context.Context, resourceGroupName, accountName, containerName string, immutabilityPeriodSinceCreationInDays *int32, etag *string) error {
	_, err := c.client.ExtendImmutabilityPolicy(ctx, resourceGroupName, accountName, containerName, *etag, &armstorage.BlobContainersClientExtendImmutabilityPolicyOptions{
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

// DeleteImmutabilityPolicy deletes the immutability policy of a container with the name <containerName>, in storage account <accountName>, and resource group <resourceGroupName>. This can only be called on containers with unlocked immutability policies.
func (c *BlobContainersClient) DeleteImmutabilityPolicy(ctx context.Context, resourceGroupName, accountName, containerName string, etag *string) error {
	_, err := c.client.DeleteImmutabilityPolicy(ctx, resourceGroupName, accountName, containerName, *etag, nil)
	return err
}

// LockImmutabilityPolicy locks the immutability policy of a container with the name <containerName>, in storage account <accountName>, and resource group <resourceGroupName>. This can only be called on containers with unlocked immutability policies.
func (c *BlobContainersClient) LockImmutabilityPolicy(ctx context.Context, resourceGroupName, accountName, containerName string, etag *string) error {
	_, err := c.client.LockImmutabilityPolicy(ctx, resourceGroupName, accountName, containerName, *etag, nil)
	return err
}

// DeleteContainer deletes the container with the name <containerName>, in storage account <accountName>, and resource group <resourceGroupName>.
// If the container that this method is being called on has an immutability policy (unlocked/locked), it will succeed if and only if the container is empty.
// If the container is not empty, all blobs are to be deleted after the retention period expires, and then this method can be called to delete container.
func (c *BlobContainersClient) DeleteContainer(ctx context.Context, resourceGroupName, accountName, containerName string) error {
	_, err := c.client.Delete(ctx, resourceGroupName, accountName, containerName, nil)
	return err
}
