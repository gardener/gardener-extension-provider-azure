// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/logger"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/test/framework"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	azureinstall "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/install"
	azurev1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	bastionctrl "github.com/gardener/gardener-extension-provider-azure/pkg/controller/bastion"
	. "github.com/gardener/gardener-extension-provider-azure/test/utils"
)

const (
	CommunityGalleryImageID = "/CommunityGalleries/gardenlinux-13e998fe-534d-4b0a-8a27-f16a73aef620/Images/gardenlinux/Versions/1443.10.0"
	machineType             = "Standard_B1s"
	VNetCIDR                = "10.250.0.0/16"
	workersSubnetCIDR       = "10.250.0.0/16"
	userData                = "#!/bin/bash\n\nsystemctl start ssh"
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
	groups         *armresources.ResourceGroupsClient
	vm             *armcompute.VirtualMachinesClient
	vnet           *armnetwork.VirtualNetworksClient
	disk           *armcompute.DisksClient
	interfaces     *armnetwork.InterfacesClient
	securityGroups *armnetwork.SecurityGroupsClient
	pubIp          *armnetwork.PublicIPAddressesClient
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

	vmClient, err := armcompute.NewVirtualMachinesClient(subscriptionId, credential, nil)
	if err != nil {
		return nil, err
	}

	vnetClient, err := armnetwork.NewVirtualNetworksClient(subscriptionId, credential, nil)
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

	pubIpClient, err := armnetwork.NewPublicIPAddressesClient(subscriptionId, credential, nil)
	if err != nil {
		return nil, err
	}

	diskClient, err := armcompute.NewDisksClient(subscriptionId, credential, nil)
	if err != nil {
		return nil, err
	}

	return &azureClientSet{
		groups:         groupsClient,
		vm:             vmClient,
		vnet:           vnetClient,
		disk:           diskClient,
		interfaces:     interfacesClient,
		securityGroups: securityGroupsClient,
		pubIp:          pubIpClient,
	}, nil
}

var (
	ctx = context.Background()
	log logr.Logger

	extensionscluster *extensionsv1alpha1.Cluster
	worker            *extensionsv1alpha1.Worker
	bastion           *extensionsv1alpha1.Bastion
	controllercluster *controller.Cluster
	options           *bastionctrl.Options
	secret            *corev1.Secret

	testEnv    *envtest.Environment
	c          client.Client
	mgrCancel  context.CancelFunc
	clientSet  *azureClientSet
	name       string
	vNetName   string
	subnetName string
	myPublicIP = ""
)

var _ = BeforeSuite(func() {
	randString, err := randomString()
	Expect(err).NotTo(HaveOccurred())

	name = fmt.Sprintf("azure-bastion-it--%s", randString)
	vNetName = name
	subnetName = vNetName + "-nodes"

	myPublicIP, err = getMyPublicIPWithMask()
	Expect(err).NotTo(HaveOccurred())

	flag.Parse()
	validateFlags()

	repoRoot := filepath.Join("..", "..", "..")

	// enable manager logs
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))

	log = logf.Log.WithName("bastion-test")

	log.Info("test environment client publicIP", "publicIP", myPublicIP)

	By("starting test environment")
	testEnv = &envtest.Environment{
		UseExistingCluster: ptr.To(true),
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_clusters.yaml"),
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_bastions.yaml"),
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_workers.yaml"),
			},
		},
	}

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	mgr, err := manager.New(cfg, manager.Options{
		Metrics: server.Options{
			BindAddress: "0",
		},
	})
	Expect(err).ToNot(HaveOccurred())

	Expect(extensionsv1alpha1.AddToScheme(mgr.GetScheme())).To(Succeed())
	Expect(azureinstall.AddToScheme(mgr.GetScheme())).To(Succeed())

	Expect(bastionctrl.AddToManager(ctx, mgr)).To(Succeed())

	var mgrContext context.Context
	mgrContext, mgrCancel = context.WithCancel(ctx)

	By("start manager")
	go func() {
		err := mgr.Start(mgrContext)
		Expect(err).ToNot(HaveOccurred())
	}()

	c = mgr.GetClient()
	Expect(c).ToNot(BeNil())

	clientSet, err = newAzureClientSet(*subscriptionId, *tenantId, *clientId, *clientSecret)
	Expect(err).ToNot(HaveOccurred())

	extensionscluster, controllercluster = createClusters(name)
	bastion, options = createBastion(controllercluster, name)
	worker = createWorker(name)

	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.SecretNameCloudProvider,
			Namespace: name,
		},
		Data: map[string][]byte{
			azure.SubscriptionIDKey: []byte(*subscriptionId),
			azure.TenantIDKey:       []byte(*tenantId),
			azure.ClientIDKey:       []byte(*clientId),
			azure.ClientSecretKey:   []byte(*clientSecret),
		},
	}
})

var _ = AfterSuite(func() {
	defer func() {
		By("stopping manager")
		mgrCancel()
	}()

	By("running cleanup actions")
	framework.RunCleanupActions()

	By("stopping test environment")
	Expect(testEnv.Stop()).To(Succeed())
})

var _ = Describe("Bastion tests", func() {

	It("should successfully create and delete", func() {
		resourceGroupName := name

		securityGroupName := name + "-workers"

		By("setup Infrastructure")
		err := prepareNewResourceGroup(ctx, log, clientSet, resourceGroupName, *region)
		Expect(err).NotTo(HaveOccurred())

		By("setup Network Security Group")
		sg := prepareSecurityGroup(ctx, log, resourceGroupName, securityGroupName, clientSet, *region)

		By("setup Virtual Network")
		prepareNewVNet(ctx, log, clientSet, resourceGroupName, vNetName, subnetName, *region, VNetCIDR, sg)

		framework.AddCleanupAction(func() {
			err = ignoreAzureNotFoundError(teardownResourceGroup(ctx, clientSet, resourceGroupName))
			Expect(err).NotTo(HaveOccurred())
		})

		By("create namespace for test execution")
		setupEnvironmentObjects(ctx, c, namespace(name), secret, extensionscluster, worker)
		framework.AddCleanupAction(func() {
			teardownShootEnvironment(ctx, c, namespace(name), secret, extensionscluster, worker)
		})

		By("setup bastion")
		err = c.Create(ctx, bastion)
		Expect(err).NotTo(HaveOccurred())

		framework.AddCleanupAction(func() {
			teardownBastion(ctx, log, c, bastion)

			By("verify bastion deletion")
			verifyDeletion(ctx, clientSet, options)
		})

		By("wait until bastion is reconciled")
		Expect(extensions.WaitUntilExtensionObjectReady(
			ctx,
			c,
			log,
			bastion,
			extensionsv1alpha1.BastionResource,
			30*time.Second,
			30*time.Second,
			5*time.Minute,
			nil,
		)).To(Succeed())

		time.Sleep(5 * time.Second)
		verifyPort22IsOpen(ctx, c, bastion)
		verifyPort42IsClosed(ctx, c, bastion)

		By("verify cloud resources")
		verifyCreation(ctx, clientSet, options)
	})
})

func getMyPublicIPWithMask() (string, error) {
	resp, err := http.Get("https://api.ipify.org")

	if err != nil {
		return "", err
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			Expect(err).NotTo(HaveOccurred())
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	ip := net.ParseIP(string(body))
	var mask net.IPMask
	if ip.To4() == nil {
		return "", fmt.Errorf("not valid IPv4 address")
	}
	mask = net.CIDRMask(24, 32) // use a /24 net for IPv4

	cidr := net.IPNet{
		IP:   ip,
		Mask: mask,
	}

	full := cidr.String()

	_, ipnet, _ := net.ParseCIDR(full)

	return ipnet.String(), nil
}

func randomString() (string, error) {
	suffix, err := gardenerutils.GenerateRandomStringFromCharset(5, "0123456789abcdefghijklmnopqrstuvwxyz")
	if err != nil {
		return "", err
	}

	return suffix, nil
}

func verifyPort22IsOpen(ctx context.Context, c client.Client, bastion *extensionsv1alpha1.Bastion) {
	By("check connection to port 22 open should not error")
	time.Sleep(1 * time.Minute)
	bastionUpdated := &extensionsv1alpha1.Bastion{}
	Expect(c.Get(ctx, client.ObjectKey{Namespace: bastion.Namespace, Name: bastion.Name}, bastionUpdated)).To(Succeed())

	ipAddress := bastionUpdated.Status.Ingress.IP
	address := net.JoinHostPort(ipAddress, "22")
	conn, err := net.DialTimeout("tcp4", address, 60*time.Second)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(conn).NotTo(BeNil())
}

func verifyPort42IsClosed(ctx context.Context, c client.Client, bastion *extensionsv1alpha1.Bastion) {
	By("check connection to port 42 which should fail")

	bastionUpdated := &extensionsv1alpha1.Bastion{}
	Expect(c.Get(ctx, client.ObjectKey{Namespace: bastion.Namespace, Name: bastion.Name}, bastionUpdated)).To(Succeed())

	ipAddress := bastionUpdated.Status.Ingress.IP
	address := net.JoinHostPort(ipAddress, "42")
	conn, err := net.DialTimeout("tcp4", address, 3*time.Second)
	Expect(err).Should(HaveOccurred())
	Expect(conn).To(BeNil())
}

func prepareNewResourceGroup(ctx context.Context, log logr.Logger, az *azureClientSet, groupName, location string) error {
	log.Info("generating new ResourceGroups", "groupName", groupName)
	_, err := az.groups.CreateOrUpdate(ctx, groupName, armresources.ResourceGroup{
		Location: ptr.To(location),
	}, nil)
	return err
}

func prepareSecurityGroup(ctx context.Context, log logr.Logger, resourceGroupName string, securityGroupName string, az *azureClientSet, location string) armnetwork.SecurityGroup {
	log.Info("generating new SecurityGroups", "securityGroupName", securityGroupName)
	poller, err := az.securityGroups.BeginCreateOrUpdate(ctx, resourceGroupName, securityGroupName, armnetwork.SecurityGroup{
		Location: ptr.To(location),
	}, nil)
	Expect(err).ShouldNot(HaveOccurred())

	result, err := poller.PollUntilDone(ctx, nil)
	Expect(err).ShouldNot(HaveOccurred())

	return result.SecurityGroup
}

func prepareNewVNet(ctx context.Context, log logr.Logger, az *azureClientSet, resourceGroupName, vNetName, subnetName, location, cidr string, nsg armnetwork.SecurityGroup) {
	log.Info("generating new resource Group/VNet/subnetName", "resourceGroupName", resourceGroupName, " vNetName", vNetName, "subnetName", subnetName)
	poller, err := az.vnet.BeginCreateOrUpdate(ctx, resourceGroupName, vNetName, armnetwork.VirtualNetwork{
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{
				AddressPrefixes: []*string{
					ptr.To(cidr),
				},
			},
			Subnets: []*armnetwork.Subnet{
				{
					Name: ptr.To(subnetName),
					Properties: &armnetwork.SubnetPropertiesFormat{
						AddressPrefix:        ptr.To(cidr),
						NetworkSecurityGroup: &nsg,
					},
				},
			},
		},
		Name:     ptr.To(vNetName),
		Location: ptr.To(location),
	}, nil)

	Expect(err).ShouldNot(HaveOccurred())

	_, err = poller.PollUntilDone(ctx, nil)
	Expect(err).ShouldNot(HaveOccurred())
}

func teardownResourceGroup(ctx context.Context, az *azureClientSet, groupName string) error {
	poller, err := az.groups.BeginDelete(ctx, groupName, nil)
	if err != nil {
		return err
	}

	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

func ignoreAzureNotFoundError(err error) error {
	if !IsNotFound(err) {
		return err
	}

	return nil
}

func namespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func setupEnvironmentObjects(ctx context.Context, c client.Client, namespace *corev1.Namespace, secret *corev1.Secret, cluster *extensionsv1alpha1.Cluster, worker *extensionsv1alpha1.Worker) {
	Expect(c.Create(ctx, namespace)).To(Succeed())
	Expect(c.Create(ctx, cluster)).To(Succeed())
	Expect(c.Create(ctx, secret)).To(Succeed())
	Expect(c.Create(ctx, worker)).To(Succeed())
}

func teardownShootEnvironment(ctx context.Context, c client.Client, namespace *corev1.Namespace, secret *corev1.Secret, cluster *extensionsv1alpha1.Cluster, worker *extensionsv1alpha1.Worker) {
	workerCopy := worker.DeepCopy()
	metav1.SetMetaDataAnnotation(&worker.ObjectMeta, "confirmation.gardener.cloud/deletion", "true")
	Expect(c.Patch(ctx, worker, client.MergeFrom(workerCopy))).To(Succeed())

	Expect(client.IgnoreNotFound(c.Delete(ctx, worker))).To(Succeed())
	Expect(client.IgnoreNotFound(c.Delete(ctx, secret))).To(Succeed())
	Expect(client.IgnoreNotFound(c.Delete(ctx, cluster))).To(Succeed())
	Expect(client.IgnoreNotFound(c.Delete(ctx, namespace))).To(Succeed())
}

func createBastion(cluster *controller.Cluster, name string) (*extensionsv1alpha1.Bastion, *bastionctrl.Options) {
	bastion := &extensionsv1alpha1.Bastion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-bastion",
			Namespace: name,
		},
		Spec: extensionsv1alpha1.BastionSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: azure.Type,
			},
			UserData: []byte(userData),
			Ingress: []extensionsv1alpha1.BastionIngressPolicy{
				{IPBlock: networkingv1.IPBlock{
					CIDR: myPublicIP,
				}},
			},
		},
	}

	options, err := bastionctrl.DetermineOptions(bastion, cluster, name)
	Expect(err).NotTo(HaveOccurred())

	return bastion, options
}

func createWorker(name string) *extensionsv1alpha1.Worker {
	return &extensionsv1alpha1.Worker{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: name,
		},
		Spec: extensionsv1alpha1.WorkerSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: azure.Type,
			},
			InfrastructureProviderStatus: &runtime.RawExtension{
				Object: &apisazure.InfrastructureStatus{
					ResourceGroup: apisazure.ResourceGroup{
						Name: name,
					},
					Networks: apisazure.NetworkStatus{
						Layout: apisazure.NetworkLayout("SingleSubnet"),
						VNet:   apisazure.VNetStatus{Name: vNetName},
						Subnets: []apisazure.Subnet{
							{
								Purpose: apisazure.PurposeNodes,
								Name:    subnetName,
							},
						},
					},
				},
			},
			Pools:  []extensionsv1alpha1.WorkerPool{},
			Region: *region,
			SecretRef: corev1.SecretReference{
				Name:      name,
				Namespace: name,
			},
		},
	}
}

func createInfrastructureConfig() *azurev1alpha1.InfrastructureConfig {
	return &azurev1alpha1.InfrastructureConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: azurev1alpha1.SchemeGroupVersion.String(),
			Kind:       "InfrastructureConfig",
		},
		Networks: azurev1alpha1.NetworkConfig{
			Workers: ptr.To(workersSubnetCIDR),
		},
	}
}

func createShoot(infrastructureConfig []byte) *gardencorev1beta1.Shoot {
	return &gardencorev1beta1.Shoot{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "core.gardener.cloud/v1beta1",
			Kind:       "Shoot",
		},

		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: gardencorev1beta1.ShootSpec{
			Region:            *region,
			SecretBindingName: ptr.To(v1beta1constants.SecretNameCloudProvider),
			Provider: gardencorev1beta1.Provider{
				InfrastructureConfig: &runtime.RawExtension{
					Raw: infrastructureConfig,
				}},
		},
	}
}

func createCloudProfile() *gardencorev1beta1.CloudProfile {
	profileConfig := &apisazure.CloudProfileConfig{MachineImages: []apisazure.MachineImages{{
		Name: "gardenlinux",
		Versions: []apisazure.MachineImageVersion{{
			Version:                 "1443.10.0",
			CommunityGalleryImageID: ptr.To(CommunityGalleryImageID),
			Architecture:            ptr.To("amd64"),
		}},
	}}}

	profileConfigData, err := json.Marshal(profileConfig)
	Expect(err).NotTo(HaveOccurred())

	cloudProfile := &gardencorev1beta1.CloudProfile{
		Spec: gardencorev1beta1.CloudProfileSpec{
			Regions: []gardencorev1beta1.Region{
				{Name: *region},
			},
			MachineTypes: []gardencorev1beta1.MachineType{{
				CPU:          resource.MustParse("1"),
				Name:         machineType,
				Architecture: ptr.To("amd64"),
			}},
			MachineImages: []gardencorev1beta1.MachineImage{{
				Name: "gardenlinux",
				Versions: []gardencorev1beta1.MachineImageVersion{
					{
						ExpirableVersion: gardencorev1beta1.ExpirableVersion{
							Version:        "1443.10.0",
							Classification: ptr.To(gardencorev1beta1.ClassificationSupported),
						},
						Architectures: []string{"amd64", "arm64"},
					}},
			}},
			ProviderConfig: &runtime.RawExtension{
				Raw: profileConfigData,
			},
		},
	}
	return cloudProfile
}

func createClusters(name string) (*extensionsv1alpha1.Cluster, *controller.Cluster) {
	infrastructureConfig := createInfrastructureConfig()
	infrastructureConfigJSON, _ := json.Marshal(&infrastructureConfig)

	shoot := createShoot(infrastructureConfigJSON)
	shootJSON, _ := json.Marshal(shoot)

	cloudProfile := createCloudProfile()
	cloudProfileJSON, _ := json.Marshal(cloudProfile)

	extensionscluster := &extensionsv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: extensionsv1alpha1.ClusterSpec{
			CloudProfile: runtime.RawExtension{
				Object: cloudProfile,
				Raw:    cloudProfileJSON,
			},
			Seed: runtime.RawExtension{
				Raw: []byte("{}"),
			},
			Shoot: runtime.RawExtension{
				Object: shoot,
				Raw:    shootJSON,
			},
		},
	}

	cluster := &controller.Cluster{
		ObjectMeta:   metav1.ObjectMeta{Name: name},
		Shoot:        shoot,
		CloudProfile: cloudProfile,
	}
	return extensionscluster, cluster
}

func teardownBastion(ctx context.Context, log logr.Logger, c client.Client, bastion *extensionsv1alpha1.Bastion) {
	By("delete bastion")
	Expect(client.IgnoreNotFound(c.Delete(ctx, bastion))).To(Succeed())

	By("wait until bastion is deleted")
	err := extensions.WaitUntilExtensionObjectDeleted(ctx, c, log, bastion, extensionsv1alpha1.BastionResource, 10*time.Second, 16*time.Minute)
	Expect(err).NotTo(HaveOccurred())
}

func verifyDeletion(ctx context.Context, az *azureClientSet, options *bastionctrl.Options) {
	// bastion public ip should be gone
	_, err := az.pubIp.Get(ctx, options.ResourceGroupName, options.BastionPublicIPName, nil)
	Expect(ignoreAzureNotFoundError(err)).To(Succeed())

	// bastion network interface should be gone
	_, err = az.interfaces.Get(ctx, options.ResourceGroupName, options.NicName, nil)
	Expect(ignoreAzureNotFoundError(err)).To(Succeed())

	// bastion network security group rules should be gone
	// Check network security group rules for Ingress / Egress
	checkSecurityRuleDoesNotExist(ctx, az, options, bastionctrl.NSGIngressAllowSSHResourceNameIPv4(options.BastionInstanceName))
	checkSecurityRuleDoesNotExist(ctx, az, options, bastionctrl.NSGEgressDenyAllResourceName(options.BastionInstanceName))
	checkSecurityRuleDoesNotExist(ctx, az, options, bastionctrl.NSGEgressAllowOnlyResourceName(options.BastionInstanceName))

	// bastion instance should be terminated and not found
	_, err = az.vm.Get(ctx, options.ResourceGroupName, options.BastionInstanceName, nil)
	Expect(ignoreAzureNotFoundError(err)).To(Succeed())

	// bastion instance disk should be terminated and not found
	_, err = az.disk.Get(ctx, options.ResourceGroupName, options.DiskName, nil)
	Expect(ignoreAzureNotFoundError(err)).To(Succeed())
}

func checkSecurityRuleDoesNotExist(ctx context.Context, az *azureClientSet, options *bastionctrl.Options, securityRuleName string) {
	// does not have authorization to performsecurityRules get due to global rule. use security group to check it.
	sg, err := az.securityGroups.Get(ctx, options.ResourceGroupName, securityRuleName, nil)
	if IsNotFound(err) {
		return
	}

	Expect(ignoreAzureNotFoundError(err)).To(Succeed())
	if sg.Properties != nil && sg.Properties.SecurityRules != nil {
		Expect(sg.Properties.SecurityRules).To(BeEmpty())
	}
}

// ruleExist is similar to bastionctrl.RuleExist but for the old network client
func ruleExist(ruleName *string, rules []*armnetwork.SecurityRule) bool {
	if ruleName == nil {
		return false
	}

	for _, rule := range rules {
		if rule.Name != nil && *rule.Name == *ruleName {
			return true
		}
	}
	return false
}

func verifyCreation(ctx context.Context, az *azureClientSet, options *bastionctrl.Options) {
	By("RuleExist")
	// does not have authorization to performsecurityRules get due to global rule. use security group to check it.
	sg, err := az.securityGroups.Get(ctx, options.ResourceGroupName, options.SecurityGroupName, nil)
	Expect(err).NotTo(HaveOccurred())

	// bastion NSG - Check Ingress / Egress firewalls created
	Expect(ruleExist(ptr.To(bastionctrl.NSGIngressAllowSSHResourceNameIPv4(options.BastionInstanceName)), sg.Properties.SecurityRules)).To(BeTrue())
	Expect(ruleExist(ptr.To(bastionctrl.NSGEgressDenyAllResourceName(options.BastionInstanceName)), sg.Properties.SecurityRules)).To(BeTrue())
	Expect(ruleExist(ptr.To(bastionctrl.NSGEgressAllowOnlyResourceName(options.BastionInstanceName)), sg.Properties.SecurityRules)).To(BeTrue())

	By("checking bastion instance")
	// bastion instance
	// compute.InstanceViewTypesUserData
	vm, err := az.vm.Get(ctx, options.ResourceGroupName, options.BastionInstanceName, &armcompute.VirtualMachinesClientGetOptions{
		Expand: ptr.To(armcompute.InstanceViewTypesUserData),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(*vm.Name).To(Equal(options.BastionInstanceName))

	By("checking bastion ingress IpConfigs exist")
	// bastion ingress IpConfigs exist
	nic, err := az.interfaces.Get(ctx, options.ResourceGroupName, options.NicName, nil)
	Expect(err).NotTo(HaveOccurred())
	internalIP := *(nic.Properties.IPConfigurations[0]).Properties.PrivateIPAddress

	publicIp, err := az.pubIp.Get(ctx, options.ResourceGroupName, options.BastionPublicIPName, nil)
	Expect(err).NotTo(HaveOccurred())
	externalIP := publicIp.PublicIPAddress

	Expect(internalIP).NotTo(BeNil())
	Expect(externalIP).NotTo(BeNil())

	By("checking bastion disks exists")
	// bastion Disk exists
	disk, err := az.disk.Get(ctx, options.ResourceGroupName, options.DiskName, nil)
	Expect(err).NotTo(HaveOccurred())
	Expect(*disk.Name).To(Equal(bastionctrl.DiskResourceName(options.BastionInstanceName)))

	By("checking userData matches the constant")
	// userdata ssh-public-key validation
	Expect(*vm.Properties.UserData).To(Equal(base64.StdEncoding.EncodeToString([]byte(userData))))
}
