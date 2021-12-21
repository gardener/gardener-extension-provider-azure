// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package dnsrecord_test

import (
	"context"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	mockazureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client/mock"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/controller/dnsrecord"

	"github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
		sw                      *mockclient.MockStatusWriter
		azureClientFactory      *mockazureclient.MockFactory
		azureDNSZoneClient      *mockazureclient.MockDNSZone
		azureDNSRecordSetClient *mockazureclient.MockDNSRecordSet
		ctx                     context.Context
		logger                  logr.Logger
		a                       dnsrecord.Actuator
		dns                     *extensionsv1alpha1.DNSRecord
		zones                   map[string]string
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)
		sw = mockclient.NewMockStatusWriter(ctrl)
		azureClientFactory = mockazureclient.NewMockFactory(ctrl)
		azureDNSZoneClient = mockazureclient.NewMockDNSZone(ctrl)
		azureDNSRecordSetClient = mockazureclient.NewMockDNSRecordSet(ctrl)

		c.EXPECT().Status().Return(sw).AnyTimes()

		ctx = context.TODO()
		logger = log.Log.WithName("test")

		a = NewActuator(c, azureClientFactory, logger)

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
		ctrl.Finish()
	})

	Describe("#Reconcile", func() {
		It("should reconcile the DNSRecord", func() {
			azureClientFactory.EXPECT().DNSZone(ctx, dns.Spec.SecretRef).Return(azureDNSZoneClient, nil)
			azureClientFactory.EXPECT().DNSRecordSet(ctx, dns.Spec.SecretRef).Return(azureDNSRecordSetClient, nil)
			azureDNSZoneClient.EXPECT().GetAll(ctx).Return(zones, nil)
			azureDNSRecordSetClient.EXPECT().CreateOrUpdate(ctx, zone, domainName, string(extensionsv1alpha1.DNSRecordTypeA), []string{address}, int64(120)).Return(nil)
			azureDNSRecordSetClient.EXPECT().Delete(ctx, zone, "comment-"+domainName, "TXT").Return(nil)
			sw.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.DNSRecord{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, obj *extensionsv1alpha1.DNSRecord, _ client.Patch, opts ...client.PatchOption) error {
					Expect(obj.Status).To(Equal(extensionsv1alpha1.DNSRecordStatus{
						Zone: pointer.String(zone),
					}))
					return nil
				},
			)

			err := a.Reconcile(ctx, dns, nil)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#Delete", func() {
		It("should delete the DNSRecord", func() {
			dns.Status.Zone = pointer.String(zone)

			azureClientFactory.EXPECT().DNSZone(ctx, dns.Spec.SecretRef).Return(azureDNSZoneClient, nil)
			azureClientFactory.EXPECT().DNSRecordSet(ctx, dns.Spec.SecretRef).Return(azureDNSRecordSetClient, nil)
			azureDNSRecordSetClient.EXPECT().Delete(ctx, zone, domainName, string(extensionsv1alpha1.DNSRecordTypeA)).Return(nil)

			err := a.Delete(ctx, dns, nil)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
