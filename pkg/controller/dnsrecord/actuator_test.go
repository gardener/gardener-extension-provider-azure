// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord_test

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	testutils "github.com/gardener/gardener/pkg/utils/test"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	azclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	mockazureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client/mock"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/controller/dnsrecord"
)

const (
	name        = "azure-external"
	namespace   = "shoot--foobar--az"
	shootDomain = "shoot.example.com"
	domainName  = "api.azure.foobar." + shootDomain
	zone        = "zone"
	address     = "1.2.3.4"
)

var _ = Describe("Actuator", func() {
	var (
		ctrl                    *gomock.Controller
		c                       client.Client
		mgr                     testutils.FakeManager
		azureClientFactory      *mockazureclient.MockFactory
		azureDNSZoneClient      *mockazureclient.MockDNSZone
		azureDNSRecordSetClient *mockazureclient.MockDNSRecordSet
		ctx                     context.Context
		logger                  logr.Logger
		a                       dnsrecord.Actuator
		dns                     *extensionsv1alpha1.DNSRecord
		zones                   map[string]string
		defaultFactory          = DefaultAzureClientFactoryFunc
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		azureClientFactory = mockazureclient.NewMockFactory(ctrl)
		azureDNSZoneClient = mockazureclient.NewMockDNSZone(ctrl)
		azureDNSRecordSetClient = mockazureclient.NewMockDNSRecordSet(ctrl)

		DefaultAzureClientFactoryFunc = func(_ context.Context, _ client.Client, _ corev1.SecretReference, _ bool, _ ...azclient.AzureFactoryOption) (azclient.Factory, error) {
			return azureClientFactory, nil
		}

		ctx = context.TODO()
		logger = log.Log.WithName("test")

		scheme := runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(scheme)).To(Succeed())

		c = fakeclient.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&extensionsv1alpha1.DNSRecord{}).
			Build()
		mgr = testutils.FakeManager{Client: c}

		a = NewActuator(mgr)

		dns = &extensionsv1alpha1.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: extensionsv1alpha1.DNSRecordSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: azure.DNSType,
				},
				SecretRef: corev1.SecretReference{
					Name:      name,
					Namespace: namespace,
				},
				Name:       domainName,
				RecordType: extensionsv1alpha1.DNSRecordTypeA,
				Values:     []string{address},
				Region:     ptr.To("Foobar"),
			},
		}
		Expect(c.Create(ctx, dns)).To(Succeed())

		zones = map[string]string{
			shootDomain:   zone,
			"example.com": "zone2",
			"other.com":   "zone3",
		}
	})

	AfterEach(func() {
		DefaultAzureClientFactoryFunc = defaultFactory
	})

	Describe("#Reconcile", func() {
		It("should reconcile the DNSRecord", func() {
			azureClientFactory.EXPECT().DNSZone().Return(azureDNSZoneClient, nil)
			azureClientFactory.EXPECT().DNSRecordSet().Return(azureDNSRecordSetClient, nil)
			azureDNSZoneClient.EXPECT().List(ctx).Return(zones, nil)
			azureDNSRecordSetClient.EXPECT().CreateOrUpdate(ctx, zone, domainName, string(extensionsv1alpha1.DNSRecordTypeA), []string{address}, int64(120)).Return(nil)

			err := a.Reconcile(ctx, logger, dns, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(dns.Status.Zone).To(Equal(ptr.To(zone)))
		})
	})

	Describe("#Delete", func() {
		It("should delete the DNSRecord", func() {
			dns.Status.Zone = ptr.To(zone)

			azureClientFactory.EXPECT().DNSZone().Return(azureDNSZoneClient, nil)
			azureClientFactory.EXPECT().DNSRecordSet().Return(azureDNSRecordSetClient, nil)
			azureDNSRecordSetClient.EXPECT().Delete(ctx, zone, domainName, string(extensionsv1alpha1.DNSRecordTypeA)).Return(nil)

			err := a.Delete(ctx, logger, dns, nil)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
