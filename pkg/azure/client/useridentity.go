package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/msi/mgmt/2018-11-30/msi"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	azureRest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

type ManagedUserIdentityClient struct {
	client msi.UserAssignedIdentitiesClient
}

func NewManagedUserIdentityClient(auth internal.ClientAuth) (*ManagedUserIdentityClient, error) {
	msiClient := msi.NewUserAssignedIdentitiesClient(auth.SubscriptionID)
	authorizer, err := getAuthorizer(auth.TenantID, auth.ClientID, auth.ClientSecret)
	msiClient.Authorizer = authorizer
	return &ManagedUserIdentityClient{msiClient}, err
}

func (m ManagedUserIdentityClient) Get(ctx context.Context, resourceGroup, id string) (msi.Identity, error) {
	return m.client.Get(ctx, resourceGroup, id)
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
