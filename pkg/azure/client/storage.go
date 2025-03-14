// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	azureapi "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

var _ BlobStorage = &BlobStorageClient{}

// BlobStorageClient is an implementation of Storage for a blob storage k8sClient.
type BlobStorageClient struct {
	client *container.Client
}

// BlobStorageDomainFromCloudConfiguration returns the storage service domain given a known cloudConfiguration.
func BlobStorageDomainFromCloudConfiguration(cloudConfiguration *azureapi.CloudConfiguration) (string, error) {
	// Unfortunately the valid values for storage domains run by Microsoft do not seem to be part of any sdk module. They might be queryable from the cloud configuration,
	// but I also haven't been able to find a documented list of proper ServiceName values.
	// Furthermore, it seems there is still no unified way of specifying the cloud instance to connect to as the domain remains part of the storage account URL while
	// the new options _also_ allow configuring the cloud instance.
	switch {
	case cloudConfiguration == nil || strings.EqualFold(cloudConfiguration.Name, "AzurePublic"):
		return azure.AzureBlobStorageDomain, nil
	case strings.EqualFold(cloudConfiguration.Name, "AzureGovernment"):
		// Note: This differs from the one mentioned in the docs ("blob.core.govcloudapi.net") but should be the right one.
		// ref.: https://github.com/google/go-cloud/blob/be1b4aee38955e1b8cd1c46f8f47fb6f9d820a9b/blob/azureblob/azureblob.go#L162
		return azure.AzureUSGovBlobStorageDomain, nil
	case strings.EqualFold(cloudConfiguration.Name, "AzureChina"):
		// source: https://learn.microsoft.com/en-us/azure/china/resources-developer-guide#check-endpoints-in-azure
		return azure.AzureChinaBlobStorageDomain, nil
	}
	return "", fmt.Errorf("unknown cloud configuration name '%s'", cloudConfiguration.Name)
}

// NewBlobStorageClient creates a blob storage client for the container <containerName>.
func NewBlobStorageClient(_ context.Context, storageAccountName, storageAccountKey, storageDomain, containerName string) (*BlobStorageClient, error) {
	credentials, err := azblob.NewSharedKeyCredential(storageAccountName, storageAccountKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create shared key credentials: %v", err)
	}

	containerEndpointURL, err := url.Parse(fmt.Sprintf("https://%s.%s/%s", storageAccountName, storageDomain, containerName))
	if err != nil {
		return nil, fmt.Errorf("failed to parse service url: %v", err)
	}

	containerClient, err := container.NewClientWithSharedKeyCredential(containerEndpointURL.String(), credentials, nil)
	return &BlobStorageClient{containerClient}, err
}

// NewBlobStorageClientFromSecretRef creates a client for an Azure Blob storage by reading auth information from a secret reference.
func NewBlobStorageClientFromSecretRef(ctx context.Context, client client.Client, secretRef *corev1.SecretReference, containerName string) (*BlobStorageClient, error) {
	secret, err := extensionscontroller.GetSecretByReference(ctx, client, secretRef)
	if err != nil {
		return nil, err
	}

	return NewBlobStorageClientFromSecret(ctx, secret, containerName)
}

// NewBlobStorageClientFromSecret creates a client for an Azure Blob storage by reading auth information from a secret for the container <containerName>.
func NewBlobStorageClientFromSecret(ctx context.Context, secret *corev1.Secret, containerName string) (*BlobStorageClient, error) {
	storageAccountName, ok := secret.Data[azure.StorageAccount]
	if !ok {
		return nil, fmt.Errorf("secret %s/%s doesn't have a storage account", secret.Namespace, secret.Name)
	}

	storageAccountKey, ok := secret.Data[azure.StorageKey]
	if !ok {
		return nil, fmt.Errorf("secret %s/%s doesn't have a storage key", secret.Namespace, secret.Name)
	}

	storageDomain := azure.AzureBlobStorageDomain
	if v, ok := secret.Data[azure.StorageDomain]; ok {
		storageDomain = string(v)
	}

	return NewBlobStorageClient(ctx, string(storageAccountName), string(storageAccountKey), storageDomain, containerName)
}

// CleanupObjectsWithPrefix cleans up the blob objects with the specific <prefix> from <container>.
//
// If the <container> has no immutability, the objects are deleted.
//
// If the <container> has immutability:
//  1. If the object's immutability has expired, it is deleted.
//  2. If the object's immutability has not expired, the BlobMarkedForDeletionTagKey tag is added to perform a delayed delete of the object through the lifecycle policy set on the storage account.
//
// If blobs with <prefix> do not exist, no error is returned.
func (c *BlobStorageClient) CleanupObjectsWithPrefix(ctx context.Context, prefix string) error {
	pager := c.client.NewListBlobsFlatPager(&azblob.ListBlobsFlatOptions{Prefix: ptr.To(prefix)})
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return err
		}
		for _, blob := range page.Segment.BlobItems {
			if err := c.cleanupBlobIfExists(ctx, *blob.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

// cleanupBlobIfExists cleans up the azure blob with name <blobName>.
//
// If the <container> has no immutability, the objects are deleted.
//
// If the <container> has immutability:
//  1. If the object's immutability has expired, it is deleted.
//  2. If the object's immutability has not expired, the BlobMarkedForDeletionTagKey tag is added to perform a delayed delete of the object through the lifecycle policy set on the storage account.
//
// If the blob with <blobName> does not exist, no error is returned.
func (c *BlobStorageClient) cleanupBlobIfExists(ctx context.Context, blobName string) error {
	blobClient := c.client.NewBlockBlobClient(blobName)
	_, err := blobClient.Delete(ctx, &blob.DeleteOptions{
		DeleteSnapshots: ptr.To(azblob.DeleteSnapshotsOptionTypeInclude),
	})
	if err == nil || bloberror.HasCode(err, bloberror.BlobNotFound) {
		return nil
	}

	// Blob is immutable, set the tag that triggers cleanup through lifecycle policy
	if bloberror.HasCode(err, bloberror.BlobImmutableDueToPolicy) {
		tags := map[string]string{
			azure.BlobMarkedForDeletionTagKey: "true",
		}
		_, err = blobClient.SetTags(ctx, tags, nil)
	}
	return err
}
