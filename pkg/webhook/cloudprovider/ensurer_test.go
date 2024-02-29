// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprovider_test

import (
	"context"
	"testing"

	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/pkg/mock/controller-runtime/manager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	. "github.com/gardener/gardener-extension-provider-azure/pkg/webhook/cloudprovider"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CloudProvider Webhook Suite")
}

var _ = Describe("Ensurer", func() {
	var (
		logger  = log.Log.WithName("azure-cloudprovider-webhook-test")
		ctx     = context.TODO()
		ensurer cloudprovider.Ensurer

		ctrl *gomock.Controller
		c    *mockclient.MockClient
		mgr  *mockmanager.MockManager

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

		mgr = mockmanager.NewMockManager(ctrl)

		mgr.EXPECT().GetClient().Return(c)

		ensurer = NewEnsurer(mgr, logger)
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
