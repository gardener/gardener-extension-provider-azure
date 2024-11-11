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

// CreateStorageAccount creates a storage account.
func (c *StorageAccountClient) CreateStorageAccount(ctx context.Context, resourceGroupName, storageAccountName, region string) error {
	poller, err := c.client.BeginCreate(ctx, resourceGroupName, storageAccountName, armstorage.AccountCreateParameters{
		Kind:     ptr.To(armstorage.KindStorageV2),
		Location: &region,
		SKU:      &armstorage.SKU{Name: ptr.To(armstorage.SKUNameStandardZRS)},
		Properties: &armstorage.AccountPropertiesCreateParameters{
			AccessTier:             ptr.To(armstorage.AccessTierCool),
			EnableHTTPSTrafficOnly: ptr.To(true),
			AllowBlobPublicAccess:  ptr.To(false),
			MinimumTLSVersion:      ptr.To(armstorage.MinimumTLSVersionTLS12),
		},
	}, nil)

	if err != nil {
		return err
	}

	_, err = poller.PollUntilDone(ctx, nil)

	return err
}

// ListStorageAccountKey lists the first key of a storage account.
func (c *StorageAccountClient) ListStorageAccountKey(ctx context.Context, resourceGroupName, storageAccountName string) (string, error) {
	keys, err := c.listStorageAccountKeys(ctx, resourceGroupName, storageAccountName)
	if err != nil {
		return "", err
	}
	return *keys[0].Value, nil
}

// ListStorageAccountKeys lists the keys of a storage account.
func (c *StorageAccountClient) listStorageAccountKeys(ctx context.Context, resourceGroupName, storageAccountName string) ([]*armstorage.AccountKey, error) {
	response, err := c.client.ListKeys(ctx, resourceGroupName, storageAccountName, &armstorage.AccountsClientListKeysOptions{
		// doc: "Specifies type of the key to be listed. Possible value is kerb.. Specifying any value will set the value to kerb."
		Expand: ptr.To("kerb"),
	})

	if err != nil {
		return nil, err
	}

	if len(response.Keys) < 1 {
		return nil, fmt.Errorf("no key found in storage account %s", storageAccountName)
	}

	return response.Keys, nil
}

// RotateKey performs a partial key rotation for the given storage account, using the property of Azure storage accounts always having exactly two keys.
// The existing storage keys are checked against the given one, and the one that does _not_ match it will be regenerated and its new value returned.
// This ensures the given key still being valid (until the next rotation) which ensures that there is time for the updated value to propagate.
func (c *StorageAccountClient) RotateKey(ctx context.Context, resourceGroupName, storageAccountName, storageAccountKey string) (string, error) {
	keys, err := c.listStorageAccountKeys(ctx, resourceGroupName, storageAccountName)
	if err != nil {
		return "", err
	}

	var keyToRotate *string
	switch {
	case ptr.Deref(keys[0].Value, "") == storageAccountKey:
		keyToRotate = keys[1].KeyName
	case ptr.Deref(keys[1].Value, "") == storageAccountKey:
		keyToRotate = keys[0].KeyName
	default:
		return "", fmt.Errorf("unable to find given key in storage account keys")
	}

	resp, err := c.client.RegenerateKey(
		ctx,
		resourceGroupName,
		storageAccountName,
		armstorage.AccountRegenerateKeyParameters{KeyName: keyToRotate},
		nil,
	)

	if err != nil {
		return "", err
	}

	// (re)Get index of regenerated key from response. This might be a bit paranoid.
	for _, k := range resp.Keys {
		if *k.KeyName == *keyToRotate {
			return *k.Value, nil
		}
	}

	return "", fmt.Errorf("error rotating storage account key '%v'", keyToRotate)

}
