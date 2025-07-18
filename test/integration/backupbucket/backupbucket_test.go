// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket_test

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"time"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
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

type TestContext struct {
	ctx                   context.Context
	client                client.Client
	azClientSet           *azureClientSet
	testNamespace         *corev1.Namespace
	testName              string
	secret                *corev1.Secret
	gardenNamespace       *corev1.Namespace
	gardenNamespaceExists bool
}

var (
	log       logr.Logger
	testEnv   *envtest.Environment
	mgrCancel context.CancelFunc
	tc        *TestContext // TestContext instance

	// Flag variables
	clientId           = flag.String("client-id", "", "Azure client ID")
	clientSecret       = flag.String("client-secret", "", "Azure client secret")
	subscriptionId     = flag.String("subscription-id", "", "Azure subscription ID")
	tenantId           = flag.String("tenant-id", "", "Azure tenant ID")
	region             = flag.String("region", "", "Azure region")
	logLevel           = flag.String("log-level", "", "Log level (debug, info, error)")
	useExistingCluster = flag.Bool("use-existing-cluster", true, "Set to true to use an existing cluster for the test")
)

const (
	backupBucketSecretName = "backupbucket"
	gardenNamespaceName    = "garden"
)

var runTest = func(tc *TestContext, backupBucket *extensionsv1alpha1.BackupBucket, verifyFuncs ...func()) {
	log.Info("Running BackupBucket test", "backupBucketName", backupBucket.Name)

	By("creating backupbucket")
	createBackupBucket(tc.ctx, tc.client, backupBucket)

	defer func() {
		By("deleting backupbucket")
		deleteBackupBucket(tc.ctx, tc.client, backupBucket)

		By("waiting until backupbucket is deleted")
		waitUntilBackupBucketDeleted(tc.ctx, tc.client, backupBucket)

		By("verifying that the Azure storage account and container do not exist")
		verifyBackupBucketDeleted(tc.ctx, tc.azClientSet, backupBucket)
	}()

	By("waiting until backupbucket is ready")
	waitUntilBackupBucketReady(tc.ctx, tc.client, backupBucket)

	// Execute any additional verification functions passed to the test
	for _, verifyFunc := range verifyFuncs {
		verifyFunc()
	}

	log.Info("BackupBucket test completed successfully", "backupBucketName", backupBucket.Name)
}

var _ = BeforeSuite(func() {
	ctx := context.Background()

	By("enabling the EnableImmutableBuckets feature gate")
	enableFeatureGate()

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

		By("deleting azure provider secret")
		if tc != nil && tc.client != nil && tc.secret != nil {
			deleteBackupBucketSecret(tc.ctx, tc.client, tc.secret)
		}

		By("deleting Azure resource group")
		if tc != nil && tc.azClientSet != nil && tc.testName != "" {
			deleteResourceGroup(tc.ctx, tc.azClientSet, tc.testName)
		}

		By("deleting namespaces")
		if tc != nil && tc.client != nil && tc.testNamespace != nil {
			deleteNamespace(tc.ctx, tc.client, tc.testNamespace)
			if !tc.gardenNamespaceExists {
				deleteNamespace(tc.ctx, tc.client, tc.gardenNamespace)
			}
		}

		By("stopping test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	By("generating randomized backupbucket test id")
	// adding '-it-' (integration test) to the name to make it trackable in the Azure Portal
	// '-it--' with '--' not allowed in the Azure Blob Container names
	testName := fmt.Sprintf("azure-backupbucket-it-%s", randomString())

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
	Expect(err).ToNot(HaveOccurred(), "Failed to start the test environment")
	Expect(cfg).ToNot(BeNil(), "Test environment configuration is nil")
	log.Info("Test environment started successfully", "useExistingCluster", *useExistingCluster)

	By("setting up manager")
	mgr, err := manager.New(cfg, manager.Options{
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
	})
	Expect(err).ToNot(HaveOccurred(), "Failed to create manager for the test environment")

	Expect(extensionsv1alpha1.AddToScheme(mgr.GetScheme())).To(Succeed(), "Failed to add extensionsv1alpha1 scheme to manager")
	Expect(azureinstall.AddToScheme(mgr.GetScheme())).To(Succeed(), "Failed to add Azure scheme to manager")

	Expect(backupbucketctrl.AddToManagerWithOptions(ctx, mgr, backupbucketctrl.AddOptions{})).To(Succeed(), "Failed to add BackupBucket controller to manager")

	var mgrContext context.Context
	mgrContext, mgrCancel = context.WithCancel(ctx)

	By("starting manager")
	go func() {
		defer GinkgoRecover()
		err := mgr.Start(mgrContext)
		Expect(err).NotTo(HaveOccurred(), "Failed to start the manager")
	}()

	By("getting clients")
	c, err := client.New(cfg, client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	Expect(err).NotTo(HaveOccurred(), "Failed to create client for the test environment")
	Expect(c).NotTo(BeNil(), "Client for the test environment is nil")

	azClientSet, err := newAzureClientSet(*subscriptionId, *tenantId, *clientId, *clientSecret)
	Expect(err).NotTo(HaveOccurred(), "Failed to create Azure client set")

	By("creating test namespace")
	testNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testName,
		},
	}
	createNamespace(ctx, c, testNamespace)

	By("ensuring garden namespace exists")
	gardenNamespace, gardenNamespaceExists := ensureGardenNamespace(ctx, c)

	By("creating azure provider secret")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupBucketSecretName,
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

	By("creating Azure resource group")
	createResourceGroup(ctx, azClientSet, testName, *region)

	// Initialize the TestContext
	tc = &TestContext{
		ctx:                   ctx,
		client:                c,
		azClientSet:           azClientSet,
		testNamespace:         testNamespace,
		testName:              testName,
		secret:                secret,
		gardenNamespace:       gardenNamespace,
		gardenNamespaceExists: gardenNamespaceExists,
	}
})

var _ = Describe("BackupBucket tests", func() {
	Context("when a BackupBucket is created with basic configuration", func() {
		It("should successfully create and delete a backupbucket", func() {
			providerConfig := &azurev1alpha1.BackupBucketConfig{}
			backupBucket := newBackupBucket(tc.testName, *region, providerConfig)
			runTest(tc, backupBucket, func() {
				verifyBackupBucketAndStatus(tc.ctx, tc.client, tc.azClientSet, backupBucket)
			})
		})
	})

	Context("when a BackupBucket is created with immutability configuration", func() {
		It("should successfully create and delete a backupbucket with immutability enabled", func() {
			providerConfig := &azurev1alpha1.BackupBucketConfig{
				Immutability: &azurev1alpha1.ImmutableConfig{
					RetentionType:   azurev1alpha1.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 24 * time.Hour},
					Locked:          false,
				},
			}
			backupBucket := newBackupBucket(tc.testName, *region, providerConfig)
			runTest(tc, backupBucket, func() {
				By("verifying immutability policy on Azure")
				verifyImmutabilityPolicy(tc.ctx, tc.azClientSet, backupBucket, providerConfig.Immutability)
			})
		})

		It("should ensure immutability of objects stored in the bucket", func() {
			providerConfig := &azurev1alpha1.BackupBucketConfig{
				Immutability: &azurev1alpha1.ImmutableConfig{
					RetentionType:   azurev1alpha1.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 24 * time.Hour},
					Locked:          false,
				},
			}
			backupBucket := newBackupBucket(tc.testName, *region, providerConfig)
			runTest(tc, backupBucket, func() {
				By("writing an object to the bucket and verifying immutability")
				verifyContainerImmutability(tc.ctx, tc.client, tc.azClientSet, backupBucket)
			})
		})

		It("should fail to modify or remove a locked immutability policy", func() {
			providerConfig := &azurev1alpha1.BackupBucketConfig{
				Immutability: &azurev1alpha1.ImmutableConfig{
					RetentionType:   azurev1alpha1.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 24 * time.Hour},
					Locked:          true,
				},
			}
			backupBucket := newBackupBucket(tc.testName, *region, providerConfig)
			runTest(tc, backupBucket, func() {
				By("verifying error when attempting to delete or modify locked immutability policy")
				verifyLockedImmutabilityPolicy(tc.ctx, tc.azClientSet, backupBucket)
			})
		})
	})

	Context("when a BackupBucket is created with key rotation configuration", func() {
		It("should successfully create and delete a backupbucket with key rotation enabled", func() {
			providerConfig := &azurev1alpha1.BackupBucketConfig{
				RotationConfig: &azurev1alpha1.RotationConfig{
					RotationPeriodDays:   2,
					ExpirationPeriodDays: ptr.To[int32](10),
				},
			}
			backupBucket := newBackupBucket(tc.testName, *region, providerConfig)
			runTest(tc, backupBucket, func() {
				By("verifying key rotation policy on Azure")
				verifyKeyRotationPolicy(tc.ctx, tc.azClientSet, backupBucket, providerConfig.RotationConfig)
			})
		})

		It("should successfully rotate keys with added key rotate annotation", func() {
			providerConfig := &azurev1alpha1.BackupBucketConfig{
				RotationConfig: &azurev1alpha1.RotationConfig{
					RotationPeriodDays:   2,
					ExpirationPeriodDays: ptr.To[int32](10),
				},
			}
			backupBucket := newBackupBucket(tc.testName, *region, providerConfig)
			runTest(tc, backupBucket, func() {
				By("verifying key rotation initiated by annotation")
				verifyKeyRotation(tc.ctx, tc.client, tc.azClientSet, backupBucket)
			})
		})
	})

	Context("when a BackupBucket is created with immutability and key rotation configuration", func() {
		It("should successfully create and delete a backupbucket with both immutability and key rotation enabled", func() {
			providerConfig := &azurev1alpha1.BackupBucketConfig{
				Immutability: &azurev1alpha1.ImmutableConfig{
					RetentionType:   azurev1alpha1.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 24 * time.Hour},
					Locked:          false,
				},
				RotationConfig: &azurev1alpha1.RotationConfig{
					RotationPeriodDays:   2,
					ExpirationPeriodDays: ptr.To[int32](10),
				},
			}
			backupBucket := newBackupBucket(tc.testName, *region, providerConfig)
			runTest(tc, backupBucket, func() {
				By("verifying both immutability and key rotation policies on Azure")
				verifyImmutabilityAndKeyRotation(tc.ctx, tc.client, tc.azClientSet, backupBucket, providerConfig)
			})
		})
	})
})
