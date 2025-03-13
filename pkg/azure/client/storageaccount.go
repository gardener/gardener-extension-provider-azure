// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

var _ StorageAccount = &StorageAccountClient{}

// StorageAccountClient is an implementation of StorageAccount for storage account k8sClient.
type StorageAccountClient struct {
	client *armstorage.AccountsClient
}

// NewStorageAccountClient creates a new StorageAccountClient
func NewStorageAccountClient(auth *internal.ClientAuth, tc azcore.TokenCredential, opts *policy.ClientOptions) (*StorageAccountClient, error) {
	client, err := armstorage.NewAccountsClient(auth.SubscriptionID, tc, opts)
	return &StorageAccountClient{client}, err
}

// CreateOrUpdateStorageAccount creates a storage account.
func (c *StorageAccountClient) CreateOrUpdateStorageAccount(ctx context.Context, resourceGroupName, storageAccountName, region string, keyExpiration *int32) error {
	properties := armstorage.AccountPropertiesCreateParameters{
		AccessTier:             ptr.To(armstorage.AccessTierCool),
		EnableHTTPSTrafficOnly: ptr.To(true),
		AllowBlobPublicAccess:  ptr.To(false),
		MinimumTLSVersion:      ptr.To(armstorage.MinimumTLSVersionTLS12),
		KeyPolicy: &armstorage.KeyPolicy{
			KeyExpirationPeriodInDays: ptr.To(int32(0)),
		},
	}
	if keyExpiration != nil {
		properties.KeyPolicy = &armstorage.KeyPolicy{
			KeyExpirationPeriodInDays: keyExpiration,
		}
	}
	poller, err := c.client.BeginCreate(ctx, resourceGroupName, storageAccountName, armstorage.AccountCreateParameters{
		Kind:       ptr.To(armstorage.KindStorageV2),
		Location:   &region,
		SKU:        &armstorage.SKU{Name: ptr.To(armstorage.SKUNameStandardZRS)},
		Properties: &properties,
	}, nil)

	if err != nil {
		return err
	}

	_, err = poller.PollUntilDone(ctx, nil)

	return err
}

// ListStorageAccountKey lists the first key of a storage account.
func (c *StorageAccountClient) ListStorageAccountKey(ctx context.Context, resourceGroupName, storageAccountName string) (string, error) {
	response, err := c.ListStorageAccountKeys(ctx, resourceGroupName, storageAccountName)
	if err != nil {
		return "", err
	}

	if len(response) < 1 {
		return "", fmt.Errorf("no key found in storage account %s", storageAccountName)
	}

	firstKey := response[0]
	return *firstKey.Value, nil
}

// ListStorageAccountKeys lists all keys for the specified storage account.
func (c *StorageAccountClient) ListStorageAccountKeys(ctx context.Context, resourceGroupName, storageAccountName string) ([]*armstorage.AccountKey, error) {
	response, err := c.client.ListKeys(ctx, resourceGroupName, storageAccountName, &armstorage.AccountsClientListKeysOptions{})

	if err != nil {
		return nil, err
	}

	return response.Keys, nil
}

// RotateKey rotates the key with the given name and returns the updated key.
func (c *StorageAccountClient) RotateKey(ctx context.Context, resourceGroupName, storageAccountName, storageAccountKeyName string) (*armstorage.AccountKey, error) {
	resp, err := c.client.RegenerateKey(
		ctx,
		resourceGroupName,
		storageAccountName,
		armstorage.AccountRegenerateKeyParameters{KeyName: ptr.To(storageAccountKeyName)},
		nil,
	)

	if err != nil {
		return nil, err
	}
	for _, k := range resp.Keys {
		if ptr.Deref(k.KeyName, "") == storageAccountKeyName {
			return k, nil
		}
	}

	return nil, fmt.Errorf("error rotating storage account key '%v'", storageAccountKeyName)
}
