// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

var _ ManagedUserIdentity = &ManagedUserIdentityClient{}

// ManagedUserIdentityClient is an implementation of ManagedUserIdentity for a managed user identity k8sClient.
type ManagedUserIdentityClient struct {
	client *armmsi.UserAssignedIdentitiesClient
}

// NewManagedUserIdentityClient creates a new ManagedUserIdentityClient
func NewManagedUserIdentityClient(auth *internal.ClientAuth, tc azcore.TokenCredential, opts *policy.ClientOptions) (*ManagedUserIdentityClient, error) {
	client, err := armmsi.NewUserAssignedIdentitiesClient(auth.SubscriptionID, tc, opts)
	return &ManagedUserIdentityClient{client}, err
}

// Get returns a Managed User Identity by name.
func (m *ManagedUserIdentityClient) Get(ctx context.Context, resourceGroup, id string) (*armmsi.UserAssignedIdentitiesClientGetResponse, error) {
	res, err := m.client.Get(ctx, resourceGroup, id, nil)
	if err != nil {
		return nil, FilterNotFoundError(err)
	}
	return &res, nil
}
