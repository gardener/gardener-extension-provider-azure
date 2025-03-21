// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/logger"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/test/framework"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	azureinstall "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/install"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	dnsrecordctrl "github.com/gardener/gardener-extension-provider-azure/pkg/controller/dnsrecord"
	. "github.com/gardener/gardener-extension-provider-azure/test/utils"
)

var (
	clientId       = flag.String("client-id", "", "Azure client ID")
	clientSecret   = flag.String("client-secret", "", "Azure client secret")
	subscriptionId = flag.String("subscription-id", "", "Azure subscription ID")
	tenantId       = flag.String("tenant-id", "", "Azure tenant ID")
	region         = flag.String("region", "", "Azure region")
	logLevel       = flag.String("logLevel", "", "Log level (debug, info, error)")
)

type azureClientSet struct {
	groups       *armresources.ResourceGroupsClient
	dnsZone      *armdns.ZonesClient
	dnsRecordSet *armdns.RecordSetsClient
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

	factory, err := armdns.NewClientFactory(subscriptionId, credential, nil)
	if err != nil {
		return nil, err
	}
	dnsZone := factory.NewZonesClient()
	dnsRecordSet := factory.NewRecordSetsClient()

	return &azureClientSet{
		groups:       groupsClient,
		dnsZone:      dnsZone,
		dnsRecordSet: dnsRecordSet,
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
}

func validateFlags() {
	if len(*subscriptionId) == 0 {
		panic("need an Azure subscription ID")
	}
	if len(*tenantId) == 0 {
		panic("need an Azure tenant ID")
	}
	if len(*clientId) == 0 {
		panic("need an Azure client ID")
	}
	if len(*clientSecret) == 0 {
		panic("need an Azure client secret")
	}
	if len(*region) == 0 {
		panic("need an Azure region")
	}
	if len(*logLevel) == 0 {
		logLevel = ptr.To(logger.DebugLevel)
	} else {
		if !slices.Contains(logger.AllLogLevels, *logLevel) {
			panic("invalid log level: " + *logLevel)
		}
	}
}

var (
	ctx = context.Background()

	log       logr.Logger
	clientSet *azureClientSet
	testEnv   *envtest.Environment
	mgrCancel context.CancelFunc
	c         client.Client

	testName string
	zoneName string
	zoneID   string

	namespace *corev1.Namespace
	secret    *corev1.Secret
	cluster   *extensionsv1alpha1.Cluster
)

var _ = BeforeSuite(func() {
	repoRoot := filepath.Join("..", "..", "..")

	// enable manager logs
	logf.SetLogger(logger.MustNewZapLogger(*logLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))

	log = logf.Log.WithName("dnsrecord-test")

	flag.Parse()
	secretsFromEnv()
	validateFlags()

	DeferCleanup(func() {
		defer func() {
			By("stopping manager")
			mgrCancel()
		}()

		By("running cleanup actions")
		framework.RunCleanupActions()

		By("deleting Azure resource group")
		deleteResourceGroup(ctx, clientSet, testName)

		By("tearing down shoot environment")
		teardownShootEnvironment(ctx, c, namespace, secret, cluster)

		By("stopping test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	By("generating randomized test resource identifiers")
	testName = fmt.Sprintf("azure-dnsrecord-it--%s", randomString())
	zoneName = testName + ".gardener.cloud"
	namespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testName,
		},
	}
	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dnsrecord",
			Namespace: testName,
		},
		Data: map[string][]byte{
			azure.SubscriptionIDKey: []byte(*subscriptionId),
			azure.TenantIDKey:       []byte(*tenantId),
			azure.ClientIDKey:       []byte(*clientId),
			azure.ClientSecretKey:   []byte(*clientSecret),
		},
	}
	cluster = &extensionsv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: testName,
		},
		Spec: extensionsv1alpha1.ClusterSpec{
			CloudProfile: runtime.RawExtension{Raw: []byte("{}")},
			Seed:         runtime.RawExtension{Raw: []byte("{}")},
			Shoot:        runtime.RawExtension{Raw: []byte("{}")},
		},
	}

	By("starting test environment")
	testEnv = &envtest.Environment{
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_dnsrecords.yaml"),
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_clusters.yaml"),
			},
		},
		ControlPlaneStopTimeout: 2 * time.Minute,
	}

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	By("setting up manager")
	mgr, err := manager.New(cfg, manager.Options{
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
	})
	Expect(err).ToNot(HaveOccurred())

	Expect(extensionsv1alpha1.AddToScheme(mgr.GetScheme())).To(Succeed())
	Expect(azureinstall.AddToScheme(mgr.GetScheme())).To(Succeed())

	Expect(dnsrecordctrl.AddToManagerWithOptions(ctx, mgr, dnsrecordctrl.AddOptions{})).To(Succeed())

	var mgrContext context.Context
	mgrContext, mgrCancel = context.WithCancel(ctx)

	By("starting manager")
	go func() {
		defer GinkgoRecover()
		err := mgr.Start(mgrContext)
		Expect(err).NotTo(HaveOccurred())
	}()

	// test client should be uncached and independent from the tested manager
	c, err = client.New(cfg, client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(c).NotTo(BeNil())

	clientSet, err = newAzureClientSet(*subscriptionId, *tenantId, *clientId, *clientSecret)
	Expect(err).NotTo(HaveOccurred())

	By("setting up shoot environment")
	setupShootEnvironment(ctx, c, namespace, secret, cluster)

	By("creating Azure resource group")
	createResourceGroup(ctx, clientSet, testName, *region)

	By("creating Azure DNS hosted zone")
	zoneID = createDNSHostedZone(ctx, clientSet, zoneName)
})

var runTest = func(dns *extensionsv1alpha1.DNSRecord, newValues []string, beforeCreate, beforeUpdate, beforeDelete func()) {
	if beforeCreate != nil {
		beforeCreate()
	}

	By("creating dnsrecord")
	createDNSRecord(ctx, c, dns)

	defer func() {
		if beforeDelete != nil {
			beforeDelete()
		}

		By("deleting dnsrecord")
		deleteDNSRecord(ctx, c, dns)

		By("waiting until dnsrecord is deleted")
		waitUntilDNSRecordDeleted(ctx, c, log, dns)

		By("verifying that the Azure DNS recordset does not exist")
		verifyDNSRecordSetDeleted(ctx, clientSet, dns)
	}()

	framework.AddCleanupAction(func() {
		By("deleting the Azure DNS recordset if it still exists")
		deleteDNSRecordSet(ctx, clientSet, dns)
	})

	By("waiting until dnsrecord is ready")
	waitUntilDNSRecordReady(ctx, c, log, dns)

	By("getting dnsrecord and verifying its status")
	getDNSRecordAndVerifyStatus(ctx, c, dns, zoneID)

	By("verifying that the Azure DNS recordset exists and matches dnsrecord")
	verifyDNSRecordSet(ctx, clientSet, dns)

	if len(newValues) > 0 {
		if beforeUpdate != nil {
			beforeUpdate()
		}

		dns.Spec.Values = newValues
		metav1.SetMetaDataAnnotation(&dns.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)

		By("updating dnsrecord")
		updateDNSRecord(ctx, c, dns)

		By("waiting until dnsrecord is ready")
		waitUntilDNSRecordReady(ctx, c, log, dns)

		By("getting dnsrecord and verifying its status")
		getDNSRecordAndVerifyStatus(ctx, c, dns, zoneID)

		By("verifying that the Azure DNS recordset exists and matches dnsrecord")
		verifyDNSRecordSet(ctx, clientSet, dns)
	}
}

var _ = Describe("DNSRecord tests", func() {
	Context("when a DNS recordset doesn't exist and is not changed or deleted before dnsrecord deletion", func() {
		It("should successfully create and delete a dnsrecord of type A", func() {
			dns := newDNSRecord(testName, zoneName, nil, extensionsv1alpha1.DNSRecordTypeA, []string{"1.1.1.1", "2.2.2.2"}, ptr.To[int64](300))
			runTest(dns, nil, nil, nil, nil)
		})

		It("should successfully create and delete a dnsrecord of type CNAME", func() {
			dns := newDNSRecord(testName, zoneName, ptr.To(zoneID), extensionsv1alpha1.DNSRecordTypeCNAME, []string{"foo.example.com"}, ptr.To[int64](600))
			runTest(dns, nil, nil, nil, nil)
		})

		It("should successfully create and delete a dnsrecord of type TXT", func() {
			dns := newDNSRecord(testName, zoneName, ptr.To(zoneID), extensionsv1alpha1.DNSRecordTypeTXT, []string{"foo", "bar"}, nil)
			runTest(dns, nil, nil, nil, nil)
		})
	})

	Context("when a DNS recordset exists and is changed before dnsrecord update and deletion", func() {
		It("should successfully create, update, and delete a dnsrecord", func() {
			dns := newDNSRecord(testName, zoneName, ptr.To(zoneID), extensionsv1alpha1.DNSRecordTypeA, []string{"1.1.1.1", "2.2.2.2"}, ptr.To[int64](300))
			aRecord := armdns.ARecord{IPv4Address: ptr.To("8.8.8.8")}
			recordSet := armdns.RecordSet{Properties: &armdns.RecordSetProperties{TTL: ptr.To(int64(120)), ARecords: []*armdns.ARecord{&aRecord}}}

			runTest(
				dns,
				[]string{"3.3.3.3", "1.1.1.1"},
				func() {
					By("creating Azure DNS recordset")
					_, err := clientSet.dnsRecordSet.CreateOrUpdate(ctx, testName, zoneName, dns.Name, armdns.RecordTypeA, recordSet, nil)
					Expect(err).ToNot(HaveOccurred())
				},
				func() {
					By("updating Azure DNS recordset")
					_, err := clientSet.dnsRecordSet.CreateOrUpdate(ctx, testName, zoneName, dns.Name, armdns.RecordTypeA, recordSet, nil)
					Expect(err).ToNot(HaveOccurred())
				},
				func() {
					By("updating Azure DNS recordset")
					_, err := clientSet.dnsRecordSet.CreateOrUpdate(ctx, testName, zoneName, dns.Name, armdns.RecordTypeA, recordSet, nil)
					Expect(err).ToNot(HaveOccurred())
				},
			)
		})
	})

	Context("when a DNS recordset exists and is deleted before dnsrecord deletion", func() {
		It("should successfully create and delete a dnsrecord", func() {
			dns := newDNSRecord(testName, zoneName, nil, extensionsv1alpha1.DNSRecordTypeA, []string{"1.1.1.1", "2.2.2.2"}, ptr.To[int64](300))
			aRecord := armdns.ARecord{IPv4Address: ptr.To("8.8.8.8")}
			recordSet := armdns.RecordSet{Properties: &armdns.RecordSetProperties{TTL: ptr.To(int64(120)), ARecords: []*armdns.ARecord{&aRecord}}}

			runTest(
				dns,
				nil,
				func() {
					By("creating Azure DNS recordset")
					_, err := clientSet.dnsRecordSet.CreateOrUpdate(ctx, testName, zoneName, dns.Name, armdns.RecordTypeA, recordSet, nil)
					Expect(err).ToNot(HaveOccurred())
				},
				nil,
				func() {
					By("deleting Azure DNS recordset")
					_, err := clientSet.dnsRecordSet.Delete(ctx, testName, zoneName, dns.Name, armdns.RecordTypeA, nil)
					Expect(err).ToNot(HaveOccurred())
				},
			)
		})
	})
})

func setupShootEnvironment(ctx context.Context, c client.Client, namespace *corev1.Namespace, secret *corev1.Secret, cluster *extensionsv1alpha1.Cluster) {
	Expect(c.Create(ctx, namespace)).To(Succeed())
	Expect(c.Create(ctx, secret)).To(Succeed())
	Expect(c.Create(ctx, cluster)).To(Succeed())
}

func teardownShootEnvironment(ctx context.Context, c client.Client, namespace *corev1.Namespace, secret *corev1.Secret, cluster *extensionsv1alpha1.Cluster) {
	Expect(client.IgnoreNotFound(c.Delete(ctx, cluster))).To(Succeed())
	Expect(client.IgnoreNotFound(c.Delete(ctx, secret))).To(Succeed())
	Expect(client.IgnoreNotFound(c.Delete(ctx, namespace))).To(Succeed())
}

func createDNSRecord(ctx context.Context, c client.Client, dns *extensionsv1alpha1.DNSRecord) {
	Expect(c.Create(ctx, dns)).To(Succeed())
}

func updateDNSRecord(ctx context.Context, c client.Client, dns *extensionsv1alpha1.DNSRecord) {
	Expect(c.Update(ctx, dns)).To(Succeed())
}

func deleteDNSRecord(ctx context.Context, c client.Client, dns *extensionsv1alpha1.DNSRecord) {
	Expect(client.IgnoreNotFound(c.Delete(ctx, dns))).To(Succeed())
}

func getDNSRecordAndVerifyStatus(ctx context.Context, c client.Client, dns *extensionsv1alpha1.DNSRecord, zoneID string) {
	Expect(c.Get(ctx, client.ObjectKey{Namespace: dns.Namespace, Name: dns.Name}, dns)).To(Succeed())
	Expect(dns.Status.Zone).To(PointTo(Equal(zoneID)))
}

func waitUntilDNSRecordReady(ctx context.Context, c client.Client, log logr.Logger, dns *extensionsv1alpha1.DNSRecord) {
	Expect(extensions.WaitUntilExtensionObjectReady(
		ctx,
		c,
		log,
		dns,
		extensionsv1alpha1.DNSRecordResource,
		10*time.Second,
		30*time.Second,
		5*time.Minute,
		nil,
	)).To(Succeed())
}

func waitUntilDNSRecordDeleted(ctx context.Context, c client.Client, log logr.Logger, dns *extensionsv1alpha1.DNSRecord) {
	Expect(extensions.WaitUntilExtensionObjectDeleted(
		ctx,
		c,
		log,
		dns.DeepCopy(),
		extensionsv1alpha1.DNSRecordResource,
		10*time.Second,
		5*time.Minute,
	)).To(Succeed())
}

func newDNSRecord(namespace string, zoneName string, zone *string, recordType extensionsv1alpha1.DNSRecordType, values []string, ttl *int64) *extensionsv1alpha1.DNSRecord {
	name := "dnsrecord-" + randomString()
	resourceGroupZone := testName + "/" + zoneName
	return &extensionsv1alpha1.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: extensionsv1alpha1.DNSRecordSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: azure.DNSType,
			},
			SecretRef: corev1.SecretReference{
				Name:      "dnsrecord",
				Namespace: namespace,
			},
			Zone:       &resourceGroupZone,
			Name:       name + "." + zoneName,
			RecordType: recordType,
			Values:     values,
			TTL:        ttl,
		},
	}
}

func createDNSHostedZone(ctx context.Context, clientSet *azureClientSet, zoneName string) string {
	global := "global"
	response, err := clientSet.dnsZone.CreateOrUpdate(ctx, testName, zoneName, armdns.Zone{
		Location: &global,
	}, &armdns.ZonesClientCreateOrUpdateOptions{})
	Expect(err).NotTo(HaveOccurred())
	zoneID := testName + "/" + *response.Name
	return zoneID
}

func createResourceGroup(ctx context.Context, clientSet *azureClientSet, groupName string, region string) {
	_, err := clientSet.groups.CreateOrUpdate(ctx, groupName, armresources.ResourceGroup{
		Location: ptr.To(region),
	}, nil)
	Expect(err).ToNot(HaveOccurred())
}

func deleteResourceGroup(ctx context.Context, clientSet *azureClientSet, groupName string) {
	poller, err := clientSet.groups.BeginDelete(ctx, groupName, nil)
	Expect(err).NotTo(HaveOccurred())
	_, err = poller.PollUntilDone(ctx, nil)
	Expect(err).NotTo(HaveOccurred())
}

func verifyDNSRecordSet(ctx context.Context, clientSet *azureClientSet, dns *extensionsv1alpha1.DNSRecord) {
	recordType := armdns.RecordType(dns.Spec.RecordType)
	response, err := clientSet.dnsRecordSet.Get(ctx, testName, zoneName, dns.Name, recordType, &armdns.RecordSetsClientGetOptions{})
	Expect(err).NotTo(HaveOccurred())

	rrs := response.RecordSet
	Expect(response.RecordSet).NotTo(BeNil())

	expectedType := string("Microsoft.Network/dnszones/" + recordType)
	Expect(rrs.Name).To(PointTo(Equal(dns.Name)))
	Expect(rrs.Type).To(PointTo(Equal(expectedType)))
	Expect(rrs.Properties.TTL).To(PointTo(Equal(ptr.Deref(dns.Spec.TTL, 120))))

	switch recordType {
	case armdns.RecordTypeA:
		expectedRecords := make([]*armdns.ARecord, len(dns.Spec.Values))
		for i, value := range dns.Spec.Values {
			expectedRecords[i] = &armdns.ARecord{
				IPv4Address: &value,
			}
		}
		Expect(rrs.Properties.ARecords).To(ConsistOf(expectedRecords))
	case armdns.RecordTypeCNAME:
		Expect(rrs.Properties.CnameRecord.Cname).To(PointTo(Equal(dns.Spec.Values[0])))
	case armdns.RecordTypeTXT:
		expectedRecords := make([]*armdns.TxtRecord, len(dns.Spec.Values))
		for i, value := range dns.Spec.Values {
			expectedRecords[i] = &armdns.TxtRecord{
				Value: []*string{&value},
			}
		}
		Expect(rrs.Properties.TxtRecords).To(ConsistOf(expectedRecords))
	}
}

func verifyDNSRecordSetDeleted(ctx context.Context, clientSet *azureClientSet, dns *extensionsv1alpha1.DNSRecord) {
	_, err := clientSet.dnsRecordSet.Get(ctx, testName, zoneName, dns.Spec.Name, armdns.RecordType(dns.Spec.RecordType), &armdns.RecordSetsClientGetOptions{})
	Expect(err).To(BeNotFoundError())
}

func deleteDNSRecordSet(ctx context.Context, clientSet *azureClientSet, dns *extensionsv1alpha1.DNSRecord) {
	_, err := clientSet.dnsRecordSet.Delete(ctx, testName, zoneName, dns.Spec.Name, armdns.RecordType(dns.Spec.RecordType), &armdns.RecordSetsClientDeleteOptions{})
	Expect(err).NotTo(HaveOccurred())
}

func randomString() string {
	rs, err := gardenerutils.GenerateRandomStringFromCharset(5, "0123456789abcdefghijklmnopqrstuvwxyz")
	Expect(err).NotTo(HaveOccurred())
	return rs
}
