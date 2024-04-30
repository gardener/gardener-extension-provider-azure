// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"
	"net/url"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

var _ BlobStorage = &BlobStorageClient{}

// BlobStorageClient is an implementation of Storage for a blob storage k8sClient.
type BlobStorageClient struct {
	// serviceURL *azblob.ServiceURL
	client *azblob.Client
}

// newStorageClient creates a client for an Azure Blob storage by reading auth information from secret reference. Requires passing the storage domain (formerly
// blobstorage host name) to determine the endpoint to build the service url for.
func newStorageClient(ctx context.Context, client client.Client, secretRef *corev1.SecretReference, storageDomain string) (*BlobStorageClient, error) {
	secret, err := extensionscontroller.GetSecretByReference(ctx, client, secretRef)
	if err != nil {
		return nil, err
	}
	storageAccountName, ok := secret.Data[azure.StorageAccount]
	if !ok {
		return nil, fmt.Errorf("secret %s/%s doesn't have a storage account", secret.Namespace, secret.Name)
	}

	storageAccountKey, ok := secret.Data[azure.StorageKey]
	if !ok {
		return nil, fmt.Errorf("secret %s/%s doesn't have a storage key", secret.Namespace, secret.Name)
	}

	credentials, err := azblob.NewSharedKeyCredential(string(storageAccountName), string(storageAccountKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create shared key credentials: %v", err)
	}

	storageAccountURL, err := url.Parse(fmt.Sprintf("https://%s.%s", storageAccountName, storageDomain))
	if err != nil {
		return nil, fmt.Errorf("failed to parse service url: %v", err)
	}

	blobclient, err := azblob.NewClientWithSharedKeyCredential(storageAccountURL.String(), credentials, nil)
	return &BlobStorageClient{blobclient}, err

}

// DeleteObjectsWithPrefix deletes the blob objects with the specific <prefix> from <container>.
// If it does not exist, no error is returned.
func (c *BlobStorageClient) DeleteObjectsWithPrefix(ctx context.Context, container, prefix string) error {
	pager := c.client.NewListBlobsFlatPager(container, &azblob.ListBlobsFlatOptions{Prefix: ptr.To(prefix)})
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return err
		}
		for _, blob := range page.Segment.BlobItems {
			if err := c.deleteBlobIfExists(ctx, container, *blob.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

// deleteBlobIfExists deletes the azure blob with name <blobName> from <container>.
// If it does not exist,no error is returned.
func (c *BlobStorageClient) deleteBlobIfExists(ctx context.Context, container, blobName string) error {
	_, err := c.client.DeleteBlob(ctx, container, blobName, &blob.DeleteOptions{
		DeleteSnapshots: ptr.To(azblob.DeleteSnapshotsOptionTypeInclude),
	})
	if err == nil || bloberror.HasCode(err, bloberror.BlobNotFound) {
		return nil
	}
	return err
}

// CreateContainerIfNotExists creates the azure blob container with name <container>.
// If it already exist,no error is returned.
func (c *BlobStorageClient) CreateContainerIfNotExists(ctx context.Context, container string) error {
	_, err := c.client.CreateContainer(ctx, container, nil)
	if err == nil || bloberror.HasCode(err, bloberror.ContainerAlreadyExists) {
		return nil
	}
	return err
}

// DeleteContainerIfExists deletes the azure blob container with name <container>.
// If it does not exist, no error is returned.
func (c *BlobStorageClient) DeleteContainerIfExists(ctx context.Context, container string) error {
	_, err := c.client.DeleteContainer(ctx, container, nil)
	if err == nil || bloberror.HasCode(err, bloberror.ContainerBeingDeleted, bloberror.ContainerNotFound) {
		return nil
	}
	return err
}
