// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2019-04-01/storage"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewStorageClientAuthFromSubscriptionSecretRef retrieves the azure storage client auth from specified by the secret reference.
func NewStorageClientAuthFromSubscriptionSecretRef(ctx context.Context, c client.Client, secretRef *corev1.SecretReference, resourceGroupName, accountName, region string) (*StorageAuth, error) {
	// Reference : https://github.com/Azure-Samples/azure-sdk-for-go-samples/blob/master/storage/account.go
	clientAuth, err := internal.GetClientAuthData(ctx, c, *secretRef)
	if err != nil {
		return nil, err
	}

	groupsClient := resources.NewGroupsClient(clientAuth.SubscriptionID)
	clientCredConfig := auth.NewClientCredentialsConfig(clientAuth.ClientID, clientAuth.ClientSecret, clientAuth.TenantID)
	authorizer, err := clientCredConfig.Authorizer()
	if err != nil {
		return nil, err
	}
	groupsClient.Authorizer = authorizer
	if _, err := groupsClient.CreateOrUpdate(ctx, resourceGroupName, resources.Group{
		Location: &region,
	}); err != nil {
		return nil, err
	}

	storageAccountClient := storage.NewAccountsClient(clientAuth.SubscriptionID)
	storageAccountClient.Authorizer = authorizer
	future, err := storageAccountClient.Create(ctx, resourceGroupName, accountName, storage.AccountCreateParameters{
		Sku: &storage.Sku{
			Name: storage.StandardLRS,
		},
		Kind:     storage.BlobStorage,
		Location: &region,
		AccountPropertiesCreateParameters: &storage.AccountPropertiesCreateParameters{
			AccessTier:             storage.Cool,
			EnableHTTPSTrafficOnly: pointer.BoolPtr(true),
		},
	})
	if err != nil {
		return nil, err
	}

	if err := future.WaitForCompletionRef(ctx, storageAccountClient.Client); err != nil {
		return nil, err
	}

	keysResponse, err := storageAccountClient.ListKeys(ctx, resourceGroupName, accountName, storage.Kerb)
	if err != nil {
		return nil, err
	}

	key := (*keysResponse.Keys)[0]

	return &StorageAuth{
		StorageAccount: []byte(accountName),
		StorageKey:     []byte(*key.Value),
	}, nil
}

// DeleteResourceGroupFromSubscriptionSecretRef deletes the resource group using subscription details from secretRef .
func DeleteResourceGroupFromSubscriptionSecretRef(ctx context.Context, c client.Client, secretRef *corev1.SecretReference, resourceGroupName string) error {
	clientAuth, err := internal.GetClientAuthData(ctx, c, *secretRef)
	if err != nil {
		return err
	}

	groupsClient := resources.NewGroupsClient(clientAuth.SubscriptionID)
	clientCredConfig := auth.NewClientCredentialsConfig(clientAuth.ClientID, clientAuth.ClientSecret, clientAuth.TenantID)
	authorizer, err := clientCredConfig.Authorizer()
	if err != nil {
		return err
	}
	groupsClient.Authorizer = authorizer

	_, err = groupsClient.Delete(ctx, resourceGroupName)
	return err
}

// NewStorageClientFromSecretRef retrieves the azure client from specified by the secret reference.
func NewStorageClientFromSecretRef(ctx context.Context, c client.Client, secretRef *corev1.SecretReference) (*StorageClient, error) {
	secret, err := extensionscontroller.GetSecretByReference(ctx, c, secretRef)
	if err != nil {
		return nil, err
	}

	storageAuth, err := ReadStorageClientAuthDataFromSecret(secret)
	if err != nil {
		return nil, err
	}

	return NewStorageClientFromStorageAuth(storageAuth)
}

// ReadStorageClientAuthDataFromSecret reads the storage client auth details from the given secret.
func ReadStorageClientAuthDataFromSecret(secret *corev1.Secret) (*StorageAuth, error) {
	storageAccount, ok := secret.Data[azure.StorageAccount]
	if !ok {
		return nil, fmt.Errorf("secret %s/%s doesn't have a storage account", secret.Namespace, secret.Name)
	}

	storageKey, ok := secret.Data[azure.StorageKey]
	if !ok {
		return nil, fmt.Errorf("secret %s/%s doesn't have a storage key", secret.Namespace, secret.Name)
	}

	return &StorageAuth{
		StorageAccount: storageAccount,
		StorageKey:     storageKey,
	}, nil
}

// NewStorageClientFromStorageAuth create the storage client from storage auth.
func NewStorageClientFromStorageAuth(storageAuth *StorageAuth) (*StorageClient, error) {
	credentials, err := azblob.NewSharedKeyCredential(string(storageAuth.StorageAccount), string(storageAuth.StorageKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create shared key credentials: %v", err)
	}

	p := azblob.NewPipeline(credentials, azblob.PipelineOptions{
		Retry: azblob.RetryOptions{
			Policy: azblob.RetryPolicyExponential,
		},
	})

	u, err := url.Parse(fmt.Sprintf("https://%s.%s", storageAuth.StorageAccount, azure.AzureBlobStorageHostName))
	if err != nil {
		return nil, fmt.Errorf("failed to parse service url: %v", err)
	}

	serviceURL := azblob.NewServiceURL(*u, p)

	return &StorageClient{
		serviceURL: serviceURL,
	}, nil
}

// DeleteObjectsWithPrefix deletes the blob objects with the specific <prefix> from <container>. If it does not exist,
// no error is returned.
func (c *StorageClient) DeleteObjectsWithPrefix(ctx context.Context, container, prefix string) error {
	containerURL := c.serviceURL.NewContainerURL(container)
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

// deleteBlobIfExists deletes the azure blob with name <blobName> from <container>. If it does not exist,
// no error is returned.
func (c *StorageClient) deleteBlobIfExists(ctx context.Context, container, blobName string) error {
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

// CreateContainerIfNotExists creates the azure blob container with name <container>. If it already exist,
// no error is returned.
func (c *StorageClient) CreateContainerIfNotExists(ctx context.Context, container string) error {
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

// DeleteContainerIfExists deletes the azure blob container with name <container>. If it does not exist,
// no error is returned.
func (c *StorageClient) DeleteContainerIfExists(ctx context.Context, container string) error {
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
