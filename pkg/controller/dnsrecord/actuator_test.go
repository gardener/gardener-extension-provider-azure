// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord_test

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/pkg/mock/controller-runtime/manager"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		c                       *mockclient.MockClient
		mgr                     *mockmanager.MockManager
		sw                      *mockclient.MockStatusWriter
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

		c = mockclient.NewMockClient(ctrl)
		sw = mockclient.NewMockStatusWriter(ctrl)
		azureClientFactory = mockazureclient.NewMockFactory(ctrl)
		azureDNSZoneClient = mockazureclient.NewMockDNSZone(ctrl)
		azureDNSRecordSetClient = mockazureclient.NewMockDNSRecordSet(ctrl)

		c.EXPECT().Status().Return(sw).AnyTimes()

		DefaultAzureClientFactoryFunc = func(ctx context.Context, client client.Client, secretRef corev1.SecretReference) (azclient.Factory, error) {
			return azureClientFactory, nil
		}

		ctx = context.TODO()
		logger = log.Log.WithName("test")

		mgr = mockmanager.NewMockManager(ctrl)
		mgr.EXPECT().GetClient().Return(c)

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
			},
		}

		zones = map[string]string{
			shootDomain:   zone,
			"example.com": "zone2",
			"other.com":   "zone3",
		}
	})

	AfterEach(func() {
		DefaultAzureClientFactoryFunc = defaultFactory
		ctrl.Finish()
	})

	Describe("#Reconcile", func() {
		It("should reconcile the DNSRecord", func() {
			azureClientFactory.EXPECT().DNSZone().Return(azureDNSZoneClient, nil)
			azureClientFactory.EXPECT().DNSRecordSet().Return(azureDNSRecordSetClient, nil)
			azureDNSZoneClient.EXPECT().List(ctx).Return(zones, nil)
			azureDNSRecordSetClient.EXPECT().CreateOrUpdate(ctx, zone, domainName, string(extensionsv1alpha1.DNSRecordTypeA), []string{address}, int64(120)).Return(nil)
			sw.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.DNSRecord{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, obj *extensionsv1alpha1.DNSRecord, _ client.Patch, opts ...client.PatchOption) error {
					Expect(obj.Status).To(Equal(extensionsv1alpha1.DNSRecordStatus{
						Zone: pointer.String(zone),
					}))
					return nil
				},
			)

			err := a.Reconcile(ctx, logger, dns, nil)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#Delete", func() {
		It("should delete the DNSRecord", func() {
			dns.Status.Zone = pointer.String(zone)

			azureClientFactory.EXPECT().DNSZone().Return(azureDNSZoneClient, nil)
			azureClientFactory.EXPECT().DNSRecordSet().Return(azureDNSRecordSetClient, nil)
			azureDNSRecordSetClient.EXPECT().Delete(ctx, zone, domainName, string(extensionsv1alpha1.DNSRecordTypeA)).Return(nil)

			err := a.Delete(ctx, logger, dns, nil)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
