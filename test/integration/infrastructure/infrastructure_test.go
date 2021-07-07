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
	"encoding/json"
	"flag"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
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
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/test/framework"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
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

var _ = Describe("Infrastructure tests", func() {

	var (
		ctx    = context.Background()
		logger *logrus.Entry

		testEnv   *envtest.Environment
		c         client.Client
		mgrCancel context.CancelFunc
		decoder   runtime.Decoder

		internalChartsPath string
		clientSet          *azureClientSet
	)

	BeforeSuite(func() {
		flag.Parse()
		validateFlags()

		internalChartsPath = azure.InternalChartsPath
		repoRoot := filepath.Join("..", "..", "..")
		azure.InternalChartsPath = filepath.Join(repoRoot, azure.InternalChartsPath)
		logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

		log := logrus.New()
		log.SetOutput(GinkgoWriter)
		logger = logrus.NewEntry(log)

		By("starting test environment")
		testEnv = &envtest.Environment{
			UseExistingCluster: pointer.BoolPtr(true),
			CRDInstallOptions: envtest.CRDInstallOptions{
				Paths: []string{
					filepath.Join(repoRoot, "example", "20-crd-cluster.yaml"),
					filepath.Join(repoRoot, "example", "20-crd-infrastructure.yaml"),
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

		Expect(infrastructure.AddToManager(mgr)).To(Succeed())

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
	})

	AfterSuite(func() {
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

	Context("AvailabilitySet cluster", func() {
		AfterEach(func() {
			framework.RunCleanupActions()
		})

		It("should successfully create and delete AvailabilitySet cluster creating new vNet", func() {
			providerConfig := newInfrastructureConfig(nil, nil, false, false)

			namespace, err := generateName()
			Expect(err).ToNot(HaveOccurred())

			err = runTest(ctx, logger, c, clientSet, namespace, providerConfig, false, decoder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should successfully create and delete AvailabilitySet cluster using existing vNet and existing identity", func() {
			foreignName, err := generateName()
			Expect(err).ToNot(HaveOccurred())

			Expect(prepareNewResourceGroup(ctx, logger, clientSet, foreignName, *region)).To(Succeed())
			Expect(prepareNewVNet(ctx, logger, clientSet, foreignName, foreignName, *region, VNetCIDR)).To(Succeed())
			Expect(prepareNewIdentity(ctx, logger, clientSet, foreignName, foreignName, *region)).To(Succeed())

			var cleanupHandle framework.CleanupActionHandle
			cleanupHandle = framework.AddCleanupAction(func() {
				By("foreign ResourceGroup teardown")
				err := teardownResourceGroup(ctx, clientSet, foreignName)
				Expect(err).ToNot(HaveOccurred())

				framework.RemoveCleanupAction(cleanupHandle)
			})

			vnetConfig := &azurev1alpha1.VNet{
				Name:          pointer.StringPtr(foreignName),
				ResourceGroup: pointer.StringPtr(foreignName),
			}
			identityConfig := &azurev1alpha1.IdentityConfig{
				Name:          foreignName,
				ResourceGroup: foreignName,
			}
			providerConfig := newInfrastructureConfig(vnetConfig, identityConfig, false, false)

			namespace, err := generateName()
			Expect(err).ToNot(HaveOccurred())
			err = runTest(ctx, logger, c, clientSet, namespace, providerConfig, false, decoder)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Zonal cluster", func() {
		AfterEach(func() {
			framework.RunCleanupActions()
		})

		It("should successfully create and delete a zonal cluster without NatGateway creating new vNet", func() {
			providerConfig := newInfrastructureConfig(nil, nil, false, true)

			namespace, err := generateName()
			Expect(err).ToNot(HaveOccurred())

			err = runTest(ctx, logger, c, clientSet, namespace, providerConfig, false, decoder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should successfully create and delete a zonal cluster with NatGateway using an existing vNet and identity", func() {
			foreignName, err := generateName()
			Expect(err).ToNot(HaveOccurred())

			Expect(prepareNewResourceGroup(ctx, logger, clientSet, foreignName, *region)).To(Succeed())
			Expect(prepareNewVNet(ctx, logger, clientSet, foreignName, foreignName, *region, VNetCIDR)).To(Succeed())
			Expect(prepareNewIdentity(ctx, logger, clientSet, foreignName, foreignName, *region)).To(Succeed())

			var cleanupHandle framework.CleanupActionHandle
			cleanupHandle = framework.AddCleanupAction(func() {
				By("foreign ResourceGroup teardown")
				err := teardownResourceGroup(ctx, clientSet, foreignName)
				Expect(err).ToNot(HaveOccurred())

				framework.RemoveCleanupAction(cleanupHandle)
			})

			vnetConfig := &azurev1alpha1.VNet{
				Name:          pointer.StringPtr(foreignName),
				ResourceGroup: pointer.StringPtr(foreignName),
			}
			identityConfig := &azurev1alpha1.IdentityConfig{
				Name:          foreignName,
				ResourceGroup: foreignName,
			}
			providerConfig := newInfrastructureConfig(vnetConfig, identityConfig, true, true)

			namespace, err := generateName()
			Expect(err).ToNot(HaveOccurred())
			err = runTest(ctx, logger, c, clientSet, namespace, providerConfig, false, decoder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should successfully create a multi zonal NAT Gateway cluster", func() {
			var (
				zone1 int32 = 1
				zone2 int32 = 1
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
							NatGateway: &azurev1alpha1.NatGatewayConfig{
								Enabled: true,
								Zone:    &zone1,
							},
						},
						{
							Name:             zone2,
							CIDR:             "10.250.1.0/24",
							ServiceEndpoints: []string{"Microsoft.Storage"},
							NatGateway: &azurev1alpha1.NatGatewayConfig{
								Enabled: true,
								Zone:    &zone2,
							},
						},
					},
				},
				Zoned: true,
			}

			namespace, err := generateName()
			Expect(err).ToNot(HaveOccurred())

			err = runTest(ctx, logger, c, clientSet, namespace, providerConfig, false, decoder)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("VMO cluster", func() {
		AfterEach(func() {
			framework.RunCleanupActions()
		})

		It("should successfully create and delete VMO cluster without NatGateway creating new vNet", func() {
			providerConfig := newInfrastructureConfig(nil, nil, false, false)

			namespace, err := generateName()
			Expect(err).ToNot(HaveOccurred())

			err = runTest(ctx, logger, c, clientSet, namespace, providerConfig, true, decoder)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should successfully create and delete VMO cluster with NatGateway using an existing vNet and identity", func() {
			foreignName, err := generateName()
			Expect(err).ToNot(HaveOccurred())

			Expect(prepareNewResourceGroup(ctx, logger, clientSet, foreignName, *region)).To(Succeed())
			Expect(prepareNewVNet(ctx, logger, clientSet, foreignName, foreignName, *region, VNetCIDR)).To(Succeed())
			Expect(prepareNewIdentity(ctx, logger, clientSet, foreignName, foreignName, *region)).To(Succeed())

			var cleanupHandle framework.CleanupActionHandle
			cleanupHandle = framework.AddCleanupAction(func() {
				By("foreign ResourceGroup teardown")
				err := teardownResourceGroup(ctx, clientSet, foreignName)
				Expect(err).ToNot(HaveOccurred())

				framework.RemoveCleanupAction(cleanupHandle)
			})

			vnetConfig := &azurev1alpha1.VNet{
				Name:          pointer.StringPtr(foreignName),
				ResourceGroup: pointer.StringPtr(foreignName),
			}
			identityConfig := &azurev1alpha1.IdentityConfig{
				Name:          foreignName,
				ResourceGroup: foreignName,
			}
			providerConfig := newInfrastructureConfig(vnetConfig, identityConfig, true, false)

			namespace, err := generateName()
			Expect(err).ToNot(HaveOccurred())
			err = runTest(ctx, logger, c, clientSet, namespace, providerConfig, true, decoder)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

func runTest(
	ctx context.Context,
	logger *logrus.Entry,
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
	logger.Infof("test running in namespace: %s", namespaceName)

	// Cleanup
	var cleanupHandle framework.CleanupActionHandle
	cleanupHandle = framework.AddCleanupAction(func() {

		By("delete infrastructure")
		Expect(client.IgnoreNotFound(c.Delete(ctx, infra))).To(Succeed())

		By("wait until infrastructure is deleted")
		err := extensions.WaitUntilExtensionObjectDeleted(
			ctx,
			c,
			logger,
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

		framework.RemoveCleanupAction(cleanupHandle)
	})

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
		logger,
		infra,
		extensionsv1alpha1.InfrastructureResource,
		10*time.Second,
		30*time.Second,
		16*time.Minute,
		nil,
	); err != nil {
		return err
	}

	By("decode infrastucture status")
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

func newInfrastructureConfig(vnet *azurev1alpha1.VNet, id *azurev1alpha1.IdentityConfig, natGateway, zoned bool) *azurev1alpha1.InfrastructureConfig {
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

	if natGateway {
		nwConfig.NatGateway = &azurev1alpha1.NatGatewayConfig{
			Enabled: true,
		}
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

func prepareNewResourceGroup(ctx context.Context, logger *logrus.Entry, az *azureClientSet, groupName, location string) error {
	logger.Infof("generating new ResourceGroups: %s", groupName)
	_, err := az.groups.CreateOrUpdate(ctx, groupName, resources.Group{
		Location: pointer.StringPtr(location),
	})
	return err
}

func prepareNewVNet(ctx context.Context, logger *logrus.Entry, az *azureClientSet, groupName, vNetName, location, cidr string) error {
	logger.Infof("generating new VNet: %s/%s", groupName, vNetName)
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

func prepareNewIdentity(ctx context.Context, logger *logrus.Entry, az *azureClientSet, groupName, idName, location string) error {
	logger.Infof("generating new Identity %s/%s", groupName, idName)
	_, err := az.msi.CreateOrUpdate(ctx, groupName, idName, msi.Identity{
		Location: pointer.StringPtr(location),
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

func hasNatGateway(config *azurev1alpha1.NatGatewayConfig) bool {
	return config != nil && config.Enabled
}

func usesNatZones(config *azurev1alpha1.InfrastructureConfig) bool {
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

	verifySubnet := func(cidr string, serviceEndpoints []string, nat *azurev1alpha1.NatGatewayConfig, index int) {
		subnetName := indexedName(infra.Namespace+"-nodes", index)
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

		// nat gateway
		if hasNatGateway(nat) {
			ngName := indexedName(infra.Namespace+"-nat-gateway", index)
			pipName := fmt.Sprintf("%s-ip", ngName)

			// public IP
			pip, err := az.pubIp.Get(ctx, status.ResourceGroup.Name, pipName, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(pip.Location).To(PointTo(Equal(*region)))
			Expect(pip.PublicIPAllocationMethod).To(Equal(network.Static))
			Expect(pip.Sku.Name).To(Equal(network.PublicIPAddressSkuNameStandard))
			Expect(pip.PublicIPAddressVersion).To(Equal(network.IPv4))

			// nat gateway
			ng, err = az.nat.Get(ctx, status.ResourceGroup.Name, ngName, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(ng.Location).To(PointTo(Equal(*region)))
			Expect(ng.Sku.Name).To(Equal(network.NatGatewaySkuNameStandard))
			Expect(ng.PublicIPAddresses).To(PointTo(ContainElement(HaveEqualID(*pip.ID))))
			if nat.IdleConnectionTimeoutMinutes != nil {
				Expect(ng.NatGatewayPropertiesFormat.IdleTimeoutInMinutes).To(PointTo(Equal(*nat.IdleConnectionTimeoutMinutes)))
			}
			if nat.Zone != nil {
				Expect(ng.Zones).To(PointTo(ContainElement(Equal(fmt.Sprintf("%d", *nat.Zone)))))
			}

			// subnet
			Expect(subnet.NatGateway).To(HaveEqualID(*ng.ID))
		}
	}

	if !usesNatZones(config) {
		verifySubnet(*config.Networks.Workers, config.Networks.ServiceEndpoints, config.Networks.NatGateway, 0)
	} else {
		for index, zone := range config.Networks.Zones {
			By(fmt.Sprintf("verifying for %d", zone.Name))
			verifySubnet(zone.CIDR, zone.ServiceEndpoints, zone.NatGateway, index)
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
		Expect(status.Identity.ID).To(Equal(*id.ID))
		Expect(status.Identity.ClientID).To(Equal(id.ClientID.String()))
	}

	return result
}

func verifyDeletion(
	ctx context.Context,
	az *azureClientSet,
	identifier azureIdentifier,
) {
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

func indexedName(name string, index int) string {
	if index == 0 {
		return name
	}
	return fmt.Sprintf("%s-z%d", name, index)
}
