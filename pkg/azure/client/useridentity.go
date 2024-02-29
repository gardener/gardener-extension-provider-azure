// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/msi/mgmt/2018-11-30/msi"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	azureRest "github.com/Azure/go-autorest/autorest/azure"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

var _ ManagedUserIdentity = &ManagedUserIdentityClient{}

// ManagedUserIdentityClient is an implementation of ManagedUserIdentity for a managed user identity k8sClient.
type ManagedUserIdentityClient struct {
	client msi.UserAssignedIdentitiesClient
}

// NewManagedUserIdentityClient creates a new ManagedUserIdentityClient
func NewManagedUserIdentityClient(auth internal.ClientAuth) (*ManagedUserIdentityClient, error) {
	msiClient := msi.NewUserAssignedIdentitiesClient(auth.SubscriptionID)
	authorizer, err := getAuthorizer(auth.TenantID, auth.ClientID, auth.ClientSecret)
	msiClient.Authorizer = authorizer
	return &ManagedUserIdentityClient{msiClient}, err
}

// Get returns a Managed User Identity by name.
func (m *ManagedUserIdentityClient) Get(ctx context.Context, resourceGroup, id string) (*msi.Identity, error) {
	res, err := m.client.Get(ctx, resourceGroup, id)
	if err != nil {
		return nil, FilterNotFoundError(err)
	}
	return &res, nil
}

func getAuthorizer(tenantId, clientId, clientSecret string) (autorest.Authorizer, error) {
	oauthConfig, err := adal.NewOAuthConfig(azureRest.PublicCloud.ActiveDirectoryEndpoint, tenantId)
	if err != nil {
		return nil, err
	}
	spToken, err := adal.NewServicePrincipalToken(*oauthConfig, clientId, clientSecret, azureRest.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		return nil, err
	}
	return autorest.NewBearerAuthorizer(spToken), nil
}
