package client

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

var _ ManagementPolicies = &ManagementPoliciesClient{}

// ManagementPoliciesClient is the necessary client used to add lifecycle policies to storage accounts
type ManagementPoliciesClient struct {
	client *armstorage.ManagementPoliciesClient
}

// NewManagementPoliciesClient creates a management policy client. This client is used to
// manage lifecycle policies on storage accounts. Returns the client and the error.
func NewManagementPoliciesClient(auth *ClientAuth, tc azcore.TokenCredential, opts *policy.ClientOptions) (*ManagementPoliciesClient, error) {
	client, err := armstorage.NewManagementPoliciesClient(auth.SubscriptionID, tc, opts)
	return &ManagementPoliciesClient{client}, err
}

// CreateOrUpdate adds a lifecycle policy on the storage account <storageAccount> in the resource group <resourceGroup> to delete blobs
// with the the tag `azure.BlobMarkedForDeletionTagKey: true` <daysAfterCreation> days after creation of the blob.
func (c *ManagementPoliciesClient) CreateOrUpdate(ctx context.Context, resourceGroup, storageAccount string, daysAfterCreation int) error {
	_, err := c.client.CreateOrUpdate(ctx, resourceGroup, storageAccount, armstorage.ManagementPolicyNameDefault, armstorage.ManagementPolicy{
		Properties: &armstorage.ManagementPolicyProperties{
			Policy: &armstorage.ManagementPolicySchema{
				Rules: []*armstorage.ManagementPolicyRule{
					{
						Name: ptr.To(string(azure.BlobDeletionLifecyclePolicyName)),
						Type: ptr.To(armstorage.RuleTypeLifecycle),
						Definition: &armstorage.ManagementPolicyDefinition{
							Actions: &armstorage.ManagementPolicyAction{
								BaseBlob: &armstorage.ManagementPolicyBaseBlob{
									Delete: &armstorage.DateAfterModification{
										DaysAfterCreationGreaterThan: ptr.To(float32(daysAfterCreation)),
									},
								},
							},
							Filters: &armstorage.ManagementPolicyFilter{
								// "blockBlob" mentioned in the SDK, they do not expose a constant unfortunately
								BlobTypes:   []*string{ptr.To("blockBlob")},
								PrefixMatch: []*string{ptr.To("")},
								BlobIndexMatch: []*armstorage.TagFilter{
									{
										Name: ptr.To(string(azure.BlobMarkedForDeletionTagKey)),
										// "==" mentioned in the SDK, they do not expose a constant unfortunately
										Op:    ptr.To("=="),
										Value: ptr.To("true"),
									},
								},
							},
						},
						Enabled: ptr.To(true),
					},
				},
			},
		},
	}, nil)

	return err
}
