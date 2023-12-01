// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
