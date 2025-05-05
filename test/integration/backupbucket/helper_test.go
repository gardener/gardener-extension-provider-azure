// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	azurev1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	azurebackupbucket "github.com/gardener/gardener-extension-provider-azure/pkg/controller/backupbucket"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createNamespace(ctx context.Context, c client.Client, namespace *corev1.Namespace) {
	log.Info("Creating namespace", "namespace", namespace.Name)
	Expect(c.Create(ctx, namespace)).To(Succeed(), "Failed to create namespace: %s", namespace.Name)
}

func deleteNamespace(ctx context.Context, c client.Client, namespace *corev1.Namespace) {
	log.Info("Deleting namespace", "namespace", namespace.Name)
	Expect(client.IgnoreNotFound(c.Delete(ctx, namespace))).To(Succeed())
}

func createBackupBucketSecret(ctx context.Context, c client.Client, secret *corev1.Secret) {
	log.Info("Creating secret", "name", secret.Name, "namespace", secret.Namespace)
	Expect(c.Create(ctx, secret)).To(Succeed())
}

func deleteBackupBucketSecret(ctx context.Context, c client.Client, secret *corev1.Secret) {
	log.Info("Deleting secret", "name", secret.Name, "namespace", secret.Namespace)
	Expect(client.IgnoreNotFound(c.Delete(ctx, secret))).To(Succeed())
}

func createResourceGroup(ctx context.Context, clientSet *azureClientSet, resourceGroupName string, region string) {
	log.Info("Creating Azure resource group", "resourceGroupName", resourceGroupName, "region", region)
	_, err := clientSet.resourceGroups.CreateOrUpdate(ctx, resourceGroupName, armresources.ResourceGroup{
		Location: to.Ptr(region),
	}, nil)
	Expect(err).NotTo(HaveOccurred())
}

func deleteResourceGroup(ctx context.Context, clientSet *azureClientSet, resourceGroupName string) {
	log.Info("Deleting Azure resource group", "resourceGroupName", resourceGroupName)
	poller, err := clientSet.resourceGroups.BeginDelete(ctx, resourceGroupName, nil)
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
	Expect(c.Create(ctx, backupBucket)).To(Succeed())
}

func deleteBackupBucket(ctx context.Context, c client.Client, backupBucket *extensionsv1alpha1.BackupBucket) {
	log.Info("Deleting backupBucket", "backupBucket", backupBucket)
	Expect(client.IgnoreNotFound(c.Delete(ctx, backupBucket))).To(Succeed())
}

func waitUntilBackupBucketReady(ctx context.Context, c client.Client, log logr.Logger, backupBucket *extensionsv1alpha1.BackupBucket) {
	err := extensions.WaitUntilExtensionObjectReady(
		ctx,
		c,
		log,
		backupBucket,
		extensionsv1alpha1.BackupBucketResource,
		10*time.Second,
		30*time.Second,
		5*time.Minute,
		nil,
	)
	if err != nil {
		log.Info("BackupBucket is not ready yet; this is expected during initial reconciliation", "error", err)
	}
	Expect(err).To(Succeed(), "BackupBucket did not become ready: %s", backupBucket.Name)
	log.Info("BackupBucket is ready", "backupBucket", backupBucket)
}

func waitUntilBackupBucketDeleted(ctx context.Context, c client.Client, log logr.Logger, backupBucket *extensionsv1alpha1.BackupBucket) {
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

func getBackupBucketAndVerifyStatus(ctx context.Context, c client.Client, backupBucket *extensionsv1alpha1.BackupBucket) {
	log.Info("Verifying backupBucket", "backupBucket", backupBucket)
	Expect(c.Get(ctx, client.ObjectKey{Name: backupBucket.Name}, backupBucket)).To(Succeed())

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

func verifyBackupBucket(ctx context.Context, clientSet *azureClientSet, backupBucket *extensionsv1alpha1.BackupBucket) {
	storageAccountName := azurebackupbucket.GenerateStorageAccountName(backupBucket.Name)
	containerName := backupBucket.Name
	log.Info("Verifying backupBucket on Azure", "storageAccountName", storageAccountName, "containerName", containerName)

	By("verifying Azure storage account")
	storageAccount, err := clientSet.storageAccounts.GetProperties(ctx, testName, storageAccountName, nil)
	Expect(err).NotTo(HaveOccurred(), "Failed to get Azure Storage Account properties")
	Expect(storageAccount).NotTo(BeNil(), "Azure Storage Account should exist")

	Expect(storageAccount.Properties).NotTo(BeNil(), "Storage Account properties should not be nil")

	By("verifying Azure blob container")
	blobContainer, err := clientSet.blobContainers.Get(ctx, testName, storageAccountName, containerName, nil)
	Expect(err).NotTo(HaveOccurred(), "Failed to get Azure Blob Container properties")
	Expect(blobContainer).NotTo(BeNil(), "Azure Blob Container should exist")
	Expect(blobContainer.ContainerProperties).NotTo(BeNil(), "Blob Container properties should not be nil")
}

func verifyBackupBucketDeleted(ctx context.Context, clientSet *azureClientSet, backupBucket *extensionsv1alpha1.BackupBucket) {
	storageAccountName := azurebackupbucket.GenerateStorageAccountName(backupBucket.Name)
	containerName := backupBucket.Name
	log.Info("Verifying backupBucket deletion on Azure", "storageAccountName", storageAccountName, "containerName", containerName)

	By("verifying Azure blob container deletion")
	_, err := clientSet.blobContainers.Get(ctx, backupBucket.Spec.Region, storageAccountName, containerName, nil)
	Expect(err).To(HaveOccurred(), "Expected blob container to be deleted, but it still exists")

	By("verifying Azure storage account deletion")
	_, err = clientSet.storageAccounts.GetProperties(ctx, backupBucket.Spec.Region, storageAccountName, nil)
	Expect(err).To(HaveOccurred(), "Expected storage account to be deleted, but it still exists")
}

func newBackupBucket(name, region string, providerConfig *azurev1alpha1.BackupBucketConfig) *extensionsv1alpha1.BackupBucket {
	var providerConfigRaw *runtime.RawExtension
	if providerConfig != nil {
		providerConfigJSON, err := json.Marshal(providerConfig)
		if err != nil {
			panic(fmt.Sprintf("Failed to serialize providerConfig: %v", err))
		}
		providerConfigRaw = &runtime.RawExtension{
			Raw: providerConfigJSON,
		}
		log.Info("Creating new backupBucket object", "region", region, "providerConfig", string(providerConfigJSON))
	} else {
		providerConfigRaw = &runtime.RawExtension{}
		log.Info("Creating new backupBucket object with empty providerConfig", "region", region)
	}

	return &extensionsv1alpha1.BackupBucket{
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
				Name:      "backupbucket",
				Namespace: testName,
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
