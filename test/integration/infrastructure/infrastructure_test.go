// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure_test

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/msi/armmsi"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	azureRest "github.com/Azure/go-autorest/autorest/azure"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/test/framework"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/uuid"
	schemev1 "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	azureinstall "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/install"
	azurev1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure"
	. "github.com/gardener/gardener-extension-provider-azure/test/integration/infrastructure"
)

const (
	CountDomain                = 1
	reconcilerUseTF     string = "tf"
	reconcilerMigrateTF string = "migrate"
	reconcilerUseFlow   string = "flow"
)

var (
	VNetCIDR   = "10.250.0.0/16"
	WorkerCIDR = "10.250.0.0/19"
)

var (
	clientId       = flag.String("client-id", "", "Azure client ID")
	clientSecret   = flag.String("client-secret", "", "Azure client secret")
	subscriptionId = flag.String("subscription-id", "", "Azure subscription ID")
	tenantId       = flag.String("tenant-id", "", "Azure tenant ID")
	region         = flag.String("region", "", "Azure region")
	secretYamlPath = flag.String("secret-path", "", "Yaml file with secret including Azure credentials")
	reconciler     = flag.String("reconciler", reconcilerUseTF, "Set annotation to use flow for reconciliation")

	testId = string(uuid.NewUUID())
)

type azureClientSet struct {
	groups           *armresources.ResourceGroupsClient
	vnet             *armnetwork.VirtualNetworksClient
	subnets          *armnetwork.SubnetsClient
	interfaces       *armnetwork.InterfacesClient
	securityGroups   *armnetwork.SecurityGroupsClient
	availabilitySets *armcompute.AvailabilitySetsClient
	routeTable       *armnetwork.RouteTablesClient
	nat              *armnetwork.NatGatewaysClient
	pubIp            *armnetwork.PublicIPAddressesClient
	msi              *armmsi.UserAssignedIdentitiesClient
}

type pubIpRef struct {
	Name          string
	ResourceGroup string
}

func newAzureClientSet(subscriptionId, tenantId, clientId, clientSecret string) (*azureClientSet, error) {
	credential, err := azidentity.NewClientSecretCredential(tenantId, clientId, clientSecret, nil)
	if err != nil {
		return nil, err
	}
	groupsClient, err := armresources.NewResourceGroupsClient(subscriptionId, credential, nil)
	if err != nil {
		return nil, err
	}

	vnetClient, err := armnetwork.NewVirtualNetworksClient(subscriptionId, credential, nil)
	if err != nil {
		return nil, err
	}
	subnetClient, err := armnetwork.NewSubnetsClient(subscriptionId, credential, nil)
	if err != nil {
		return nil, err
	}

	interfacesClient, err := armnetwork.NewInterfacesClient(subscriptionId, credential, nil)
	if err != nil {
		return nil, err
	}

	securityGroupsClient, err := armnetwork.NewSecurityGroupsClient(subscriptionId, credential, nil)
	if err != nil {
		return nil, err
	}

	availabilitySetsClient, err := armcompute.NewAvailabilitySetsClient(subscriptionId, credential, nil)
	if err != nil {
		return nil, err
	}

	tablesClient, err := armnetwork.NewRouteTablesClient(subscriptionId, credential, nil)
	if err != nil {
		return nil, err
	}

	natClient, err := armnetwork.NewNatGatewaysClient(subscriptionId, credential, nil)
	if err != nil {
		return nil, err
	}

	pubIpClient, err := armnetwork.NewPublicIPAddressesClient(subscriptionId, credential, nil)
	if err != nil {
		return nil, err
	}

	msiClient, err := armmsi.NewUserAssignedIdentitiesClient(subscriptionId, credential, nil)
	if err != nil {
		return nil, err
	}

	return &azureClientSet{
		groups:           groupsClient,
		vnet:             vnetClient,
		subnets:          subnetClient,
		interfaces:       interfacesClient,
		securityGroups:   securityGroupsClient,
		availabilitySets: availabilitySetsClient,
		routeTable:       tablesClient,
		nat:              natClient,
		pubIp:            pubIpClient,
		msi:              msiClient,
	}, nil
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

type azureIdentifier struct {
	resourceGroup     string
	vnetResourceGroup *string
	vnet              *string
	subnets           []string
}

var (
	ctx = context.Background()
	log logr.Logger

	testEnv   *envtest.Environment
	c         client.Client
	mgrCancel context.CancelFunc
	decoder   runtime.Decoder

	clientSet *azureClientSet
)

var _ = BeforeSuite(func() {
	flag.Parse()
	validateFlags()

	repoRoot := filepath.Join("..", "..", "..")

	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))

	log = logf.Log.WithName("infrastructure-test")

	DeferCleanup(func() {
		defer func() {
			By("stopping manager")
			mgrCancel()
		}()

		By("running cleanup actions")
		framework.RunCleanupActions()

		By("stopping test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	By("starting test environment")
	testEnv = &envtest.Environment{
		UseExistingCluster: ptr.To(true),
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_clusters.yaml"),
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_infrastructures.yaml"),
			},
		},
	}

	restConfig, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())

	httpClient, err := rest.HTTPClientFor(restConfig)
	Expect(err).NotTo(HaveOccurred())
	mapper, err := apiutil.NewDynamicRESTMapper(restConfig, httpClient)
	Expect(err).NotTo(HaveOccurred())

	scheme := runtime.NewScheme()
	Expect(extensionsv1alpha1.AddToScheme(scheme)).To(Succeed())
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: "0",
		},
		Cache: cache.Options{
			Mapper: mapper,
			ByObject: map[client.Object]cache.ByObject{
				&extensionsv1alpha1.Infrastructure{}: {
					Label: labels.SelectorFromSet(labels.Set{"test-id": testId}),
				},
			},
		},
	})
	Expect(err).ToNot(HaveOccurred())

	Expect(schemev1.AddToScheme(mgr.GetScheme())).To(Succeed())
	Expect(extensionsv1alpha1.AddToScheme(mgr.GetScheme())).To(Succeed())
	Expect(azureinstall.AddToScheme(mgr.GetScheme())).To(Succeed())

	Expect(infrastructure.AddToManagerWithOptions(ctx, mgr, infrastructure.AddOptions{
		// During testing in testmachinery cluster, there is no gardener-resource-manager to inject the volume mount.
		// Hence, we need to run without projected token mount.
		DisableProjectedTokenMount: true,
	})).To(Succeed())

	var mgrContext context.Context
	mgrContext, mgrCancel = context.WithCancel(ctx)

	By("start manager")
	go func() {
		err := mgr.Start(mgrContext)
		Expect(err).ToNot(HaveOccurred())
	}()

	c = mgr.GetClient()
	Expect(c).ToNot(BeNil())
	decoder = serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder()

	clientSet, err = newAzureClientSet(*subscriptionId, *tenantId, *clientId, *clientSecret)
	Expect(err).ToNot(HaveOccurred())

	priorityClass := &schedulingv1.PriorityClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1beta1constants.PriorityClassNameShootControlPlane300,
		},
		Description:   "PriorityClass for Shoot control plane components",
		GlobalDefault: false,
		Value:         999998300,
	}
	Expect(client.IgnoreAlreadyExists(c.Create(ctx, priorityClass))).To(BeNil())
})

var _ = Describe("Infrastructure tests", func() {
	Context("AvailabilitySet cluster", func() {
		AfterEach(func() {
			framework.RunCleanupActions()
		})

		It("should successfully create and delete AvailabilitySet cluster creating new vNet", func() {
			providerConfig := newInfrastructureConfig(nil, nil, nil, false)

			namespace, err := generateName()
			Expect(err).ToNot(HaveOccurred())

			err = runTest(ctx, log, c, clientSet, namespace, providerConfig, false, decoder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should successfully create and delete AvailabilitySet cluster using existing vNet and existing identity", func() {
			foreignName, err := generateName()
			Expect(err).ToNot(HaveOccurred())
			foreignNameVnet := foreignName + "-vnet"
			foreignNameId := foreignName + "-id"

			var cleanupHandle framework.CleanupActionHandle
			cleanupHandle = framework.AddCleanupAction(func() {
				Expect(ignoreAzureNotFoundError(teardownResourceGroup(ctx, clientSet, foreignName))).To(Succeed())
				framework.RemoveCleanupAction(cleanupHandle)
			})

			Expect(prepareNewResourceGroup(ctx, log, clientSet, foreignName, *region)).To(Succeed())
			Expect(prepareNewVNet(ctx, log, clientSet, foreignName, foreignNameVnet, *region, VNetCIDR)).To(Succeed())
			Expect(prepareNewIdentity(ctx, log, clientSet, foreignName, foreignNameId, *region)).To(Succeed())

			vnetConfig := &azurev1alpha1.VNet{
				Name:          ptr.To(foreignNameVnet),
				ResourceGroup: ptr.To(foreignName),
			}
			identityConfig := &azurev1alpha1.IdentityConfig{
				Name:          foreignNameId,
				ResourceGroup: foreignName,
			}
			providerConfig := newInfrastructureConfig(vnetConfig, nil, identityConfig, false)

			namespace, err := generateName()
			Expect(err).ToNot(HaveOccurred())
			err = runTest(ctx, log, c, clientSet, namespace, providerConfig, false, decoder)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Zonal cluster", func() {
		AfterEach(func() {
			framework.RunCleanupActions()
		})

		It("should successfully create and delete a zonal cluster without NatGateway creating new vNet", func() {
			providerConfig := newInfrastructureConfig(nil, nil, nil, true)

			namespace, err := generateName()
			Expect(err).ToNot(HaveOccurred())

			err = runTest(ctx, log, c, clientSet, namespace, providerConfig, false, decoder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should successfully create and delete a zonal cluster with NatGateway using an existing vNet and identity", func() {
			foreignName, err := generateName()
			Expect(err).ToNot(HaveOccurred())
			foreignNameVnet := foreignName + "-vnet"
			foreignNameId := foreignName + "-id"

			var cleanupHandle framework.CleanupActionHandle
			cleanupHandle = framework.AddCleanupAction(func() {
				Expect(ignoreAzureNotFoundError(teardownResourceGroup(ctx, clientSet, foreignName))).To(Succeed())
				framework.RemoveCleanupAction(cleanupHandle)
			})

			Expect(prepareNewResourceGroup(ctx, log, clientSet, foreignName, *region)).To(Succeed())
			Expect(prepareNewVNet(ctx, log, clientSet, foreignName, foreignNameVnet, *region, VNetCIDR)).To(Succeed())
			Expect(prepareNewIdentity(ctx, log, clientSet, foreignName, foreignNameId, *region)).To(Succeed())

			vnetConfig := &azurev1alpha1.VNet{
				Name:          ptr.To(foreignNameVnet),
				ResourceGroup: ptr.To(foreignName),
			}
			identityConfig := &azurev1alpha1.IdentityConfig{
				Name:          foreignNameId,
				ResourceGroup: foreignName,
			}
			natGatewayConfig := &azurev1alpha1.NatGatewayConfig{
				Enabled: true,
			}
			providerConfig := newInfrastructureConfig(vnetConfig, natGatewayConfig, identityConfig, true)

			namespace, err := generateName()
			Expect(err).ToNot(HaveOccurred())
			err = runTest(ctx, log, c, clientSet, namespace, providerConfig, false, decoder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should successfully create and delete a zonal cluster with Nat Gateway using user provided public IPs", func() {
			foreignName, err := generateName()
			Expect(err).ToNot(HaveOccurred())

			var cleanupHandle framework.CleanupActionHandle
			cleanupHandle = framework.AddCleanupAction(func() {
				Expect(ignoreAzureNotFoundError(teardownResourceGroup(ctx, clientSet, foreignName))).To(Succeed())
				framework.RemoveCleanupAction(cleanupHandle)
			})

			var (
				zone       int32 = 1
				natIPName1       = "nat-ip-1"
				natIPName2       = "nat-ip-2"
			)

			Expect(prepareNewResourceGroup(ctx, log, clientSet, foreignName, *region)).To(Succeed())
			Expect(prepareNewNatIp(ctx, log, clientSet, foreignName, natIPName1, *region, fmt.Sprintf("%d", zone))).To(Succeed())
			Expect(prepareNewNatIp(ctx, log, clientSet, foreignName, natIPName2, *region, fmt.Sprintf("%d", zone))).To(Succeed())

			natGatewayConfig := &azurev1alpha1.NatGatewayConfig{
				Enabled: true,
				Zone:    &zone,
				IPAddresses: []azurev1alpha1.PublicIPReference{
					{
						Name:          natIPName1,
						ResourceGroup: foreignName,
						Zone:          zone,
					},
					{
						Name:          natIPName2,
						ResourceGroup: foreignName,
						Zone:          zone,
					},
				},
			}

			providerConfig := newInfrastructureConfig(nil, natGatewayConfig, nil, true)

			namespace, err := generateName()
			Expect(err).ToNot(HaveOccurred())
			err = runTest(ctx, log, c, clientSet, namespace, providerConfig, false, decoder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should successfully create a multi zonal NAT Gateway cluster", func() {
			var (
				zone1 int32 = 1
				zone2 int32 = 2
			)
			providerConfig := &azurev1alpha1.InfrastructureConfig{
				TypeMeta: metav1.TypeMeta{
					APIVersion: azurev1alpha1.SchemeGroupVersion.String(),
					Kind:       "InfrastructureConfig",
				},
				Networks: azurev1alpha1.NetworkConfig{
					VNet: azurev1alpha1.VNet{
						CIDR: &VNetCIDR,
					},
					Zones: []azurev1alpha1.Zone{
						{
							Name:             zone1,
							CIDR:             "10.250.0.0/24",
							ServiceEndpoints: []string{"Microsoft.Storage"},
							NatGateway: &azurev1alpha1.ZonedNatGatewayConfig{
								Enabled: true,
							},
						},
						{
							Name:             zone2,
							CIDR:             "10.250.1.0/24",
							ServiceEndpoints: []string{"Microsoft.Storage"},
							NatGateway: &azurev1alpha1.ZonedNatGatewayConfig{
								Enabled: true,
							},
						},
					},
				},
				Zoned: true,
			}

			namespace, err := generateName()
			Expect(err).ToNot(HaveOccurred())

			err = runTest(ctx, log, c, clientSet, namespace, providerConfig, false, decoder)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("VMO cluster", func() {
		AfterEach(func() {
			framework.RunCleanupActions()
		})

		It("should successfully create and delete VMO cluster without NatGateway creating new vNet", func() {
			providerConfig := newInfrastructureConfig(nil, nil, nil, false)

			namespace, err := generateName()
			Expect(err).ToNot(HaveOccurred())

			err = runTest(ctx, log, c, clientSet, namespace, providerConfig, true, decoder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should successfully create and delete VMO cluster with NatGateway using an existing vNet and identity", func() {
			foreignName, err := generateName()
			Expect(err).ToNot(HaveOccurred())
			foreignNameVnet := foreignName + "-vnet"
			foreignNameId := foreignName + "-id"

			var cleanupHandle framework.CleanupActionHandle
			cleanupHandle = framework.AddCleanupAction(func() {
				Expect(ignoreAzureNotFoundError(teardownResourceGroup(ctx, clientSet, foreignName))).To(Succeed())
				framework.RemoveCleanupAction(cleanupHandle)
			})

			Expect(prepareNewResourceGroup(ctx, log, clientSet, foreignName, *region)).To(Succeed())
			Expect(prepareNewVNet(ctx, log, clientSet, foreignName, foreignNameVnet, *region, VNetCIDR)).To(Succeed())
			Expect(prepareNewIdentity(ctx, log, clientSet, foreignName, foreignNameId, *region)).To(Succeed())

			vnetConfig := &azurev1alpha1.VNet{
				Name:          ptr.To(foreignNameVnet),
				ResourceGroup: ptr.To(foreignName),
			}
			identityConfig := &azurev1alpha1.IdentityConfig{
				Name:          foreignNameId,
				ResourceGroup: foreignName,
			}
			natGatewayConfig := &azurev1alpha1.NatGatewayConfig{
				Enabled: true,
			}
			providerConfig := newInfrastructureConfig(vnetConfig, natGatewayConfig, identityConfig, false)

			namespace, err := generateName()
			Expect(err).ToNot(HaveOccurred())
			err = runTest(ctx, log, c, clientSet, namespace, providerConfig, true, decoder)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("with invalid credentials", func() {
		It("should fail creation but succeed deletion", func() {
			namespaceName, err := generateName()
			Expect(err).ToNot(HaveOccurred())

			providerConfig := newInfrastructureConfig(nil, nil, nil, false)

			var (
				namespace *corev1.Namespace
				cluster   *extensionsv1alpha1.Cluster
				infra     *extensionsv1alpha1.Infrastructure
			)

			framework.AddCleanupAction(func() {
				By("cleaning up namespace and cluster")
				Expect(client.IgnoreNotFound(c.Delete(ctx, namespace))).To(Succeed())
				Expect(client.IgnoreNotFound(c.Delete(ctx, cluster))).To(Succeed())
			})

			defer func() {
				By("delete infrastructure")
				Expect(client.IgnoreNotFound(c.Delete(ctx, infra))).To(Succeed())

				By("wait until infrastructure is deleted")
				err := extensions.WaitUntilExtensionObjectDeleted(
					ctx,
					c,
					log,
					infra,
					extensionsv1alpha1.InfrastructureResource,
					10*time.Second,
					16*time.Minute,
				)
				Expect(err).ToNot(HaveOccurred())
			}()

			By("create namespace for test execution")
			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
				},
			}
			Expect(c.Create(ctx, namespace)).To(Succeed())

			By("create cluster CR")
			cluster, err = newCluster(namespaceName, *region, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(c.Create(ctx, cluster)).To(Succeed())

			By("deploy invalid cloudprovider secret into namespace")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      v1beta1constants.SecretNameCloudProvider,
					Namespace: namespaceName,
				},
				Data: map[string][]byte{
					azure.SubscriptionIDKey: []byte(*subscriptionId),
					azure.TenantIDKey:       []byte(*tenantId),
					azure.ClientIDKey:       []byte(*clientId),
					azure.ClientSecretKey:   []byte("fake"),
				},
			}
			Expect(c.Create(ctx, secret)).To(Succeed())

			By("create infrastructure")
			infra, err = newInfrastructure(namespaceName, providerConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(c.Create(ctx, infra)).To(Succeed())

			By("wait until infrastructure creation has failed")
			err = extensions.WaitUntilExtensionObjectReady(
				ctx,
				c,
				log,
				infra,
				extensionsv1alpha1.InfrastructureResource,
				10*time.Second,
				30*time.Second,
				60*time.Second,
				nil,
			)
			var errorWithCode *gardencorev1beta1helper.ErrorWithCodes
			Expect(errors.As(err, &errorWithCode)).To(BeTrue())
			Expect(errorWithCode.Codes()).To(Or(
				ContainElement(gardencorev1beta1.ErrorInfraUnauthorized),
				ContainElement(gardencorev1beta1.ErrorInfraUnauthenticated)))
		})
	})
})

func runTest(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	az *azureClientSet,
	namespaceName string,
	providerConfig *azurev1alpha1.InfrastructureConfig,
	setVmoAnnotationToShoot bool,
	decoder runtime.Decoder,
) error {
	var (
		namespace  *corev1.Namespace
		cluster    *extensionsv1alpha1.Cluster
		infra      *extensionsv1alpha1.Infrastructure
		identifier azureIdentifier
	)
	log.Info("test running in namespace", "namespaceName", namespaceName)

	// Cleanup
	defer func() {
		By("delete infrastructure")
		Expect(client.IgnoreNotFound(c.Delete(ctx, infra))).To(Succeed())

		By("wait until infrastructure is deleted")
		err := extensions.WaitUntilExtensionObjectDeleted(
			ctx,
			c,
			log,
			infra,
			extensionsv1alpha1.InfrastructureResource,
			10*time.Second,
			16*time.Minute,
		)
		Expect(err).ToNot(HaveOccurred())

		By("verify infrastructure deletion")
		verifyDeletion(ctx, az, identifier)

		Expect(client.IgnoreNotFound(c.Delete(ctx, namespace))).To(Succeed())
		Expect(client.IgnoreNotFound(c.Delete(ctx, cluster))).To(Succeed())
	}()

	By("create namespace for test execution")
	namespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
	}
	if err := c.Create(ctx, namespace); err != nil {
		return err
	}

	By("create cluster CR")
	cluster, err := newCluster(namespaceName, *region, setVmoAnnotationToShoot)
	if err != nil {
		return err
	}
	if err := c.Create(ctx, cluster); err != nil {
		return err
	}

	By("deploy cloudprovider secret into namespace")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.SecretNameCloudProvider,
			Namespace: namespaceName,
		},
		Data: map[string][]byte{
			azure.SubscriptionIDKey: []byte(*subscriptionId),
			azure.TenantIDKey:       []byte(*tenantId),
			azure.ClientIDKey:       []byte(*clientId),
			azure.ClientSecretKey:   []byte(*clientSecret),
		},
	}
	if err := c.Create(ctx, secret); err != nil {
		return err
	}

	By("create infrastructure")
	infra, err = newInfrastructure(namespaceName, providerConfig)
	if err != nil {
		return err
	}

	if err := c.Create(ctx, infra); err != nil {
		return err
	}

	By("wait until infrastructure is created")
	if err := extensions.WaitUntilExtensionObjectReady(
		ctx,
		c,
		log,
		infra,
		extensionsv1alpha1.InfrastructureResource,
		10*time.Second,
		30*time.Second,
		16*time.Minute,
		nil,
	); err != nil {
		return err
	}

	if *reconciler == reconcilerMigrateTF {
		By("verifying terraform migration")
		infraCopy := infra.DeepCopy()
		metav1.SetMetaDataAnnotation(&infra.ObjectMeta, "gardener.cloud/operation", "reconcile")
		metav1.SetMetaDataAnnotation(&infra.ObjectMeta, azure.AnnotationKeyUseFlow, "true")
		Expect(c.Patch(ctx, infra, client.MergeFrom(infraCopy))).To(Succeed())

		By("wait until infrastructure is reconciled")
		if err := extensions.WaitUntilExtensionObjectReady(
			ctx,
			c,
			log,
			infra,
			"Infrastructure",
			10*time.Second,
			30*time.Second,
			16*time.Minute,
			nil,
		); err != nil {
			return err
		}
	}

	By("decode infrastructure status")
	if err := c.Get(ctx, client.ObjectKey{Namespace: infra.Namespace, Name: infra.Name}, infra); err != nil {
		return err
	}

	providerStatus := &azurev1alpha1.InfrastructureStatus{}
	if _, _, err := decoder.Decode(infra.Status.ProviderStatus.Raw, nil, providerStatus); err != nil {
		return err
	}
	By("verify infrastructure creation")
	identifier = verifyCreation(ctx, az, infra, providerConfig, providerStatus)
	return nil
}

func generateName() (string, error) {
	suffix, err := utils.GenerateRandomStringFromCharset(5, "0123456789abcdefghijklmnopqrstuvwxyz")
	if err != nil {
		return "", err
	}

	return "azure-infrastructure-it--" + suffix, nil
}

func newInfrastructureConfig(vnet *azurev1alpha1.VNet, natGateway *azurev1alpha1.NatGatewayConfig, id *azurev1alpha1.IdentityConfig, zoned bool) *azurev1alpha1.InfrastructureConfig {
	if vnet == nil {
		vnet = &azurev1alpha1.VNet{
			CIDR: ptr.To(VNetCIDR),
		}
	}
	nwConfig := azurev1alpha1.NetworkConfig{
		VNet:             *vnet,
		Workers:          &WorkerCIDR,
		NatGateway:       nil,
		ServiceEndpoints: []string{"Microsoft.Storage"},
	}

	if natGateway != nil {
		nwConfig.NatGateway = natGateway
	}

	return &azurev1alpha1.InfrastructureConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: azurev1alpha1.SchemeGroupVersion.String(),
			Kind:       "InfrastructureConfig",
		},
		Identity: id,
		Networks: nwConfig,
		Zoned:    zoned,
	}
}

func newCluster(name, region string, setVmoAnnotationToShoot bool) (*extensionsv1alpha1.Cluster, error) {
	rawAzureCloudProfileConfig, err := json.Marshal(
		azurev1alpha1.CloudProfileConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: azurev1alpha1.SchemeGroupVersion.String(),
				Kind:       "CloudProfileConfig",
			},
			CountFaultDomains: []azurev1alpha1.DomainCount{
				{
					Region: region,
					Count:  CountDomain,
				},
			},
			CountUpdateDomains: []azurev1alpha1.DomainCount{
				{
					Region: region,
					Count:  CountDomain,
				},
			},
		})
	if err != nil {
		return nil, err
	}

	rawCloudProfile, err := json.Marshal(
		gardencorev1beta1.CloudProfile{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				Kind:       "CloudProfile",
			},
			Spec: gardencorev1beta1.CloudProfileSpec{
				ProviderConfig: &runtime.RawExtension{
					Raw: rawAzureCloudProfileConfig,
				},
			},
		})
	if err != nil {
		return nil, err
	}

	shoot := gardencorev1beta1.Shoot{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
			Kind:       "Shoot",
		},
	}
	if setVmoAnnotationToShoot {
		shoot.Annotations = map[string]string{
			"alpha.azure.provider.extensions.gardener.cloud/vmo": "true",
		}
	}

	rawShoot, err := json.Marshal(shoot)
	if err != nil {
		return nil, err
	}

	return &extensionsv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: extensionsv1alpha1.ClusterSpec{
			CloudProfile: runtime.RawExtension{
				Raw: rawCloudProfile,
			},
			Seed: runtime.RawExtension{
				Raw: []byte("{}"),
			},
			Shoot: runtime.RawExtension{
				Raw: rawShoot,
			},
		},
	}, nil
}

func newInfrastructure(namespace string, providerConfig *azurev1alpha1.InfrastructureConfig) (*extensionsv1alpha1.Infrastructure, error) {
	const sshPublicKey = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDcSZKq0lM9w+ElLp9I9jFvqEFbOV1+iOBX7WEe66GvPLOWl9ul03ecjhOf06+FhPsWFac1yaxo2xj+SJ+FVZ3DdSn4fjTpS9NGyQVPInSZveetRw0TV0rbYCFBTJuVqUFu6yPEgdcWq8dlUjLqnRNwlelHRcJeBfACBZDLNSxjj0oUz7ANRNCEne1ecySwuJUAz3IlNLPXFexRT0alV7Nl9hmJke3dD73nbeGbQtwvtu8GNFEoO4Eu3xOCKsLw6ILLo4FBiFcYQOZqvYZgCb4ncKM52bnABagG54upgBMZBRzOJvWp0ol+jK3Em7Vb6ufDTTVNiQY78U6BAlNZ8Xg+LUVeyk1C6vWjzAQf02eRvMdfnRCFvmwUpzbHWaVMsQm8gf3AgnTUuDR0ev1nQH/5892wZA86uLYW/wLiiSbvQsqtY1jSn9BAGFGdhXgWLAkGsd/E1vOT+vDcor6/6KjHBm0rG697A3TDBRkbXQ/1oFxcM9m17RteCaXuTiAYWMqGKDoJvTMDc4L+Uvy544pEfbOH39zfkIYE76WLAFPFsUWX6lXFjQrX3O7vEV73bCHoJnwzaNd03PSdJOw+LCzrTmxVezwli3F9wUDiBRB0HkQxIXQmncc1HSecCKALkogIK+1e1OumoWh6gPdkF4PlTMUxRitrwPWSaiUIlPfCpQ== your_email@example.com"

	providerConfigJSON, err := json.Marshal(&providerConfig)
	if err != nil {
		return nil, err
	}

	infra := &extensionsv1alpha1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "infrastructure",
			Namespace: namespace,
			Labels: map[string]string{
				"test-id": testId,
			},
		},
		Spec: extensionsv1alpha1.InfrastructureSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: azure.Type,
				ProviderConfig: &runtime.RawExtension{
					Raw: providerConfigJSON,
				},
			},
			SecretRef: corev1.SecretReference{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: namespace,
			},
			Region:       *region,
			SSHPublicKey: []byte(sshPublicKey),
		},
	}

	if *reconciler == reconcilerUseFlow {
		log.Info("creating infrastructure with flow annotation")
		metav1.SetMetaDataAnnotation(&infra.ObjectMeta, azure.AnnotationKeyUseFlow, "true")
	}
	return infra, nil
}

func prepareNewResourceGroup(ctx context.Context, log logr.Logger, az *azureClientSet, groupName, location string) error {
	log.Info("generating new ResourceGroups", "groupName", groupName)
	_, err := az.groups.CreateOrUpdate(ctx, groupName, armresources.ResourceGroup{
		Location: ptr.To(location),
	}, nil)
	return err
}

func prepareNewVNet(ctx context.Context, log logr.Logger, az *azureClientSet, groupName, vNetName, location, cidr string) error {
	log.Info("generating new VNet", "groupName", groupName, "vNetName", vNetName)
	poller, err := az.vnet.BeginCreateOrUpdate(ctx, groupName, vNetName, armnetwork.VirtualNetwork{
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{
				AddressPrefixes: []*string{
					ptr.To(cidr),
				},
			},
		},
		Name:     ptr.To(vNetName),
		Location: ptr.To(location),
	}, nil)
	Expect(err).ShouldNot(HaveOccurred())

	_, err = poller.PollUntilDone(ctx, nil)
	Expect(err).ShouldNot(HaveOccurred())

	return nil
}

func prepareNewIdentity(ctx context.Context, log logr.Logger, az *azureClientSet, groupName, idName, location string) error {
	log.Info("generating new Identity", "groupName", groupName, "idName", idName)
	_, err := az.msi.CreateOrUpdate(ctx, groupName, idName, armmsi.Identity{
		Location: ptr.To(location),
	}, nil)
	return err
}

func prepareNewNatIp(ctx context.Context, log logr.Logger, az *azureClientSet, groupName, pubIpName, location, zone string) error {
	log.Info("generating new nat ip", "groupName", groupName, "pubIpName", pubIpName)
	_, err := az.pubIp.BeginCreateOrUpdate(ctx, groupName, pubIpName, armnetwork.PublicIPAddress{
		Name: ptr.To(pubIpName),
		SKU: &armnetwork.PublicIPAddressSKU{
			Name: ptr.To(armnetwork.PublicIPAddressSKUNameStandard),
		},
		Zones:    []*string{ptr.To(zone)},
		Location: &location,
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAllocationMethod: ptr.To(armnetwork.IPAllocationMethodStatic),
			PublicIPAddressVersion:   ptr.To(armnetwork.IPVersionIPv4),
		},
	}, nil)
	return err
}

func teardownResourceGroup(ctx context.Context, az *azureClientSet, groupName string) error {
	poller, err := az.groups.BeginDelete(ctx, groupName, nil)
	Expect(err).ShouldNot(HaveOccurred())

	_, err = poller.PollUntilDone(ctx, nil)
	Expect(err).ShouldNot(HaveOccurred())

	return nil
}

func hasForeignVNet(config *azurev1alpha1.InfrastructureConfig) bool {
	return config.Networks.VNet.ResourceGroup != nil && config.Networks.VNet.Name != nil
}

func hasDedicatedSubnets(config *azurev1alpha1.InfrastructureConfig) bool {
	return config.Networks.Workers == nil
}

func verifyCreation(
	ctx context.Context,
	az *azureClientSet,
	infra *extensionsv1alpha1.Infrastructure,
	config *azurev1alpha1.InfrastructureConfig,
	status *azurev1alpha1.InfrastructureStatus,
) azureIdentifier {
	var (
		vnetResourceGroupName string
		vnet                  armnetwork.VirtualNetwork
		ng                    armnetwork.NatGateway
		err                   error
		result                = azureIdentifier{}
	)

	// resource groups
	if config.ResourceGroup != nil {
		Expect(status.ResourceGroup.Name).To(Equal(config.ResourceGroup.Name))
	} else {
		Expect(infra.Namespace).To(Equal(status.ResourceGroup.Name))
	}

	group, err := az.groups.Get(ctx, status.ResourceGroup.Name, nil)
	Expect(err).ToNot(HaveOccurred())
	Expect(group.Location).To(PointTo(Equal(*region)))
	Expect(group.Tags).To(BeEmpty())

	result.resourceGroup = status.ResourceGroup.Name

	// VNet
	if hasForeignVNet(config) {
		Expect(status.Networks.VNet.ResourceGroup).To(PointTo(Equal(*config.Networks.VNet.ResourceGroup)))
		Expect(status.Networks.VNet.Name).To(Equal(*config.Networks.VNet.Name))

		vnetResourceGroupName = *status.Networks.VNet.ResourceGroup
		response, err := az.vnet.Get(ctx, vnetResourceGroupName, status.Networks.VNet.Name, nil)
		Expect(err).ToNot(HaveOccurred())
		vnet = response.VirtualNetwork

		result.vnetResourceGroup = ptr.To(vnetResourceGroupName)
		result.vnet = ptr.To(status.Networks.VNet.Name)
	} else {
		Expect(status.Networks.VNet.ResourceGroup).To(BeNil())

		vnetResourceGroupName = status.ResourceGroup.Name
		response, err := az.vnet.Get(ctx, vnetResourceGroupName, status.Networks.VNet.Name, nil)
		Expect(err).ToNot(HaveOccurred())
		vnet = response.VirtualNetwork

		Expect(vnet.Properties.AddressSpace).ToNot(BeNil())
		Expect(vnet.Properties.AddressSpace.AddressPrefixes).To(Equal([]*string{ptr.To(VNetCIDR)}))
		Expect(vnet.Location).To(PointTo(Equal(*region)))
	}
	Expect(vnet.Tags).To(BeEmpty())

	// security groups
	securityGroupName := infra.Namespace + "-workers"
	secgroup, err := az.securityGroups.Get(ctx, status.ResourceGroup.Name, securityGroupName, nil)
	Expect(err).ToNot(HaveOccurred())
	Expect(secgroup.Properties.SecurityRules).To(BeEmpty())
	Expect(secgroup.Location).To(PointTo(Equal(*region)))

	// route tables
	routeTableName := "worker_route_table"
	rt, err := az.routeTable.Get(ctx, status.ResourceGroup.Name, routeTableName, nil)
	Expect(err).ToNot(HaveOccurred())
	Expect(rt.Location).To(PointTo(Equal(*region)))
	Expect(rt.Properties.Routes).To(BeEmpty())

	ngBaseName := infra.Namespace + "-nat-gateway"
	subnetBaseName := infra.Namespace + "-nodes"
	if !hasDedicatedSubnets(config) {
		nat := config.Networks.NatGateway
		var natID *string
		if nat != nil && nat.Enabled {
			ngName := indexedName(ngBaseName, 0)
			ipNames := []pubIpRef{}

			if len(nat.IPAddresses) > 0 {
				for _, ipRef := range nat.IPAddresses {
					ipNames = append(ipNames, pubIpRef{
						Name:          ipRef.Name,
						ResourceGroup: ipRef.ResourceGroup,
					})
				}
			} else {
				ipNames = []pubIpRef{{
					Name:          fmt.Sprintf("%s-ip", ngName),
					ResourceGroup: status.ResourceGroup.Name,
				}}
			}
			ng = verifyNAT(az, nat.Zone, nat.IdleConnectionTimeoutMinutes, ngName, ipNames, status)
			natID = ng.ID
		}
		verifySubnet(
			az,
			vnet,
			config.Networks.ServiceEndpoints,
			natID,
			*config.Networks.Workers,
			*rt.ID,
			*secgroup.ID,
			subnetBaseName,
			vnetResourceGroupName,
			status.Networks.VNet.Name,
		)
		result.subnets = append(result.subnets, subnetBaseName)

	} else {
		for _, zone := range config.Networks.Zones {
			By(fmt.Sprintf("verifying for %d", zone.Name))
			nat := zone.NatGateway

			var natID *string
			if nat != nil && nat.Enabled {
				ngName := indexedName(ngBaseName, zone.Name)
				ipNames := []pubIpRef{}

				if len(nat.IPAddresses) > 0 {
					for _, ipRef := range nat.IPAddresses {
						ipNames = append(ipNames, pubIpRef{
							Name:          ipRef.Name,
							ResourceGroup: ipRef.ResourceGroup,
						})
					}
				} else {
					ipNames = []pubIpRef{{
						Name:          fmt.Sprintf("%s-ip", ngName),
						ResourceGroup: status.ResourceGroup.Name,
					}}
				}

				ng = verifyNAT(az, &zone.Name, nat.IdleConnectionTimeoutMinutes, ngName, ipNames, status)
				natID = ng.ID
			}
			subnetName := indexedName(subnetBaseName, zone.Name)
			verifySubnet(
				az,
				vnet,
				zone.ServiceEndpoints,
				natID,
				zone.CIDR,
				*rt.ID,
				*secgroup.ID,
				subnetName,
				vnetResourceGroupName,
				status.Networks.VNet.Name,
			)
			result.subnets = append(result.subnets, subnetName)
		}
	}

	// availabilitySets
	if len(status.AvailabilitySets) != 0 {
		for _, avset := range status.AvailabilitySets {
			as, err := az.availabilitySets.Get(ctx, status.ResourceGroup.Name, avset.Name, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(as.Location).To(PointTo(Equal(*region)))
			Expect(as.Properties.PlatformFaultDomainCount).To(PointTo(Equal(int32(CountDomain))))
			Expect(as.Properties.PlatformUpdateDomainCount).To(PointTo(Equal(int32(CountDomain))))
		}
	}

	// identity
	if config.Identity != nil {
		id, err := az.msi.Get(ctx, config.Identity.ResourceGroup, config.Identity.Name, nil)
		Expect(err).ToNot(HaveOccurred())

		Expect(id.Name).To(PointTo(Equal(config.Identity.Name)))
		Expect(status.Identity).ToNot(BeNil())
		Expect(id.Properties.ClientID).ToNot(BeNil())
		Expect(status.Identity.ClientID).To(Equal(*(id.Properties.ClientID)))
		Expect(id.ID).ToNot(BeNil())
		// This is a case-insensitive check to determine if the resouce IDs match. In some cases Azure would respond with
		// different cases in certain parts of the ID string (e.g. resourceGroups vs resourcegroups). IDs in Azure however seem to not take
		// case into account, hence we can safely check with EqualFold.
		Expect(strings.EqualFold(status.Identity.ID, *id.ID)).To(BeTrue())
	}
	Expect(result.resourceGroup).ToNot(BeEmpty())
	return result
}

func verifySubnet(
	az *azureClientSet,
	vnet armnetwork.VirtualNetwork,
	serviceEndpoints []string,
	natGatewayId *string,
	cidr,
	routeTableId,
	securityGroupId,
	subnetName,
	resourceGroupName,
	vnetName string,
) {
	var subnetNames []string
	for _, subnet := range vnet.Properties.Subnets {
		subnetNames = append(subnetNames, *subnet.Name)
	}
	Expect(subnetNames).To(ContainElement(subnetName))

	// subnets
	response, err := az.subnets.Get(ctx, resourceGroupName, vnetName, subnetName, nil)
	Expect(err).ToNot(HaveOccurred())
	subnet := response.Subnet
	Expect(subnet.Properties.AddressPrefix).To(PointTo(Equal(cidr)))
	Expect(subnet.Properties.RouteTable.ID).To(PointTo(Equal(routeTableId)))
	Expect(subnet.Properties.NetworkSecurityGroup.ID).To(PointTo(Equal(securityGroupId)))
	Expect(subnet.Properties.ServiceEndpoints).To(HaveLen(len(serviceEndpoints)))
	for _, se := range subnet.Properties.ServiceEndpoints {
		Expect(serviceEndpoints).To(ContainElement(*se.Service))
	}
	if natGatewayId != nil {
		Expect(subnet.Properties.NatGateway).To(HaveEqualID(*natGatewayId))
	}
}

func verifyNAT(az *azureClientSet, zone, timeout *int32, ngName string, ipNames []pubIpRef, status *azurev1alpha1.InfrastructureStatus) armnetwork.NatGateway {
	response, err := az.nat.Get(ctx, status.ResourceGroup.Name, ngName, nil)
	Expect(err).ToNot(HaveOccurred())

	natGateway := response.NatGateway
	Expect(natGateway.Location).To(PointTo(Equal(*region)))
	Expect(*natGateway.SKU.Name).To(Equal(armnetwork.NatGatewaySKUNameStandard))

	// public IP
	for _, ipName := range ipNames {
		pip, err := az.pubIp.Get(ctx, ipName.ResourceGroup, ipName.Name, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(pip.Location).To(PointTo(Equal(*region)))
		Expect(pip.Properties.PublicIPAllocationMethod).To(PointTo(Equal(armnetwork.IPAllocationMethodStatic)))
		Expect(pip.SKU.Name).To(PointTo(Equal(armnetwork.PublicIPAddressSKUNameStandard)))
		Expect(pip.Properties.PublicIPAddressVersion).To(PointTo(Equal(armnetwork.IPVersionIPv4)))
		Expect(natGateway.Properties.PublicIPAddresses).To(ContainElement(HaveEqualID(*pip.ID)))
	}
	if timeout != nil {
		Expect(natGateway.Properties.IdleTimeoutInMinutes).To(PointTo(Equal(*timeout)))
	}
	if zone != nil {
		var zones []string
		for _, zone := range natGateway.Zones {
			zones = append(zones, *zone)
		}
		Expect(zones).To(ContainElement(Equal(fmt.Sprintf("%d", *zone))))
	}
	return natGateway
}

func verifyDeletion(
	ctx context.Context,
	az *azureClientSet,
	identifier azureIdentifier,
) {
	Expect(identifier.resourceGroup).ToNot(BeEmpty())
	_, err := az.groups.Get(ctx, identifier.resourceGroup, nil)
	Expect(err).To(HaveOccurred())
	Expect(err).To(BeNotFoundError())

	if identifier.vnetResourceGroup != nil && identifier.vnet != nil {
		for _, subnet := range identifier.subnets {
			_, err := az.subnets.Get(ctx, *identifier.vnetResourceGroup, *identifier.vnet, subnet, nil)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeNotFoundError())
		}
	}
}

func indexedName(name string, index int32) string {
	if index == 0 {
		return name
	}
	return fmt.Sprintf("%s-z%d", name, index)
}

func ignoreAzureNotFoundError(err error) error {
	if !IsNotFound(err) {
		return err
	}

	return nil
}
