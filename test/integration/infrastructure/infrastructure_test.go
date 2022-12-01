// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package infrastructure_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-03-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/msi/mgmt/2018-11-30/msi"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-05-01/network"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	azureRest "github.com/Azure/go-autorest/autorest/azure"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/test/framework"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	azureinstall "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/install"
	azurev1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure"
	. "github.com/gardener/gardener-extension-provider-azure/test/integration/infrastructure"
)

const (
	CountDomain = 1
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
	useFlow        = flag.Bool("use-flow", true, "Set annotation to use flow for reconcilation")
)

func validateFlags() {
	if len(*clientId) == 0 {
		panic("client-id flag is not specified")
	}
	if len(*clientSecret) == 0 {
		panic("client-secret flag is not specified")
	}
	if len(*subscriptionId) == 0 {
		panic("subscription-id flag is not specified")
	}
	if len(*tenantId) == 0 {
		panic("tenant-id flag is not specified")
	}
	if len(*region) == 0 {
		panic("region flag is not specified")
	}
}

// ClientAuth represents a Azure Client Auth credentials.
type ClientAuth struct {
	// SubscriptionID is the Azure subscription ID.
	SubscriptionID string `yaml:"subscriptionID"`
	// TenantID is the Azure tenant ID.
	TenantID string `yaml:"tenantID"`
	// ClientID is the Azure client ID.
	ClientID string `yaml:"clientID"`
	// ClientSecret is the Azure client secret.
	ClientSecret string `yaml:"clientSecret"`
}

func (clientAuth ClientAuth) GetAzClientCredentials() (*azidentity.ClientSecretCredential, error) {
	return azidentity.NewClientSecretCredential(clientAuth.TenantID, clientAuth.ClientID, clientAuth.ClientSecret, nil)
}

type ProviderSecret struct {
	Data ClientAuth `yaml:"data"`
}

func readAuthFromFile(fileName string) ClientAuth {
	secret := ProviderSecret{}
	data, err := os.ReadFile(fileName)
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(data, &secret)
	if err != nil {
		panic(err)
	}
	secret.Data.ClientID = decodeString(secret.Data.ClientID)
	secret.Data.ClientSecret = decodeString(secret.Data.ClientSecret)
	secret.Data.SubscriptionID = decodeString(secret.Data.SubscriptionID)
	secret.Data.TenantID = decodeString(secret.Data.TenantID)
	return secret.Data
}

func decodeString(s string) string {
	res, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return string(res)
}

type azureClientSet struct {
	groups           resources.GroupsClient
	vnet             network.VirtualNetworksClient
	subnets          network.SubnetsClient
	interfaces       network.InterfacesClient
	securityGroups   network.SecurityGroupsClient
	availabilitySets compute.AvailabilitySetsClient
	routeTable       network.RouteTablesClient
	nat              network.NatGatewaysClient
	pubIp            network.PublicIPAddressesClient
	msi              msi.UserAssignedIdentitiesClient
}

func newAzureClientSet(subscriptionId string, authorizer autorest.Authorizer) *azureClientSet {
	groupsClient := resources.NewGroupsClient(subscriptionId)
	groupsClient.Authorizer = authorizer
	vnetClient := network.NewVirtualNetworksClient(subscriptionId)
	vnetClient.Authorizer = authorizer
	subnetClient := network.NewSubnetsClient(subscriptionId)
	subnetClient.Authorizer = authorizer
	interfacesClient := network.NewInterfacesClient(subscriptionId)
	interfacesClient.Authorizer = authorizer
	securityGroupsClient := network.NewSecurityGroupsClient(subscriptionId)
	securityGroupsClient.Authorizer = authorizer
	availabilitySetsClient := compute.NewAvailabilitySetsClient(subscriptionId)
	availabilitySetsClient.Authorizer = authorizer
	tablesClient := network.NewRouteTablesClient(subscriptionId)
	tablesClient.Authorizer = authorizer
	natClient := network.NewNatGatewaysClient(subscriptionId)
	natClient.Authorizer = authorizer
	pubIpClient := network.NewPublicIPAddressesClient(subscriptionId)
	pubIpClient.Authorizer = authorizer
	msiClient := msi.NewUserAssignedIdentitiesClient(subscriptionId)
	msiClient.Authorizer = authorizer

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
	}
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
	// validateFlags() ALTERNATIVE TO SECRET YAML
	//region = to.Ptr("westeurope")
	auth := readAuthFromFile(*secretYamlPath)
	clientId = &auth.ClientID
	clientSecret = &auth.ClientSecret
	subscriptionId = &auth.SubscriptionID
	tenantId = &auth.TenantID

	internalChartsPath := azure.InternalChartsPath
	repoRoot := filepath.Join("..", "..", "..")
	azure.InternalChartsPath = filepath.Join(repoRoot, azure.InternalChartsPath)

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

		azure.InternalChartsPath = internalChartsPath
	})

	By("starting test environment")
	testEnv = &envtest.Environment{
		UseExistingCluster: pointer.BoolPtr(true),
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_clusters.yaml"),
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_infrastructures.yaml"),
			},
		},
	}

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	mgr, err := manager.New(cfg, manager.Options{
		MetricsBindAddress: "0",
	})
	Expect(err).ToNot(HaveOccurred())

	Expect(extensionsv1alpha1.AddToScheme(mgr.GetScheme())).To(Succeed())
	Expect(azureinstall.AddToScheme(mgr.GetScheme())).To(Succeed())

	Expect(infrastructure.AddToManagerWithOptions(mgr, infrastructure.AddOptions{
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

	authorizer, err := getAuthorizer(*tenantId, *clientId, *clientSecret)
	Expect(err).ToNot(HaveOccurred())
	clientSet = newAzureClientSet(*subscriptionId, authorizer)

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

		It("should successfully create and delete AvailabilitySet cluster creating new vNet", Label("passed"), func() {
			providerConfig := newInfrastructureConfig(nil, nil, nil, false)

			namespace, err := generateName()
			Expect(err).ToNot(HaveOccurred())

			err = runTest(ctx, log, c, clientSet, namespace, providerConfig, false, decoder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should successfully create and delete AvailabilitySet cluster using existing vNet and existing identity", Label("passed"), func() {
			foreignName, err := generateName()
			Expect(err).ToNot(HaveOccurred())

			var cleanupHandle framework.CleanupActionHandle
			cleanupHandle = framework.AddCleanupAction(func() {
				Expect(ignoreAzureNotFoundError(teardownResourceGroup(ctx, clientSet, foreignName))).To(Succeed())
				framework.RemoveCleanupAction(cleanupHandle)
			})

			Expect(prepareNewResourceGroup(ctx, log, clientSet, foreignName, *region)).To(Succeed())
			Expect(prepareNewVNet(ctx, log, clientSet, foreignName, foreignName, *region, VNetCIDR)).To(Succeed())
			Expect(prepareNewIdentity(ctx, log, clientSet, foreignName, foreignName, *region)).To(Succeed())

			vnetConfig := &azurev1alpha1.VNet{
				Name:          pointer.StringPtr(foreignName),
				ResourceGroup: pointer.StringPtr(foreignName),
			}
			identityConfig := &azurev1alpha1.IdentityConfig{
				Name:          foreignName,
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

		It("should successfully create and delete a zonal cluster without NatGateway creating new vNet", Label("passed"), func() {
			providerConfig := newInfrastructureConfig(nil, nil, nil, true)

			namespace, err := generateName()
			Expect(err).ToNot(HaveOccurred())

			err = runTest(ctx, log, c, clientSet, namespace, providerConfig, false, decoder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should successfully create and delete a zonal cluster with NatGateway using an existing vNet and identity", Label("single"), func() {
			foreignName, err := generateName()
			Expect(err).ToNot(HaveOccurred())
			var cleanupHandle framework.CleanupActionHandle
			cleanupHandle = framework.AddCleanupAction(func() {
				Expect(ignoreAzureNotFoundError(teardownResourceGroup(ctx, clientSet, foreignName))).To(Succeed())
				framework.RemoveCleanupAction(cleanupHandle)
			})

			Expect(prepareNewResourceGroup(ctx, log, clientSet, foreignName, *region)).To(Succeed())
			Expect(prepareNewVNet(ctx, log, clientSet, foreignName, foreignName, *region, VNetCIDR)).To(Succeed())
			Expect(prepareNewIdentity(ctx, log, clientSet, foreignName, foreignName, *region)).To(Succeed())

			vnetConfig := &azurev1alpha1.VNet{
				Name:          pointer.StringPtr(foreignName),
				ResourceGroup: pointer.StringPtr(foreignName),
			}
			identityConfig := &azurev1alpha1.IdentityConfig{
				Name:          foreignName,
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

		It("should successfully create a multi zonal NAT Gateway cluster", Label("failed"), func() {
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

		It("should successfully create and delete VMO cluster without NatGateway creating new vNet", Label("passed"), func() {
			providerConfig := newInfrastructureConfig(nil, nil, nil, false)

			namespace, err := generateName()
			Expect(err).ToNot(HaveOccurred())

			err = runTest(ctx, log, c, clientSet, namespace, providerConfig, true, decoder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should successfully create and delete VMO cluster with NatGateway using an existing vNet and identity", func() {
			foreignName, err := generateName()
			Expect(err).ToNot(HaveOccurred())

			var cleanupHandle framework.CleanupActionHandle
			cleanupHandle = framework.AddCleanupAction(func() {
				Expect(ignoreAzureNotFoundError(teardownResourceGroup(ctx, clientSet, foreignName))).To(Succeed())
				framework.RemoveCleanupAction(cleanupHandle)
			})

			Expect(prepareNewResourceGroup(ctx, log, clientSet, foreignName, *region)).To(Succeed())
			Expect(prepareNewVNet(ctx, log, clientSet, foreignName, foreignName, *region, VNetCIDR)).To(Succeed())
			Expect(prepareNewIdentity(ctx, log, clientSet, foreignName, foreignName, *region)).To(Succeed())

			vnetConfig := &azurev1alpha1.VNet{
				Name:          pointer.StringPtr(foreignName),
				ResourceGroup: pointer.StringPtr(foreignName),
			}
			identityConfig := &azurev1alpha1.IdentityConfig{
				Name:          foreignName,
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

	By("set flow annotation (based on config)")
	if *useFlow {
		metav1.SetMetaDataAnnotation(&infra.ObjectMeta, infrastructure.AnnotationKeyUseFlow, "true")
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
			CIDR: pointer.StringPtr(VNetCIDR),
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

	return &extensionsv1alpha1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "infrastructure",
			Namespace: namespace,
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
	}, nil
}

func prepareNewResourceGroup(ctx context.Context, log logr.Logger, az *azureClientSet, groupName, location string) error {
	log.Info("generating new ResourceGroups", "groupName", groupName)
	_, err := az.groups.CreateOrUpdate(ctx, groupName, resources.Group{
		Location: pointer.StringPtr(location),
	})
	return err
}

func prepareNewVNet(ctx context.Context, log logr.Logger, az *azureClientSet, groupName, vNetName, location, cidr string) error {
	log.Info("generating new VNet", "groupName", groupName, "vNetName", vNetName)
	vNetFuture, err := az.vnet.CreateOrUpdate(ctx, groupName, vNetName, network.VirtualNetwork{
		VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
			AddressSpace: &network.AddressSpace{
				AddressPrefixes: &[]string{
					cidr,
				},
			},
		},
		Name:     pointer.StringPtr(vNetName),
		Location: pointer.StringPtr(location),
	})
	if err != nil {
		return err
	}
	if err = vNetFuture.WaitForCompletionRef(ctx, az.vnet.Client); err != nil {
		return err
	}

	_, err = vNetFuture.Result(az.vnet)
	return err
}

func prepareNewIdentity(ctx context.Context, log logr.Logger, az *azureClientSet, groupName, idName, location string) error {
	log.Info("generating new Identity", "groupName", groupName, "idName", idName)
	_, err := az.msi.CreateOrUpdate(ctx, groupName, idName, msi.Identity{
		Location: pointer.StringPtr(location),
	})
	return err
}

func prepareNewNatIp(ctx context.Context, log logr.Logger, az *azureClientSet, groupName, pubIpName, location, zone string) error {
	log.Info("generating new nat ip", "groupName", groupName, "pubIpName", pubIpName)
	_, err := az.pubIp.CreateOrUpdate(ctx, groupName, pubIpName, network.PublicIPAddress{
		Name: pointer.String(pubIpName),
		Sku: &network.PublicIPAddressSku{
			Name: network.PublicIPAddressSkuNameStandard,
		},
		Zones:    &[]string{zone},
		Location: &location,
		PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
			PublicIPAllocationMethod: network.Static,
			PublicIPAddressVersion:   network.IPv4,
		},
	})
	return err
}

func teardownResourceGroup(ctx context.Context, az *azureClientSet, groupName string) error {
	future, err := az.groups.Delete(ctx, groupName)
	if err != nil {
		return err
	}

	return future.WaitForCompletionRef(ctx, az.groups.Client)
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
		vnet                  network.VirtualNetwork
		ng                    network.NatGateway
		err                   error
		result                = azureIdentifier{}
	)

	// resource groups
	if config.ResourceGroup != nil {
		Expect(status.ResourceGroup.Name).To(Equal(config.ResourceGroup.Name))
	} else {
		Expect(infra.Namespace).To(Equal(status.ResourceGroup.Name))
	}

	group, err := az.groups.Get(ctx, status.ResourceGroup.Name)
	Expect(err).ToNot(HaveOccurred())
	Expect(group.Location).To(PointTo(Equal(*region)))
	Expect(group.Tags).To(BeEmpty())

	result.resourceGroup = status.ResourceGroup.Name

	// VNet
	if hasForeignVNet(config) {
		Expect(status.Networks.VNet.ResourceGroup).To(PointTo(Equal(*config.Networks.VNet.ResourceGroup)))
		Expect(status.Networks.VNet.Name).To(Equal(*config.Networks.VNet.Name))

		vnetResourceGroupName = *status.Networks.VNet.ResourceGroup
		vnet, err = az.vnet.Get(ctx, vnetResourceGroupName, status.Networks.VNet.Name, "")
		Expect(err).ToNot(HaveOccurred())

		result.vnetResourceGroup = pointer.StringPtr(vnetResourceGroupName)
		result.vnet = pointer.StringPtr(status.Networks.VNet.Name)
	} else {
		Expect(status.Networks.VNet.ResourceGroup).To(BeNil())

		vnetResourceGroupName = status.ResourceGroup.Name
		vnet, err = az.vnet.Get(ctx, vnetResourceGroupName, status.Networks.VNet.Name, "")
		Expect(err).ToNot(HaveOccurred())

		Expect(vnet.VirtualNetworkPropertiesFormat.AddressSpace).ToNot(BeNil())
		Expect(vnet.VirtualNetworkPropertiesFormat.AddressSpace.AddressPrefixes).To(PointTo(ConsistOf([]string{VNetCIDR})))
		Expect(vnet.Location).To(PointTo(Equal(*region)))
	}
	Expect(vnet.Tags).To(BeEmpty())

	// security groups
	securityGroupName := infra.Namespace + "-workers"
	secgroup, err := az.securityGroups.Get(ctx, status.ResourceGroup.Name, securityGroupName, "")
	Expect(err).ToNot(HaveOccurred())
	Expect(secgroup.SecurityRules).To(Or(BeNil(), PointTo(BeEmpty())))
	Expect(secgroup.Location).To(PointTo(Equal(*region)))

	// route tables
	routeTableName := "worker_route_table"
	rt, err := az.routeTable.Get(ctx, status.ResourceGroup.Name, routeTableName, "")
	Expect(err).ToNot(HaveOccurred())
	Expect(rt.Location).To(PointTo(Equal(*region)))
	Expect(rt.Routes).To(Or(BeNil(), PointTo(BeEmpty())))

	verifySubnet := func(cidr string, serviceEndpoints []string, natID *string, subnetName string) {
		// subnetName := indexedName(infra.Namespace+"-nodes", index)
		Expect(vnet.VirtualNetworkPropertiesFormat.Subnets).To(PointTo(ContainElement(MatchFields(IgnoreExtras, Fields{
			"Name": PointTo(Equal(subnetName)),
		}))))
		result.subnets = append(result.subnets, subnetName)

		// subnets
		subnet, err := az.subnets.Get(ctx, vnetResourceGroupName, status.Networks.VNet.Name, subnetName, "")
		Expect(err).ToNot(HaveOccurred())
		Expect(subnet.AddressPrefix).To(PointTo(Equal(cidr)))
		Expect(subnet.RouteTable).To(HaveEqualID(*rt.ID))
		Expect(subnet.NetworkSecurityGroup).To(HaveEqualID(*secgroup.ID))
		Expect(subnet.ServiceEndpoints).To(PointTo(HaveLen(len(serviceEndpoints))))
		for _, se := range serviceEndpoints {
			Expect(subnet.ServiceEndpoints).To(PointTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Service": PointTo(Equal(se)),
			}))))
		}
		if natID != nil {
			Expect(subnet.NatGateway).To(HaveEqualID(*ng.ID))
		}
	}

	type pubIpRef struct {
		Name          string
		ResourceGroup string
	}

	verifyNAT := func(zone, timeout *int32, ngName string, ipNames []pubIpRef) *string {
		ng, err = az.nat.Get(ctx, status.ResourceGroup.Name, ngName, "")
		Expect(err).ToNot(HaveOccurred())
		Expect(ng.Location).To(PointTo(Equal(*region)))
		Expect(ng.Sku.Name).To(Equal(network.NatGatewaySkuNameStandard))

		// public IP
		for _, ipName := range ipNames {
			pip, err := az.pubIp.Get(ctx, ipName.ResourceGroup, ipName.Name, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(pip.Location).To(PointTo(Equal(*region)))
			Expect(pip.PublicIPAllocationMethod).To(Equal(network.Static))
			Expect(pip.Sku.Name).To(Equal(network.PublicIPAddressSkuNameStandard))
			Expect(pip.PublicIPAddressVersion).To(Equal(network.IPv4))
			Expect(ng.PublicIPAddresses).To(PointTo(ContainElement(HaveEqualID(*pip.ID))))
		}
		if timeout != nil {
			Expect(ng.NatGatewayPropertiesFormat.IdleTimeoutInMinutes).To(PointTo(Equal(*timeout)))
		}
		if zone != nil {
			Expect(ng.Zones).To(PointTo(ContainElement(Equal(fmt.Sprintf("%d", *zone)))))
		}

		return ng.ID
	}

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

			natID = verifyNAT(nat.Zone, nat.IdleConnectionTimeoutMinutes, ngName, ipNames)
		}
		verifySubnet(*config.Networks.Workers, config.Networks.ServiceEndpoints, natID, subnetBaseName)
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

				natID = verifyNAT(&zone.Name, nat.IdleConnectionTimeoutMinutes, ngName, ipNames)
			}
			subnetName := indexedName(subnetBaseName, zone.Name)
			verifySubnet(zone.CIDR, zone.ServiceEndpoints, natID, subnetName)
		}
	}

	// availabilitySets
	if len(status.AvailabilitySets) != 0 {
		for _, avset := range status.AvailabilitySets {
			as, err := az.availabilitySets.Get(ctx, status.ResourceGroup.Name, avset.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(as.Location).To(PointTo(Equal(*region)))
			Expect(as.PlatformFaultDomainCount).To(PointTo(Equal(int32(CountDomain))))
			Expect(as.PlatformUpdateDomainCount).To(PointTo(Equal(int32(CountDomain))))
		}
	}

	// identity
	if config.Identity != nil {
		id, err := az.msi.Get(ctx, config.Identity.ResourceGroup, config.Identity.Name)
		Expect(err).ToNot(HaveOccurred())

		Expect(id.Name).To(PointTo(Equal(config.Identity.Name)))
		Expect(status.Identity).ToNot(BeNil())
		Expect(id.ClientID).ToNot(BeNil())
		Expect(status.Identity.ClientID).To(Equal(id.ClientID.String()))
		Expect(id.ID).ToNot(BeNil())
		// This is a case-insensitive check to determine if the resouce IDs match. In some cases Azure would respond with
		// different cases in certain parts of the ID string (e.g. resourceGroups vs resourcegroups). IDs in Azure however seem to not take
		// case into account, hence we can safely check with EqualFold.
		Expect(strings.EqualFold(status.Identity.ID, *id.ID)).To(BeTrue())
	}

	return result
}

func verifyDeletion(
	ctx context.Context,
	az *azureClientSet,
	identifier azureIdentifier,
) {
	Expect(identifier.resourceGroup).To(Not(BeEmpty()))
	_, err := az.groups.Get(ctx, identifier.resourceGroup)
	Expect(err).To(HaveOccurred())
	Expect(err).To(BeNotFoundError())

	if identifier.vnetResourceGroup != nil && identifier.vnet != nil {
		for _, subnet := range identifier.subnets {
			_, err := az.subnets.Get(ctx, *identifier.vnetResourceGroup, *identifier.vnet, subnet, "")
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
