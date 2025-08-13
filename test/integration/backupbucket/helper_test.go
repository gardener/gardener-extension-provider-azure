// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/logger"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	azurev1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	azurebackupbucket "github.com/gardener/gardener-extension-provider-azure/pkg/controller/backupbucket"
	"github.com/gardener/gardener-extension-provider-azure/pkg/features"
)

type azureClientSet struct {
	resourceGroups  *armresources.ResourceGroupsClient
	storageAccounts *armstorage.AccountsClient
	blobContainers  *armstorage.BlobContainersClient
}

func newAzureClientSet(subscriptionId, tenantId, clientId, clientSecret string) (*azureClientSet, error) {
	credential, err := azidentity.NewClientSecretCredential(tenantId, clientId, clientSecret, nil)
	if err != nil {
		return nil, err
	}

	resourceGroupsClient, err := armresources.NewResourceGroupsClient(subscriptionId, credential, nil)
	if err != nil {
		return nil, err
	}

	storageAccountsClient, err := armstorage.NewAccountsClient(subscriptionId, credential, nil)
	if err != nil {
		return nil, err
	}

	blobContainersClient, err := armstorage.NewBlobContainersClient(subscriptionId, credential, nil)
	if err != nil {
		return nil, err
	}

	return &azureClientSet{
		resourceGroups:  resourceGroupsClient,
		storageAccounts: storageAccountsClient,
		blobContainers:  blobContainersClient,
	}, nil
}

func secretsFromEnv() {
	if len(*subscriptionId) == 0 {
		subscriptionId = ptr.To(os.Getenv("SUBSCRIPTION_ID"))
	}
	if len(*tenantId) == 0 {
		tenantId = ptr.To(os.Getenv("TENANT_ID"))
	}
	if len(*clientId) == 0 {
		clientId = ptr.To(os.Getenv("CLIENT_ID"))
	}
	if len(*clientSecret) == 0 {
		clientSecret = ptr.To(os.Getenv("CLIENT_SECRET"))
	}
	if len(*region) == 0 {
		region = ptr.To(os.Getenv("REGION"))
	}
}

func validateFlags() {
	if len(*subscriptionId) == 0 {
		panic("Azure subscription ID required. Either provide it via the subscription-id flag or set the SUBSCRIPTION_ID environment variable")
	}
	if len(*tenantId) == 0 {
		panic("Azure tenant ID required. Either provide it via the tenant-id flag or set the TENANT_ID environment variable")
	}
	if len(*clientId) == 0 {
		panic("Azure client ID required. Either provide it via the client-id flag or set the CLIENT_ID environment variable")
	}
	if len(*clientSecret) == 0 {
		panic("Azure client secret required. Either provide it via the client-secret flag or set the CLIENT_SECRET environment variable")
	}
	if len(*region) == 0 {
		panic("Azure region required. Either provide it via the region flag or set the REGION environment variable")
	}
	if len(*logLevel) == 0 {
		logLevel = ptr.To(logger.DebugLevel)
	} else {
		if !slices.Contains(logger.AllLogLevels, *logLevel) {
			panic("Invalid log level: " + *logLevel)
		}
	}
}

func enableFeatureGate() {
	err := features.ExtensionFeatureGate.Set("EnableImmutableBuckets=true")
	Expect(err).NotTo(HaveOccurred(), "Failed to enable feature gate EnableImmutableBuckets")
	Expect(features.ExtensionFeatureGate.Enabled(features.EnableImmutableBuckets)).To(BeTrue(), "EnableImmutableBuckets feature gate should be enabled")
}

func createNamespace(ctx context.Context, c client.Client, namespace *corev1.Namespace) {
	log.Info("Creating namespace", "namespace", namespace.Name)
	Expect(c.Create(ctx, namespace)).To(Succeed(), "Failed to create namespace: %s", namespace.Name)
}

func deleteNamespace(ctx context.Context, c client.Client, namespace *corev1.Namespace) {
	log.Info("Deleting namespace", "namespace", namespace.Name)
	Expect(client.IgnoreNotFound(c.Delete(ctx, namespace))).To(Succeed())
}

func ensureGardenNamespace(ctx context.Context, c client.Client) (*corev1.Namespace, bool) {
	gardenNamespaceAlreadyExists := false
	gardenNamespace := &corev1.Namespace{}
	err := c.Get(ctx, client.ObjectKey{Name: gardenNamespaceName}, gardenNamespace)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			log.Info("Garden namespace not found, creating it", "namespace", gardenNamespaceName)
			gardenNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: gardenNamespaceName,
				},
			}
			Expect(c.Create(ctx, gardenNamespace)).To(Succeed(), "Failed to create garden namespace")
		} else {
			log.Error(err, "Failed to check for garden namespace")
			Expect(err).NotTo(HaveOccurred(), "Unexpected error while checking for garden namespace")
		}
	} else {
		gardenNamespaceAlreadyExists = true
		log.Info("Garden namespace already exists", "namespace", gardenNamespaceName)
	}
	return gardenNamespace, gardenNamespaceAlreadyExists
}

func createBackupBucketSecret(ctx context.Context, c client.Client, secret *corev1.Secret) {
	log.Info("Creating secret", "name", secret.Name, "namespace", secret.Namespace)
	Expect(c.Create(ctx, secret)).To(Succeed(), "Failed to create secret: %s", secret.Name)
}

func deleteBackupBucketSecret(ctx context.Context, c client.Client, secret *corev1.Secret) {
	log.Info("Deleting secret", "name", secret.Name, "namespace", secret.Namespace)
	Expect(client.IgnoreNotFound(c.Delete(ctx, secret))).To(Succeed())
}

func createResourceGroup(ctx context.Context, azClientSet *azureClientSet, resourceGroupName string, region string) {
	log.Info("Creating Azure resource group", "resourceGroupName", resourceGroupName, "region", region)
	_, err := azClientSet.resourceGroups.CreateOrUpdate(ctx, resourceGroupName, armresources.ResourceGroup{
		Location: ptr.To(region),
	}, nil)
	Expect(err).NotTo(HaveOccurred(), "Failed to create Azure resource group %s in region %s. Error: %v", resourceGroupName, region, err)
}

func deleteResourceGroup(ctx context.Context, azClientSet *azureClientSet, resourceGroupName string) {
	log.Info("Deleting Azure resource group", "resourceGroupName", resourceGroupName)
	poller, err := azClientSet.resourceGroups.BeginDelete(ctx, resourceGroupName, nil)
	if err != nil {
		if isNotFoundError(err) {
			log.Info("Resource group is already marked for deletion or does not exist", "resourceGroupName", resourceGroupName)
			return
		}
		Expect(err).NotTo(HaveOccurred(), "Failed to initiate deletion of resource group: %s", resourceGroupName)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		if isNotFoundError(err) {
			log.Info("Resource group already deleted during polling", "resourceGroupName", resourceGroupName)
			return
		}
		Expect(err).NotTo(HaveOccurred(), "Failed to delete resource group")
	}
	log.Info("Azure resource group successfully deleted", "resourceGroupName", resourceGroupName)
}

func isNotFoundError(err error) bool {
	var responseError *azcore.ResponseError
	if errors.As(err, &responseError) {
		return responseError.StatusCode == http.StatusNotFound
	}
	return false
}

func createBackupBucket(ctx context.Context, c client.Client, backupBucket *extensionsv1alpha1.BackupBucket) {
	log.Info("Creating backupBucket", "backupBucket", backupBucket)
	Expect(c.Create(ctx, backupBucket)).To(Succeed(), "Failed to create backupBucket: %s", backupBucket.Name)
}

func fetchBackupBucket(ctx context.Context, c client.Client, name string) *extensionsv1alpha1.BackupBucket {
	backupBucket := &extensionsv1alpha1.BackupBucket{}
	err := c.Get(ctx, client.ObjectKey{Name: name}, backupBucket)
	Expect(err).NotTo(HaveOccurred(), "Failed to fetch backupBucket from the cluster")
	return backupBucket
}

func deleteBackupBucket(ctx context.Context, c client.Client, backupBucket *extensionsv1alpha1.BackupBucket) {
	log.Info("Deleting backupBucket", "backupBucket", backupBucket)
	Expect(client.IgnoreNotFound(c.Delete(ctx, backupBucket))).To(Succeed())
}

func waitForObservedGeneration(ctx context.Context, c client.Client, backupBucket *extensionsv1alpha1.BackupBucket) {
	generation := backupBucket.Generation
	Eventually(func() bool {
		updated := &extensionsv1alpha1.BackupBucket{}
		if err := c.Get(ctx, client.ObjectKey{Name: backupBucket.Name}, updated); err != nil {
			return false
		}
		return updated.Status.ObservedGeneration == generation
	}, 2*time.Minute, 2*time.Second).Should(BeTrue(), "BackupBucket's observedGeneration did not match generation")
}

func waitForRotationAnnotationRemoval(ctx context.Context, c client.Client, backupBucketName string) {
	Eventually(func() bool {
		bb := &extensionsv1alpha1.BackupBucket{}
		if err := c.Get(ctx, client.ObjectKey{Name: backupBucketName}, bb); err != nil {
			return false
		}
		_, exists := bb.Annotations[azure.StorageAccountKeyMustRotate]
		return !exists
	}, 2*time.Minute, 2*time.Second).Should(BeTrue(), "Rotation annotation was not removed")
}

func waitForImmutabilityPolicyRemoval(ctx context.Context, azClientSet *azureClientSet, resourceGroupName, storageAccountName, containerName string) {
	Eventually(func() bool {
		policy, err := azClientSet.blobContainers.GetImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, containerName, nil)
		if err != nil {
			return isNotFoundError(err)
		}
		return policy.Etag == nil || *policy.Etag == ""
	}, 2*time.Minute, 2*time.Second).Should(BeTrue(), "Immutability policy was not removed from blob container")
}

func waitUntilBackupBucketReady(ctx context.Context, c client.Client, backupBucket *extensionsv1alpha1.BackupBucket) {
	Expect(extensions.WaitUntilExtensionObjectReady(
		ctx,
		c,
		log,
		backupBucket,
		extensionsv1alpha1.BackupBucketResource,
		10*time.Second,
		30*time.Second,
		5*time.Minute,
		nil,
	)).To(Succeed(), "BackupBucket did not become ready: %s", backupBucket.Name)
	log.Info("BackupBucket is ready", "backupBucket", backupBucket)
}

func waitUntilBackupBucketDeleted(ctx context.Context, c client.Client, backupBucket *extensionsv1alpha1.BackupBucket) {
	Expect(extensions.WaitUntilExtensionObjectDeleted(
		ctx,
		c,
		log,
		backupBucket.DeepCopy(),
		extensionsv1alpha1.BackupBucketResource,
		10*time.Second,
		5*time.Minute,
	)).To(Succeed())
	log.Info("BackupBucket successfully deleted", "backupBucket", backupBucket)
}

func newBackupBucket(name, region string, providerConfig *azurev1alpha1.BackupBucketConfig) *extensionsv1alpha1.BackupBucket {
	var providerConfigRaw *runtime.RawExtension
	if providerConfig != nil {
		providerConfig.APIVersion = "azure.provider.extensions.gardener.cloud/v1alpha1"
		providerConfig.Kind = "BackupBucketConfig"
		providerConfigJSON, err := json.Marshal(providerConfig)
		Expect(err).NotTo(HaveOccurred(), "Failed to marshal providerConfig to JSON")
		providerConfigRaw = &runtime.RawExtension{
			Raw: providerConfigJSON,
		}
		log.Info("Creating new backupBucket object", "region", region, "providerConfig", string(providerConfigJSON))
	} else {
		providerConfigRaw = &runtime.RawExtension{}
		log.Info("Creating new backupBucket object with empty providerConfig", "region", region)
	}

	return &extensionsv1alpha1.BackupBucket{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "extensions.gardener.cloud/v1alpha1",
			Kind:       "BackupBucket",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: extensionsv1alpha1.BackupBucketSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           azure.Type,
				ProviderConfig: providerConfigRaw,
			},
			Region: region,
			SecretRef: corev1.SecretReference{
				Name:      backupBucketSecretName,
				Namespace: name,
			},
		},
	}
}

func randomString() string {
	rs, err := gardenerutils.GenerateRandomStringFromCharset(5, "0123456789abcdefghijklmnopqrstuvwxyz")
	Expect(err).NotTo(HaveOccurred(), "Failed to generate random string")
	log.Info("Generated random string", "randomString", rs)
	return rs
}

// functions for verification
func verifyBackupBucketAndStatus(ctx context.Context, c client.Client, azClientSet *azureClientSet, backupBucket *extensionsv1alpha1.BackupBucket) {
	By("getting backupbucket and verifying its status")
	verifyBackupBucketStatus(ctx, c, backupBucket)

	By("verifying that the Azure storage account and container exist and match backupbucket")
	verifyBackupBucket(ctx, azClientSet, backupBucket)
}

func verifyBackupBucketStatus(ctx context.Context, c client.Client, backupBucket *extensionsv1alpha1.BackupBucket) {
	log.Info("Verifying backupBucket", "backupBucket", backupBucket)
	By("fetching backupBucket from the cluster")
	backupBucket = fetchBackupBucket(ctx, c, backupBucket.Name)

	By("verifying LastOperation state")
	Expect(backupBucket.Status.LastOperation).NotTo(BeNil(), "LastOperation should not be nil")
	Expect(backupBucket.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded), "LastOperation state should be Succeeded")
	Expect(backupBucket.Status.LastOperation.Type).To(Equal(gardencorev1beta1.LastOperationTypeCreate), "LastOperation type should be Create")

	By("verifying GeneratedSecretRef")
	if backupBucket.Status.GeneratedSecretRef != nil {
		Expect(backupBucket.Status.GeneratedSecretRef.Name).NotTo(BeEmpty(), "GeneratedSecretRef name should not be empty")
		Expect(backupBucket.Status.GeneratedSecretRef.Namespace).NotTo(BeEmpty(), "GeneratedSecretRef namespace should not be empty")
	}
}

func verifyBackupBucket(ctx context.Context, azClientSet *azureClientSet, backupBucket *extensionsv1alpha1.BackupBucket) {
	storageAccountName := azurebackupbucket.GenerateStorageAccountName(backupBucket.Name)
	resourceGroupName, containerName := backupBucket.Name, backupBucket.Name
	log.Info("Verifying backupBucket on Azure", "storageAccountName", storageAccountName, "containerName", containerName)

	By("verifying Azure storage account")
	storageAccount, err := azClientSet.storageAccounts.GetProperties(ctx, resourceGroupName, storageAccountName, nil)
	Expect(err).NotTo(HaveOccurred(), "Failed to get Azure Storage Account properties")
	Expect(storageAccount).NotTo(BeNil(), "Azure Storage Account should exist")
	Expect(storageAccount.Properties).NotTo(BeNil(), "Storage Account properties should not be nil")

	By("verifying Azure blob container")
	blobContainer, err := azClientSet.blobContainers.Get(ctx, resourceGroupName, storageAccountName, containerName, nil)
	Expect(err).NotTo(HaveOccurred(), "Failed to get Azure Blob Container properties")
	Expect(blobContainer).NotTo(BeNil(), "Azure Blob Container should exist")
	Expect(blobContainer.ContainerProperties).NotTo(BeNil(), "Blob Container properties should not be nil")
}

func verifyBackupBucketDeleted(ctx context.Context, azClientSet *azureClientSet, backupBucket *extensionsv1alpha1.BackupBucket) {
	storageAccountName := azurebackupbucket.GenerateStorageAccountName(backupBucket.Name)
	containerName := backupBucket.Name
	log.Info("Verifying backupBucket deletion on Azure", "storageAccountName", storageAccountName, "containerName", containerName)

	By("verifying Azure blob container deletion")
	_, err := azClientSet.blobContainers.Get(ctx, backupBucket.Spec.Region, storageAccountName, containerName, nil)
	Expect(err).To(HaveOccurred(), "Expected blob container to be deleted, but it still exists")

	By("verifying Azure storage account deletion")
	_, err = azClientSet.storageAccounts.GetProperties(ctx, backupBucket.Spec.Region, storageAccountName, nil)
	Expect(err).To(HaveOccurred(), "Expected storage account to be deleted, but it still exists")
}

func verifyImmutabilityPolicy(ctx context.Context, azClientSet *azureClientSet, backupBucket *extensionsv1alpha1.BackupBucket, immutabilityConfig *azurev1alpha1.ImmutableConfig) {
	storageAccountName := azurebackupbucket.GenerateStorageAccountName(backupBucket.Name)
	resourceGroupName, containerName := backupBucket.Name, backupBucket.Name

	By("fetching immutability policy from Azure")
	policy, err := azClientSet.blobContainers.GetImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, containerName, nil)
	Expect(err).NotTo(HaveOccurred(), "Failed to fetch immutability policy from Azure")

	By("verifying immutability policy configuration")
	Expect(*policy.Properties.ImmutabilityPeriodSinceCreationInDays).To(Equal(int32(immutabilityConfig.RetentionPeriod.Hours()/24)), "Retention period mismatch")
	Expect(*policy.Properties.State).To(Equal(armstorage.ImmutabilityPolicyStateUnlocked), "Immutability policy state mismatch")
}

func verifyContainerImmutability(ctx context.Context, c client.Client, azClientSet *azureClientSet, backupBucket *extensionsv1alpha1.BackupBucket) {
	storageAccountName := azurebackupbucket.GenerateStorageAccountName(backupBucket.Name)
	resourceGroupName, containerName := backupBucket.Name, backupBucket.Name

	defer func() {
		By("deleting immutability policy on backupBucket")
		backupBucket = fetchBackupBucket(ctx, c, backupBucket.Name)
		backupBucketPatch := client.MergeFrom(backupBucket.DeepCopy())
		if backupBucket.Annotations == nil {
			backupBucket.Annotations = make(map[string]string)
		}
		backupBucket.Annotations["gardener.cloud/operation"] = "reconcile"
		backupBucket.Spec.ProviderConfig = nil
		err := c.Patch(ctx, backupBucket, backupBucketPatch)
		Expect(err).NotTo(HaveOccurred(), "failed to patch backupBucket to remove immutability policy")

		By("waiting for observed generation to match backupBucket generation")
		waitForObservedGeneration(ctx, c, backupBucket)

		By("waiting for backupBucket to become ready")
		waitUntilBackupBucketReady(ctx, c, backupBucket)

		By("waiting for immutability policy to be removed")
		waitForImmutabilityPolicyRemoval(ctx, azClientSet, resourceGroupName, storageAccountName, containerName)
	}()

	By("creating block blob client")
	// create clients: azblobClient -> containerClient -> blockBlobClient
	// get storage account key to create azblob client
	response, err := azClientSet.storageAccounts.ListKeys(ctx, resourceGroupName, storageAccountName, nil)
	Expect(err).NotTo(HaveOccurred(), "Failed to list storage account keys")
	connectionString := fmt.Sprintf("DefaultEndpointsProtocol=https;AccountName=%s;AccountKey=%s;EndpointSuffix=%s", storageAccountName, *response.Keys[0].Value, strings.TrimPrefix(azure.AzureBlobStorageDomain, "blob."))

	azblobClient, err := azblob.NewClientFromConnectionString(connectionString, nil)
	Expect(err).NotTo(HaveOccurred(), "Failed to create Azure Blob client")

	containerClient := azblobClient.ServiceClient().NewContainerClient(containerName)

	blockBlobClient := containerClient.NewBlockBlobClient(containerName)

	By("writing an object to the bucket")
	_, err = blockBlobClient.UploadBuffer(ctx, []byte("test data"), nil)
	Expect(err).NotTo(HaveOccurred(), "Failed to upload buffer to blob")

	By("attempting to overwrite the block blob")
	_, err = blockBlobClient.UploadBuffer(ctx, []byte("new data"), nil)
	Expect(err).To(HaveOccurred(), "Expected error when overwriting a blob with immutability policy enabled")
	log.Info("Expected error when overwriting a blob with immutability policy enabled", "error", err)

	By("attempting to delete the block blob")
	_, err = blockBlobClient.Delete(ctx, nil)
	Expect(err).To(HaveOccurred(), "Expected error when deleting a blob with immutability policy enabled")
	log.Info("Expected error when deleting a blob with immutability policy enabled", "error", err)
}

func verifyLockedImmutabilityPolicy(ctx context.Context, azClientSet *azureClientSet, backupBucket *extensionsv1alpha1.BackupBucket) {
	storageAccountName := azurebackupbucket.GenerateStorageAccountName(backupBucket.Name)
	resourceGroupName, containerName := backupBucket.Name, backupBucket.Name

	By("attempting to delete the immutability policy on blob container")
	policy, err := azClientSet.blobContainers.GetImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, containerName, nil)
	Expect(err).NotTo(HaveOccurred(), "Failed to fetch immutability policy")
	_, err = azClientSet.blobContainers.DeleteImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, containerName, *policy.Etag, nil)
	Expect(err).To(HaveOccurred(), "Expected error when deleting a locked immutability policy")
	log.Info("Expected error when deleting a locked immutability policy", "error", err)

	By("attempting to mutate the immutability policy on blob container")
	options := &armstorage.BlobContainersClientCreateOrUpdateImmutabilityPolicyOptions{
		IfMatch: policy.Etag,
		Parameters: &armstorage.ImmutabilityPolicy{
			Properties: &armstorage.ImmutabilityPolicyProperty{
				// decreasing immutability period to 0 days (increasing is allowed for a locked immutability policy)
				ImmutabilityPeriodSinceCreationInDays: ptr.To(int32(0)),
			},
		},
	}
	_, err = azClientSet.blobContainers.CreateOrUpdateImmutabilityPolicy(ctx, resourceGroupName, storageAccountName, containerName, options)
	Expect(err).To(HaveOccurred(), "Expected error when trying to mutate a locked immutability policy")
	log.Info("Expected error when trying to mutate a locked immutability policy", "error", err)
}

func verifyKeyRotationPolicy(ctx context.Context, azClientSet *azureClientSet, backupBucket *extensionsv1alpha1.BackupBucket, rotationConfig *azurev1alpha1.RotationConfig) {
	resourceGroupName := backupBucket.Name
	storageAccountName := azurebackupbucket.GenerateStorageAccountName(backupBucket.Name)

	By("verifying number of storage account keys")
	response, err := azClientSet.storageAccounts.ListKeys(ctx, resourceGroupName, storageAccountName, nil)
	Expect(err).NotTo(HaveOccurred(), "Failed to fetch storage account keys from Azure")
	Expect(response.Keys).To(HaveLen(2), "Expected two storage account keys")

	By("verifying key creation times")
	for _, key := range response.Keys {
		Expect(key.CreationTime).NotTo(BeNil(), "Key creation time should not be nil")
		Expect(time.Since(*key.CreationTime)).To(BeNumerically("<", time.Duration(rotationConfig.RotationPeriodDays)*24*time.Hour), "Key age should not exceed rotation period")
	}

	By("verifying key rotation policy configuration")
	properties, err := azClientSet.storageAccounts.GetProperties(ctx, resourceGroupName, storageAccountName, nil)
	Expect(err).NotTo(HaveOccurred(), "Failed to get storage account properties")
	Expect(properties.Account.Properties.KeyPolicy.KeyExpirationPeriodInDays).To(Equal(rotationConfig.ExpirationPeriodDays), "Key expiration period mismatch")
}

func verifyKeyRotation(ctx context.Context, c client.Client, azClientSet *azureClientSet, backupBucket *extensionsv1alpha1.BackupBucket) {
	resourceGroupName := backupBucket.Name
	storageAccountName := azurebackupbucket.GenerateStorageAccountName(backupBucket.Name)

	By("getting current storage account keys")
	responseBeforeRotation, err := azClientSet.storageAccounts.ListKeys(ctx, resourceGroupName, storageAccountName, nil)
	Expect(err).NotTo(HaveOccurred(), "Failed to fetch storage account keys from Azure before rotation")
	Expect(responseBeforeRotation.Keys).To(HaveLen(2), "Expected two storage account keys")

	By("adding key rotation annotation to backupBucket")
	backupBucket = fetchBackupBucket(ctx, c, backupBucket.Name)
	backupBucketPatch := client.MergeFrom(backupBucket.DeepCopy())
	if backupBucket.Annotations == nil {
		backupBucket.Annotations = make(map[string]string)
	}
	backupBucket.Annotations[azure.StorageAccountKeyMustRotate] = "true"
	backupBucket.Annotations["gardener.cloud/operation"] = "reconcile"
	err = c.Patch(ctx, backupBucket, backupBucketPatch)
	Expect(err).NotTo(HaveOccurred(), "Failed to add key rotation annotation to backupBucket")

	By("waiting for observed generation to match backupBucket generation")
	waitForObservedGeneration(ctx, c, backupBucket)

	By("waiting for rotation annotation to be removed from the backupBucket")
	waitForRotationAnnotationRemoval(ctx, c, backupBucket.Name)

	By("ensuring keys are rotated")
	responseAfterRotation, err := azClientSet.storageAccounts.ListKeys(ctx, resourceGroupName, storageAccountName, nil)
	Expect(err).NotTo(HaveOccurred(), "Failed to fetch storage account keys from Azure after rotation")
	Expect(responseAfterRotation.Keys).To(HaveLen(2), "Expected two storage account keys after rotation")
	oldKeys := azurebackupbucket.SortKeysByAge(responseBeforeRotation.Keys)
	newKeys := azurebackupbucket.SortKeysByAge(responseAfterRotation.Keys)
	Expect(newKeys[1].Value).To(Equal(oldKeys[0].Value), "Expected the newest key to be the oldest key after rotation")
	Expect(newKeys[0].Value).NotTo(Equal(oldKeys[0].Value), "Expected rotated key not be the same as the newest key before rotation")
	Expect(newKeys[0].Value).NotTo(Equal(oldKeys[1].Value), "Expected rotated key not to be the same as the oldest key before rotation")

	By("ensuring BackupBucket's GeneratedSecretRef is updated")
	backupBucket = fetchBackupBucket(ctx, c, backupBucket.Name)
	Expect(backupBucket.Status.GeneratedSecretRef).NotTo(BeNil(), "GeneratedSecretRef should not be nil after key rotation")
	secret, err := kutil.GetSecretByReference(ctx, c, backupBucket.Status.GeneratedSecretRef)
	Expect(err).NotTo(HaveOccurred(), "Failed to fetch generated secret after key rotation")
	Expect(secret.Data[azure.StorageAccount]).To(Equal([]byte(storageAccountName)), "Storage account name in generated secret should match")
	Expect(secret.Data[azure.StorageKey]).To(Equal([]byte(*newKeys[0].Value)), "Storage key in generated secret should match the new key")
}

func verifyImmutabilityAndKeyRotation(ctx context.Context, c client.Client, azClientSet *azureClientSet, backupBucket *extensionsv1alpha1.BackupBucket, providerConfig *azurev1alpha1.BackupBucketConfig) {
	By("verifying key rotation policy")
	verifyKeyRotationPolicy(ctx, azClientSet, backupBucket, providerConfig.RotationConfig)

	By("verifying key rotation")
	verifyKeyRotation(ctx, c, azClientSet, backupBucket)

	By("verifying immutability policy")
	verifyImmutabilityPolicy(ctx, azClientSet, backupBucket, providerConfig.Immutability)

	By("verifying container immutability")
	verifyContainerImmutability(ctx, c, azClientSet, backupBucket)
}
