// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket_test

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	azclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	mockazureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client/mock"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/controller/backupbucket"
)

const (
	name              = "azure-backupbucket"
	namespace         = "shoot--foobar-az"
	storageAccountKey = "storage-account-key-for-azure"
)

var _ = Describe("Actuator", func() {
	var (
		ctx                           context.Context
		ctrl                          *gomock.Controller
		c                             *mockclient.MockClient
		sw                            *mockclient.MockStatusWriter
		mgr                           *mockmanager.MockManager
		azureClientFactory            *mockazureclient.MockFactory
		azureGroupClient              *mockazureclient.MockResourceGroup
		azureStorageAccountClient     *mockazureclient.MockStorageAccount
		azureBlobContainersClient     *mockazureclient.MockBlobContainers
		azureManagementPoliciesClient *mockazureclient.MockManagementPolicies
		a                             backupbucket.Actuator
		logger                        logr.Logger
		backupBucket                  *extensionsv1alpha1.BackupBucket
		defaultFactory                = DefaultAzureClientFactoryFunc
		storageAccountName            string
		resourceGroupName             string
		etag                          = "backupbucket-first-etag"
		etag2                         = "backupbucket-second-etag"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)
		sw = mockclient.NewMockStatusWriter(ctrl)
		mgr = mockmanager.NewMockManager(ctrl)
		azureClientFactory = mockazureclient.NewMockFactory(ctrl)
		azureGroupClient = mockazureclient.NewMockResourceGroup(ctrl)
		azureStorageAccountClient = mockazureclient.NewMockStorageAccount(ctrl)
		azureBlobContainersClient = mockazureclient.NewMockBlobContainers(ctrl)
		azureManagementPoliciesClient = mockazureclient.NewMockManagementPolicies(ctrl)

		c.EXPECT().Status().Return(sw).AnyTimes()

		DefaultAzureClientFactoryFunc = func(_ context.Context, _ client.Client, _ corev1.SecretReference, _ bool, _ ...azclient.AzureFactoryOption) (azclient.Factory, error) {
			return azureClientFactory, nil
		}

		ctx = context.TODO()
		logger = log.Log.WithName("test")

		mgr = mockmanager.NewMockManager(ctrl)
		mgr.EXPECT().GetClient().Return(c)

		a = NewActuator(mgr)

		backupBucket = &extensionsv1alpha1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: extensionsv1alpha1.BackupBucketSpec{
				SecretRef: corev1.SecretReference{
					Name:      name + "-secret",
					Namespace: namespace,
				},
				Region: "westeurope",
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: "azure",
				},
			},
		}

		storageAccountName = GenerateStorageAccountName(backupBucket.Name)
		resourceGroupName = backupBucket.Name
	})

	AfterEach(func() {
		DefaultAzureClientFactoryFunc = defaultFactory
		ctrl.Finish()
	})

	Describe("#Reconcile", func() {
		Context("client creation fails during reconiliation", func() {
			It("should error", func() {
				// resource group client is the first client created in the reconciliation
				azureClientFactory.EXPECT().Group().Return(azureGroupClient, fmt.Errorf("resource group client creation error test"))
				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).Should(HaveOccurred())
			})
		})

		Context("when the resource group and storage account do not exist", func() {
			It("should error if creating resource group fails", func() {
				// try creating the resource group
				azureClientFactory.EXPECT().Group().Return(azureGroupClient, nil)
				azureGroupClient.EXPECT().CreateOrUpdate(ctx, name, armresources.ResourceGroup{
					Location: to.Ptr(backupBucket.Spec.Region),
				}).Return(&armresources.ResourceGroup{}, fmt.Errorf("resource group creation error test"))

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).Should(HaveOccurred())
			})

			It("should error if creating storage account fails", func() {
				// create the resource group
				azureClientFactory.EXPECT().Group().Return(azureGroupClient, nil)
				azureGroupClient.EXPECT().CreateOrUpdate(ctx, name, armresources.ResourceGroup{
					Location: to.Ptr(backupBucket.Spec.Region),
				})

				// try creating storage account
				azureClientFactory.EXPECT().StorageAccount().Return(azureStorageAccountClient, nil)
				azureStorageAccountClient.EXPECT().CreateStorageAccount(ctx, name, storageAccountName, backupBucket.Spec.Region).Return(fmt.Errorf("storage account creation error test"))

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).Should(HaveOccurred())
			})
		})

		Context("set lifecycle policy on the storage account during each reconciliation", func() {
			It("should error if adding the lifecycle policy to the storage account fails", func() {
				mockEnsureResourceGroupAndStorageAccount(ctx, azureClientFactory, azureGroupClient, azureStorageAccountClient, storageAccountName, backupBucket)

				// create generated secret
				mockGeneratedSecretCreation(ctx, c, sw, storageAccountName, backupBucket)

				azureClientFactory.EXPECT().ManagementPolicies().Return(azureManagementPoliciesClient, nil)
				azureManagementPoliciesClient.EXPECT().CreateOrUpdate(ctx, resourceGroupName, storageAccountName, 0).Return(fmt.Errorf("management policy addition on storage account error test"))

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).Should(HaveOccurred())
			})
		})

		Context("when the backupBucket configured without immutability does not exist", func() {
			BeforeEach(func() {
				mockEnsureResourceGroupAndStorageAccount(ctx, azureClientFactory, azureGroupClient, azureStorageAccountClient, storageAccountName, backupBucket)

				// create generated secret
				mockGeneratedSecretCreation(ctx, c, sw, storageAccountName, backupBucket)

				azureClientFactory.EXPECT().ManagementPolicies().Return(azureManagementPoliciesClient, nil)
				azureManagementPoliciesClient.EXPECT().CreateOrUpdate(ctx, resourceGroupName, storageAccountName, 0)

				azureClientFactory.EXPECT().BlobContainers().Return(azureBlobContainersClient, nil)

				// no bucket exists yet, return http.StatusNotFound
				azureBlobContainersClient.EXPECT().GetContainer(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(armstorage.BlobContainersClientGetResponse{}, &azcore.ResponseError{
					StatusCode: http.StatusNotFound,
				})
			})

			It("should error if bucket creation fails", func() {
				// create the bucket
				azureBlobContainersClient.EXPECT().CreateContainer(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(armstorage.BlobContainersClientCreateResponse{}, fmt.Errorf("blob storage container creation error test"))

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).Should(HaveOccurred())
			})

			It("should create the bucket successfully", func() {
				// create the bucket
				azureBlobContainersClient.EXPECT().CreateContainer(ctx, resourceGroupName, storageAccountName, backupBucket.Name)

				// No immutability policy will be present on the newly created bucket
				azureBlobContainersClient.EXPECT().GetImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(nil, false, nil, nil)

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when the backupBucket configured with unlocked immutability does not exist", func() {
			BeforeEach(func() {
				backupBucket.Spec.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1","kind":"BackupBucketConfig","immutability":{"retentionType":"bucket","retentionPeriod":"24h","locked":false}}`),
				}

				mockEnsureResourceGroupAndStorageAccount(ctx, azureClientFactory, azureGroupClient, azureStorageAccountClient, storageAccountName, backupBucket)

				// create generated secret
				mockGeneratedSecretCreation(ctx, c, sw, storageAccountName, backupBucket)

				azureClientFactory.EXPECT().ManagementPolicies().Return(azureManagementPoliciesClient, nil)
				azureManagementPoliciesClient.EXPECT().CreateOrUpdate(ctx, resourceGroupName, storageAccountName, 0)

				azureClientFactory.EXPECT().BlobContainers().Return(azureBlobContainersClient, nil)

				// no bucket exists yet, return http.StatusNotFound
				azureBlobContainersClient.EXPECT().GetContainer(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(armstorage.BlobContainersClientGetResponse{}, &azcore.ResponseError{
					StatusCode: http.StatusNotFound,
				})

				// create the bucket
				azureBlobContainersClient.EXPECT().CreateContainer(ctx, resourceGroupName, storageAccountName, backupBucket.Name)

				// No immutability policy will be present on the newly created bucket
				azureBlobContainersClient.EXPECT().GetImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(nil, false, &etag, nil)
			})

			It("should error if adding the immutability policy fails", func() {
				immutabilityDays := int32(1)
				azureBlobContainersClient.EXPECT().CreateOrUpdateImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name, &immutabilityDays).Return(nil, fmt.Errorf("adding the immutability policy error test"))

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).Should(HaveOccurred())
			})

			It("should add the immutability policy as configured", func() {
				immutabilityDays := int32(1)
				azureBlobContainersClient.EXPECT().CreateOrUpdateImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name, &immutabilityDays)

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when the backupBucket configured with locked immutability does not exist", func() {
			BeforeEach(func() {
				backupBucket.Spec.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1","kind":"BackupBucketConfig","immutability":{"retentionType":"bucket","retentionPeriod":"24h","locked":true}}`),
				}

				mockEnsureResourceGroupAndStorageAccount(ctx, azureClientFactory, azureGroupClient, azureStorageAccountClient, storageAccountName, backupBucket)

				// create generated secret
				mockGeneratedSecretCreation(ctx, c, sw, storageAccountName, backupBucket)

				azureClientFactory.EXPECT().ManagementPolicies().Return(azureManagementPoliciesClient, nil)
				azureManagementPoliciesClient.EXPECT().CreateOrUpdate(ctx, resourceGroupName, storageAccountName, 0)

				azureClientFactory.EXPECT().BlobContainers().Return(azureBlobContainersClient, nil)

				// no bucket exists yet, return http.StatusNotFound
				azureBlobContainersClient.EXPECT().GetContainer(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(armstorage.BlobContainersClientGetResponse{}, &azcore.ResponseError{
					StatusCode: http.StatusNotFound,
				})

				// create the bucket
				azureBlobContainersClient.EXPECT().CreateContainer(ctx, resourceGroupName, storageAccountName, backupBucket.Name)

				// No immutability policy will be present on the newly created bucket
				azureBlobContainersClient.EXPECT().GetImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(nil, false, &etag, nil)

				immutabilityDays := int32(1)
				azureBlobContainersClient.EXPECT().CreateOrUpdateImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name, &immutabilityDays).Return(&etag2, nil)
			})

			It("should error if locking the immutability policy fails after creation of bucket and addition of policy", func() {
				// lock the immutability policy
				azureBlobContainersClient.EXPECT().LockImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name, &etag2).Return(fmt.Errorf("locking the immutability policy error test"))

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).Should(HaveOccurred())
			})

			It("should create a locked bucket as configured", func() {
				// lock the immutability policy
				azureBlobContainersClient.EXPECT().LockImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name, &etag2)

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when a backupBucket configured for unlocked immutability already exists without immutability", func() {
			BeforeEach(func() {
				backupBucket.Status.GeneratedSecretRef = &corev1.SecretReference{
					Name:      fmt.Sprintf("generated-bucket-%s", backupBucket.Name),
					Namespace: "garden",
				}
				backupBucket.Spec.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1","kind":"BackupBucketConfig","immutability":{"retentionType":"bucket","retentionPeriod":"24h","locked":false}}`),
				}

				azureClientFactory.EXPECT().ManagementPolicies().Return(azureManagementPoliciesClient, nil)
				azureManagementPoliciesClient.EXPECT().CreateOrUpdate(ctx, resourceGroupName, storageAccountName, 0)

				azureClientFactory.EXPECT().BlobContainers().Return(azureBlobContainersClient, nil)

				// bucket already exists
				azureBlobContainersClient.EXPECT().GetContainer(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(armstorage.BlobContainersClientGetResponse{}, nil)

				// No immutability policy will be present on the bucket
				azureBlobContainersClient.EXPECT().GetImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(nil, false, &etag, nil)
			})

			It("should error if adding the policy fails", func() {
				immutabilityDays := int32(1)
				azureBlobContainersClient.EXPECT().CreateOrUpdateImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name, &immutabilityDays).Return(nil, fmt.Errorf("adding the immutability policy error test"))

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).Should(HaveOccurred())
			})

			It("should become unlocked immutable", func() {
				immutabilityDays := int32(1)
				azureBlobContainersClient.EXPECT().CreateOrUpdateImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name, &immutabilityDays)

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when a backupBucket configured for locked immutability already exists without immutability", func() {
			BeforeEach(func() {
				backupBucket.Status.GeneratedSecretRef = &corev1.SecretReference{
					Name:      fmt.Sprintf("generated-bucket-%s", backupBucket.Name),
					Namespace: "garden",
				}
				backupBucket.Spec.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1","kind":"BackupBucketConfig","immutability":{"retentionType":"bucket","retentionPeriod":"24h","locked":true}}`),
				}

				azureClientFactory.EXPECT().ManagementPolicies().Return(azureManagementPoliciesClient, nil)
				azureManagementPoliciesClient.EXPECT().CreateOrUpdate(ctx, resourceGroupName, storageAccountName, 0)

				azureClientFactory.EXPECT().BlobContainers().Return(azureBlobContainersClient, nil)

				// bucket already exists
				azureBlobContainersClient.EXPECT().GetContainer(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(armstorage.BlobContainersClientGetResponse{}, nil)

				// No immutability policy will be present on the bucket
				azureBlobContainersClient.EXPECT().GetImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(nil, false, &etag, nil)

				immutabilityDays := int32(1)
				azureBlobContainersClient.EXPECT().CreateOrUpdateImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name, &immutabilityDays).Return(&etag2, nil)

			})

			It("should error if adding the lock fails", func() {
				// lock the immutability policy
				azureBlobContainersClient.EXPECT().LockImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name, &etag2).Return(fmt.Errorf("locking the policy error test"))

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).Should(HaveOccurred())
			})

			It("should become locked immutable", func() {
				// lock the immutability policy
				azureBlobContainersClient.EXPECT().LockImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name, &etag2)

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when the backupBucket's configured unlocked immutability duration is different than present", func() {
			BeforeEach(func() {
				backupBucket.Status.GeneratedSecretRef = &corev1.SecretReference{
					Name:      fmt.Sprintf("generated-bucket-%s", backupBucket.Name),
					Namespace: "garden",
				}

				azureClientFactory.EXPECT().ManagementPolicies().Return(azureManagementPoliciesClient, nil)
				azureManagementPoliciesClient.EXPECT().CreateOrUpdate(ctx, resourceGroupName, storageAccountName, 0)
			})

			It("should increase the duration when configured so", func() {
				backupBucket.Spec.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1","kind":"BackupBucketConfig","immutability":{"retentionType":"bucket","retentionPeriod":"72h","locked":false}}`),
				}
				azureClientFactory.EXPECT().BlobContainers().Return(azureBlobContainersClient, nil)

				// bucket already exists
				azureBlobContainersClient.EXPECT().GetContainer(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(armstorage.BlobContainersClientGetResponse{}, nil)

				// immutability policy will be present on the bucket
				currentImmutabilityDays := int32(2)
				azureBlobContainersClient.EXPECT().GetImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(&currentImmutabilityDays, false, &etag, nil)

				newImmutabilityDays := int32(3)
				azureBlobContainersClient.EXPECT().CreateOrUpdateImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name, &newImmutabilityDays)

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should decrease the duration when configured so", func() {
				backupBucket.Spec.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1","kind":"BackupBucketConfig","immutability":{"retentionType":"bucket","retentionPeriod":"24h","locked":false}}`),
				}
				azureClientFactory.EXPECT().BlobContainers().Return(azureBlobContainersClient, nil)

				// bucket already exists
				azureBlobContainersClient.EXPECT().GetContainer(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(armstorage.BlobContainersClientGetResponse{}, nil)

				// immutability policy will be present on the bucket
				currentImmutabilityDays := int32(2)
				azureBlobContainersClient.EXPECT().GetImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(&currentImmutabilityDays, false, &etag, nil)

				newImmutabilityDays := int32(1)
				azureBlobContainersClient.EXPECT().CreateOrUpdateImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name, &newImmutabilityDays)

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should delete the policy when configured so", func() {
				backupBucket.Spec.ProviderConfig = nil
				azureClientFactory.EXPECT().BlobContainers().Return(azureBlobContainersClient, nil)

				// bucket already exists
				azureBlobContainersClient.EXPECT().GetContainer(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(armstorage.BlobContainersClientGetResponse{}, nil)

				// immutability policy will be present on the bucket
				currentImmutabilityDays := int32(2)
				azureBlobContainersClient.EXPECT().GetImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(&currentImmutabilityDays, false, &etag, nil)

				azureBlobContainersClient.EXPECT().DeleteImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name, &etag)

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when a backupbucket configured for locked immutability already exists in a locked state", func() {
			BeforeEach(func() {
				backupBucket.Status.GeneratedSecretRef = &corev1.SecretReference{
					Name:      fmt.Sprintf("generated-bucket-%s", backupBucket.Name),
					Namespace: "garden",
				}

				azureClientFactory.EXPECT().ManagementPolicies().Return(azureManagementPoliciesClient, nil)
				azureManagementPoliciesClient.EXPECT().CreateOrUpdate(ctx, resourceGroupName, storageAccountName, 0)
			})
			It("should extend the locked duration if configured", func() {
				backupBucket.Spec.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1","kind":"BackupBucketConfig","immutability":{"retentionType":"bucket","retentionPeriod":"48h","locked":true}}`),
				}
				azureClientFactory.EXPECT().BlobContainers().Return(azureBlobContainersClient, nil)

				// bucket already exists
				azureBlobContainersClient.EXPECT().GetContainer(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(armstorage.BlobContainersClientGetResponse{}, nil)

				// immutability policy will be present on the bucket
				currentImmutabilityDays := int32(1)
				azureBlobContainersClient.EXPECT().GetImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(&currentImmutabilityDays, true, &etag, nil)

				newImmutabilityDays := int32(2)
				azureBlobContainersClient.EXPECT().ExtendImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name, &newImmutabilityDays, &etag)

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when a backupbucket already exists in the expected state", func() {
			BeforeEach(func() {
				backupBucket.Status.GeneratedSecretRef = &corev1.SecretReference{
					Name:      fmt.Sprintf("generated-bucket-%s", backupBucket.Name),
					Namespace: "garden",
				}

				azureClientFactory.EXPECT().ManagementPolicies().Return(azureManagementPoliciesClient, nil)
				azureManagementPoliciesClient.EXPECT().CreateOrUpdate(ctx, resourceGroupName, storageAccountName, 0)
			})

			It("should no-op in a non-immutable state", func() {
				azureClientFactory.EXPECT().BlobContainers().Return(azureBlobContainersClient, nil)

				// bucket already exists
				azureBlobContainersClient.EXPECT().GetContainer(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(armstorage.BlobContainersClientGetResponse{}, nil)

				// No immutability policy will be present on the bucket
				azureBlobContainersClient.EXPECT().GetImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(nil, false, &etag, nil)

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should no-op in an unlocked immutable state", func() {
				backupBucket.Spec.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1","kind":"BackupBucketConfig","immutability":{"retentionType":"bucket","retentionPeriod":"24h","locked":false}}`),
				}

				azureClientFactory.EXPECT().BlobContainers().Return(azureBlobContainersClient, nil)

				// bucket already exists
				azureBlobContainersClient.EXPECT().GetContainer(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(armstorage.BlobContainersClientGetResponse{}, nil)

				// immutability policy will be present on the bucket
				immutabilityDays := int32(1)
				azureBlobContainersClient.EXPECT().GetImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(&immutabilityDays, false, &etag, nil)

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).ShouldNot(HaveOccurred())
			})

			It("should no-op in an locked immutable state", func() {
				backupBucket.Spec.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1","kind":"BackupBucketConfig","immutability":{"retentionType":"bucket","retentionPeriod":"24h","locked":true}}`),
				}

				azureClientFactory.EXPECT().BlobContainers().Return(azureBlobContainersClient, nil)

				// bucket already exists
				azureBlobContainersClient.EXPECT().GetContainer(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(armstorage.BlobContainersClientGetResponse{}, nil)

				// immutability policy will be present on the bucket
				immutabilityDays := int32(1)
				azureBlobContainersClient.EXPECT().GetImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(&immutabilityDays, true, &etag, nil)

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when providerConfig can not be decoded", func() {
			It("should error", func() {
				backupBucket.Spec.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1","kind":"BackupBucketConfig","immutblity":{"retenonType":"bucket","retentoeriod":"24h"}}`),
				}
				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).Should(HaveOccurred())
			})
		})
	})

	Describe("#Delete", func() {
		var (
			generatedSecret *corev1.Secret
		)

		BeforeEach(func() {
			generatedSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("generated-bucket-%s", backupBucket.Name),
					Namespace: "garden",
				},
			}
			backupBucket.Status.GeneratedSecretRef = &corev1.SecretReference{
				Name:      generatedSecret.Name,
				Namespace: generatedSecret.Namespace,
			}
			// passing &corev1.Secret{} here instead of generatedSecret
			// since kutil.GetSecretByReference() uses a &corev1.Secret{} to Get()
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(generatedSecret), &corev1.Secret{})
			azureClientFactory.EXPECT().BlobContainers().Return(azureBlobContainersClient, nil)
		})

		It("should error if deleting the bucket fails", func() {
			azureBlobContainersClient.EXPECT().DeleteContainer(ctx, resourceGroupName, storageAccountName, backupBucket.Name).Return(fmt.Errorf("bucket deletion error test"))

			err := a.Delete(ctx, logger, backupBucket)
			Expect(err).Should(HaveOccurred())
		})

		It("should delete the bucket", func() {
			azureBlobContainersClient.EXPECT().DeleteContainer(ctx, resourceGroupName, storageAccountName, backupBucket.Name)

			azureClientFactory.EXPECT().Group().Return(azureGroupClient, nil)
			azureGroupClient.EXPECT().Delete(ctx, resourceGroupName)

			c.EXPECT().Delete(ctx, generatedSecret)

			err := a.Delete(ctx, logger, backupBucket)
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})

// mocks ensureResourceGroupAndStorageAccount() which creates the resource group and storage account
func mockEnsureResourceGroupAndStorageAccount(
	ctx context.Context,
	azureClientFactory *mockazureclient.MockFactory,
	azureGroupClient *mockazureclient.MockResourceGroup,
	azureStorageAccountClient *mockazureclient.MockStorageAccount,
	storageAccountName string, backupBucket *extensionsv1alpha1.BackupBucket,

) {
	// create resource group
	azureClientFactory.EXPECT().Group().Return(azureGroupClient, nil)
	azureGroupClient.EXPECT().CreateOrUpdate(ctx, name, armresources.ResourceGroup{
		Location: to.Ptr(backupBucket.Spec.Region),
	})

	// create storage account
	azureClientFactory.EXPECT().StorageAccount().Return(azureStorageAccountClient, nil)
	azureStorageAccountClient.EXPECT().CreateStorageAccount(ctx, name, storageAccountName, backupBucket.Spec.Region)
	azureStorageAccountClient.EXPECT().ListStorageAccountKey(ctx, name, storageAccountName).Return(storageAccountKey, nil)
}

func mockGeneratedSecretCreation(
	ctx context.Context,
	c *mockclient.MockClient,
	sw *mockclient.MockStatusWriter,
	storageAccountName string,
	backupBucket *extensionsv1alpha1.BackupBucket,
) {
	generatedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("generated-bucket-%s", backupBucket.Name),
			Namespace: "garden",
		},
	}

	c.EXPECT().Get(ctx, client.ObjectKeyFromObject(generatedSecret), generatedSecret.DeepCopy()).Return(apierrors.NewNotFound(schema.GroupResource{}, generatedSecret.Name))
	// mutateFn's side effect
	generatedSecret.Data = map[string][]byte{
		"domain":         []byte(azure.AzureBlobStorageDomain),
		"storageAccount": []byte(storageAccountName),
		"storageKey":     []byte(storageAccountKey),
	}

	c.EXPECT().Create(ctx, generatedSecret)
	backupBucketCopy := backupBucket.DeepCopy()
	backupBucketCopy.Status.GeneratedSecretRef = &corev1.SecretReference{
		Name:      generatedSecret.Name,
		Namespace: generatedSecret.Namespace,
	}

	// gomock.Any() needs to be used here since the patch can never be the same
	// as MergeFrom() needs a deepcopy, which creates a different base object
	sw.EXPECT().Patch(ctx, backupBucketCopy, gomock.Any())
}
