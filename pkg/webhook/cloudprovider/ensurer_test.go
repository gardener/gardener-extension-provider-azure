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

package cloudprovider

import (
	"context"
	"testing"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/install"

	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CloudProvider Webhook Suite")
}

var _ = Describe("Ensurer", func() {
	var (
		ctx     = context.TODO()
		ensurer cloudprovider.Ensurer
		scheme  *runtime.Scheme

		ctrl *gomock.Controller
		c    *mockclient.MockClient

		secret                 *corev1.Secret
		servicePrincipalSecret corev1.Secret

		gctx          = gcontext.NewGardenContext(nil, nil)
		labelSelector = client.MatchingLabels{"azure.provider.extensions.gardener.cloud/purpose": "tenant-service-principal-secret"}
	)

	BeforeEach(func() {
		secret = &corev1.Secret{
			Data: map[string][]byte{
				"tenantID": []byte("tenant-id"),
			},
		}
		servicePrincipalSecret = corev1.Secret{
			Data: map[string][]byte{
				"tenantID":     []byte("tenant-id"),
				"clientID":     []byte("client-id"),
				"clientSecret": []byte("client-secret"),
			},
		}

		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		scheme = runtime.NewScheme()
		install.Install(scheme)
		ensurer = NewEnsurer(logger)

		err := ensurer.(inject.Scheme).InjectScheme(scheme)
		Expect(err).NotTo(HaveOccurred())

		err = ensurer.(inject.Client).InjectClient(c)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#EnsureCloudProviderSecret", func() {
		It("should pass as clientID and clientSecret are present", func() {
			secret.Data["clientID"] = []byte("client-id")
			secret.Data["clientSecret"] = []byte("client-secret")

			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail as no tenantID is present", func() {
			delete(secret.Data, "tenantID")
			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should add clientID and clientSecret", func() {
			c.EXPECT().List(gomock.Any(), &corev1.SecretList{}, labelSelector).
				DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = []corev1.Secret{servicePrincipalSecret}
					return nil
				})

			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Data).To(Equal(map[string][]byte{
				"tenantID":     []byte("tenant-id"),
				"clientID":     []byte("client-id"),
				"clientSecret": []byte("client-secret"),
			}))
		})

		It("should fail as service principal secret matching to the tenant id exists", func() {
			c.EXPECT().List(gomock.Any(), &corev1.SecretList{}, labelSelector).
				DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = []corev1.Secret{}
					return nil
				})

			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should fail as multiple service principal secrets matching to the tenant id exists", func() {
			c.EXPECT().List(gomock.Any(), &corev1.SecretList{}, labelSelector).
				DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = []corev1.Secret{servicePrincipalSecret, servicePrincipalSecret}
					return nil
				})

			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should fail as multiple service principal secrets matching to the tenant id exists", func() {
			servicePrincipalSecret.Data["tenantID"] = []byte("some-different-tenant-id")
			c.EXPECT().List(gomock.Any(), &corev1.SecretList{}, labelSelector).
				DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					list.Items = []corev1.Secret{servicePrincipalSecret}
					return nil
				})

			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).To(HaveOccurred())
		})
	})

})
