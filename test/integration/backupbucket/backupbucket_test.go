// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/test/framework"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	azureinstall "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/install"
	azurev1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	backupbucketctrl "github.com/gardener/gardener-extension-provider-azure/pkg/controller/backupbucket"
)

var (
	clientId           = flag.String("client-id", "", "Azure client ID")
	clientSecret       = flag.String("client-secret", "", "Azure client secret")
	subscriptionId     = flag.String("subscription-id", "", "Azure subscription ID")
	tenantId           = flag.String("tenant-id", "", "Azure tenant ID")
	region             = flag.String("region", "", "Azure region")
	logLevel           = flag.String("log-level", "", "Log level (debug, info, error)")
	useExistingCluster = flag.Bool("use-existing-cluster", true, "Set to true to use an existing cluster for the test")
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

var (
	ctx = context.Background()

	log                          logr.Logger
	azClientSet                  *azureClientSet
	testEnv                      *envtest.Environment
	mgrCancel                    context.CancelFunc
	c                            client.Client
	secret                       *corev1.Secret
	testNamespace                *corev1.Namespace
	gardenNamespace              *corev1.Namespace
	gardenNamespaceAlreadyExists bool

	testName string
)

var runTest = func(backupBucket *extensionsv1alpha1.BackupBucket) {
	log.Info("Running BackupBucket test", "backupBucketName", backupBucket.Name)

	By("creating backupbucket")
	createBackupBucket(ctx, c, backupBucket)

	defer func() {
		By("deleting backupbucket")
		deleteBackupBucket(ctx, c, backupBucket)

		By("waiting until backupbucket is deleted")
		waitUntilBackupBucketDeleted(ctx, c, log, backupBucket)

		By("verifying that the Azure storage account and container do not exist")
		verifyBackupBucketDeleted(ctx, azClientSet, backupBucket)
	}()

	By("waiting until backupbucket is ready")
	waitUntilBackupBucketReady(ctx, c, log, backupBucket)

	By("getting backupbucket and verifying its status")
	getBackupBucketAndVerifyStatus(ctx, c, backupBucket)

	By("verifying that the Azure storage account and container exist and match backupbucket")
	verifyBackupBucket(ctx, azClientSet, backupBucket)

	log.Info("BackupBucket test completed successfully", "backupBucketName", backupBucket.Name)
}

var _ = BeforeSuite(func() {
	repoRoot := filepath.Join("..", "..", "..")

	flag.Parse()
	secretsFromEnv()
	validateFlags()

	logf.SetLogger(logger.MustNewZapLogger(*logLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName("backupbucket-test")
	log.Info("Starting BackupBucket test", "logLevel", *logLevel)

	DeferCleanup(func() {
		By("stopping manager")
		mgrCancel()

		By("running cleanup actions")
		framework.RunCleanupActions()

		By("deleting azure provider secret")
		deleteBackupBucketSecret(ctx, c, secret)

		By("deleting Azure resource group")
		deleteResourceGroup(ctx, azClientSet, testName)

		By("deleting namespaces")
		deleteNamespace(ctx, c, testNamespace)
		if !gardenNamespaceAlreadyExists {
			deleteNamespace(ctx, c, gardenNamespace)
		}

		By("stopping test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	By("generating randomized backupbucket test id")
	// adding '-it-' (integration test) to the name to make it trackable in the Azure Portal
	// '-it--' with '--' not allowed in the Azure Blob Container names
	testName = fmt.Sprintf("azure-backupbucket-it-%s", randomString())

	By("starting test environment")
	testEnv = &envtest.Environment{
		UseExistingCluster: ptr.To(*useExistingCluster),
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_backupbuckets.yaml"),
			},
		},
		ControlPlaneStopTimeout: 2 * time.Minute,
	}

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())
	log.Info("Test environment started successfully", "useExistingCluster", *useExistingCluster)

	By("setting up manager")
	mgr, err := manager.New(cfg, manager.Options{
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
	})
	Expect(err).ToNot(HaveOccurred())

	Expect(extensionsv1alpha1.AddToScheme(mgr.GetScheme())).To(Succeed())
	Expect(azureinstall.AddToScheme(mgr.GetScheme())).To(Succeed())

	Expect(backupbucketctrl.AddToManagerWithOptions(ctx, mgr, backupbucketctrl.AddOptions{})).To(Succeed())

	var mgrContext context.Context
	mgrContext, mgrCancel = context.WithCancel(ctx)

	By("starting manager")
	go func() {
		defer GinkgoRecover()
		err := mgr.Start(mgrContext)
		Expect(err).NotTo(HaveOccurred())
	}()

	By("getting clients")
	c, err = client.New(cfg, client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(c).NotTo(BeNil())

	azClientSet, err = newAzureClientSet(*subscriptionId, *tenantId, *clientId, *clientSecret)
	Expect(err).NotTo(HaveOccurred())

	By("creating test namespace")
	testNamespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testName,
		},
	}
	createNamespace(ctx, c, testNamespace)

	By("ensuring garden namespace exists")
	ensureGardenNamespace(ctx, c, log)

	By("creating azure provider secret")
	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backupbucket",
			Namespace: testName,
		},
		Data: map[string][]byte{
			azure.SubscriptionIDKey: []byte(*subscriptionId),
			azure.TenantIDKey:       []byte(*tenantId),
			azure.ClientIDKey:       []byte(*clientId),
			azure.ClientSecretKey:   []byte(*clientSecret),
		},
	}
	createBackupBucketSecret(ctx, c, secret)

	By("creating Azure resource group ")
	createResourceGroup(ctx, azClientSet, testName, *region)
})

var _ = Describe("BackupBucket tests", func() {
	Context("when a BackupBucket is created with basic configuration", func() {
		It("should successfully create and delete a backupbucket", func() {
			providerConfig := &azurev1alpha1.BackupBucketConfig{}
			backupBucket := newBackupBucket(testName, *region, providerConfig)
			runTest(backupBucket)
		})
	})
})
