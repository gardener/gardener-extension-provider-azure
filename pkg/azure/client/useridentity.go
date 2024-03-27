// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/services/msi/mgmt/2018-11-30/msi"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/pkg/errors"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

var _ ManagedUserIdentity = &ManagedUserIdentityClient{}

// ManagedUserIdentityClient is an implementation of ManagedUserIdentity for a managed user identity k8sClient.
type ManagedUserIdentityClient struct {
	client msi.UserAssignedIdentitiesClient
}

// NewManagedUserIdentityClient creates a new ManagedUserIdentityClient
func NewManagedUserIdentityClient(auth internal.ClientAuth, opts *policy.ClientOptions) (*ManagedUserIdentityClient, error) {
	var cloudConfiguration cloud.Configuration

	if opts == nil {
		cloudConfiguration = cloud.AzurePublic
	} else {
		cloudConfiguration = opts.Cloud
	}

	var resourceManagerEndpoint string
	activeDirectoryEndpoint := cloudConfiguration.ActiveDirectoryAuthorityHost
	if c, ok := cloudConfiguration.Services[cloud.ResourceManager]; ok {
		resourceManagerEndpoint = c.Endpoint
	} else {
		return nil, errors.New("unable to determine ResourceManager endpoint from given cloud configuration")
	}

	msiClient := msi.NewUserAssignedIdentitiesClientWithBaseURI(resourceManagerEndpoint, auth.SubscriptionID)
	authorizer, err := getAuthorizer(auth.TenantID, auth.ClientID, auth.ClientSecret, activeDirectoryEndpoint, resourceManagerEndpoint)
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

func getAuthorizer(tenantId, clientId, clientSecret, activeDirectoryEndpoint, resourceManagerEndpoint string) (autorest.Authorizer, error) {

	oauthConfig, err := adal.NewOAuthConfig(activeDirectoryEndpoint, tenantId)
	if err != nil {
		return nil, err
	}
	spToken, err := adal.NewServicePrincipalToken(*oauthConfig, clientId, clientSecret, resourceManagerEndpoint)
	if err != nil {
		return nil, err
	}
	return autorest.NewBearerAuthorizer(spToken), nil
}
