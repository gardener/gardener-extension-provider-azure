// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"context"
	"fmt"
	"net/url"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"

	"github.com/Azure/azure-storage-blob-go/azblob"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeleteObjectsWithPrefix deletes the blob objects with the specific <prefix> from <container>.
// If it does not exist, no error is returned.
func (c StorageClient) DeleteObjectsWithPrefix(ctx context.Context, container, prefix string) error {
	var containerURL = c.serviceURL.NewContainerURL(container)
	opts := azblob.ListBlobsSegmentOptions{
		Details: azblob.BlobListingDetails{
			Deleted: true,
		},
		Prefix: prefix,
	}

	for marker := (azblob.Marker{}); marker.NotDone(); {
		// Get a result segment starting with the blob indicated by the current Marker.
		listBlob, err := containerURL.ListBlobsFlatSegment(ctx, marker, opts)
		if err != nil {
			return fmt.Errorf("failed to list the blobs, error: %v", err)
		}
		marker = listBlob.NextMarker

		// Process the blobs returned in this result segment
		for _, blob := range listBlob.Segment.BlobItems {
			if err := c.deleteBlobIfExists(ctx, container, blob.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

// deleteBlobIfExists deletes the azure blob with name <blobName> from <container>.
// If it does not exist,no error is returned.
func (c StorageClient) deleteBlobIfExists(ctx context.Context, container, blobName string) error {
	blockBlobURL := c.serviceURL.NewContainerURL(container).NewBlockBlobURL(blobName)
	if _, err := blockBlobURL.Delete(ctx, azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{}); err != nil {
		if stgErr, ok := err.(azblob.StorageError); ok {
			switch stgErr.ServiceCode() {
			case azblob.ServiceCodeBlobNotFound:
				return nil
			}
		}
		return err
	}
	return nil
}

// CreateContainerIfNotExists creates the azure blob container with name <container>.
// If it already exist,no error is returned.
func (c StorageClient) CreateContainerIfNotExists(ctx context.Context, container string) error {
	containerURL := c.serviceURL.NewContainerURL(container)
	if _, err := containerURL.Create(ctx, nil, azblob.PublicAccessNone); err != nil {
		if stgErr, ok := err.(azblob.StorageError); ok {
			switch stgErr.ServiceCode() {
			case azblob.ServiceCodeContainerAlreadyExists:
				return nil
			}
		}
		return err
	}
	return nil
}

// DeleteContainerIfExists deletes the azure blob container with name <container>.
// If it does not exist, no error is returned.
func (c StorageClient) DeleteContainerIfExists(ctx context.Context, container string) error {
	containerURL := c.serviceURL.NewContainerURL(container)
	if _, err := containerURL.Delete(ctx, azblob.ContainerAccessConditions{}); err != nil {
		if stgErr, ok := err.(azblob.StorageError); ok {
			switch stgErr.ServiceCode() {
			case azblob.ServiceCodeContainerNotFound:
				return nil
			case azblob.ServiceCodeContainerBeingDeleted:
				return nil
			}
		}
		return err
	}
	return nil
}

// newStorageClient creates a client for an Azure Blob storage by reading auth information from secret reference.
func newStorageClient(ctx context.Context, client client.Client, secretRef *corev1.SecretReference) (*azblob.ServiceURL, error) {
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

	pipeline := azblob.NewPipeline(credentials, azblob.PipelineOptions{
		Retry: azblob.RetryOptions{
			Policy: azblob.RetryPolicyExponential,
		},
	})

	storageAccountURL, err := url.Parse(fmt.Sprintf("https://%s.%s", storageAccountName, azure.AzureBlobStorageHostName))
	if err != nil {
		return nil, fmt.Errorf("failed to parse service url: %v", err)
	}

	serviceURL := azblob.NewServiceURL(*storageAccountURL, pipeline)
	return &serviceURL, nil
}
