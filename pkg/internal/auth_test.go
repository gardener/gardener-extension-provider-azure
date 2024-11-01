// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package internal_test

import (
	"context"

	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

var _ = Describe("Azure Auth", func() {
	var (
		ctrl *gomock.Controller

		ctx context.Context

		clientAuth *ClientAuth
		secret     *corev1.Secret
		dnsSecret  *corev1.Secret
		secretRef  corev1.SecretReference

		name      string
		namespace string
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clientSecret, clientID, tenantID, subscriptionID := "secret", "client_id", "tenant_id", "subscription_id"
		clientAuth = &ClientAuth{
			ClientSecret:   clientSecret,
			ClientID:       clientID,
			TenantID:       tenantID,
			SubscriptionID: subscriptionID,
		}
		secret = &corev1.Secret{
			Data: map[string][]byte{
				azure.ClientSecretKey:   []byte(clientSecret),
				azure.ClientIDKey:       []byte(clientID),
				azure.TenantIDKey:       []byte(tenantID),
				azure.SubscriptionIDKey: []byte(subscriptionID),
			},
		}
		dnsSecret = &corev1.Secret{
			Data: map[string][]byte{
				azure.DNSClientSecretKey:   []byte(clientSecret),
				azure.DNSClientIDKey:       []byte(clientID),
				azure.DNSTenantIDKey:       []byte(tenantID),
				azure.DNSSubscriptionIDKey: []byte(subscriptionID),
			},
		}

		ctx = context.TODO()
		namespace = "foo"
		name = "bar"
		secretRef = corev1.SecretReference{
			Namespace: namespace,
			Name:      name,
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#NewClientAuthDataFromSecret", func() {
		It("should read the client auth data from the secret", func() {
			actual, err := NewClientAuthDataFromSecret(secret, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(clientAuth))
		})

		It("should read the client auth data from the secret when workload identity is enabled", func() {
			secret.Labels = map[string]string{
				"security.gardener.cloud/purpose": "workload-identity-token-requestor",
			}
			secret.Data["token"] = []byte("foo")
			actual, err := NewClientAuthDataFromSecret(secret, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(actual.SubscriptionID).To(Equal(clientAuth.SubscriptionID))
			Expect(actual.TenantID).To(Equal(clientAuth.TenantID))
			Expect(actual.ClientID).To(Equal(clientAuth.ClientID))
			Expect(actual.ClientSecret).To(Equal(""))

			token, err := actual.TokenRetriever(context.TODO())
			Expect(err).NotTo(HaveOccurred())
			Expect(token).To(Equal("foo"))
		})
	})

	Describe("#GetClientAuthData", func() {
		Context("DNS keys are not allowed", func() {
			It("should retrieve the client auth data if non-DNS keys ar used", func() {
				var c = mockclient.NewMockClient(ctrl)
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *corev1.Secret, _ ...client.GetOption) error {
						*actual = *secret
						return nil
					})

				actual, _, err := GetClientAuthData(ctx, c, secretRef, false)

				Expect(err).NotTo(HaveOccurred())
				Expect(actual).To(Equal(clientAuth))
			})

			It("should fail if DNS keys ar used", func() {
				var c = mockclient.NewMockClient(ctrl)
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *corev1.Secret, _ ...client.GetOption) error {
						*actual = *dnsSecret
						return nil
					})

				actual, _, err := GetClientAuthData(ctx, c, secretRef, false)

				Expect(err).To(HaveOccurred())
				Expect(actual).To(BeNil())
			})
		})

		Context("DNS keys are allowed", func() {
			It("should retrieve the client auth data if non-DNS keys ar used", func() {
				var c = mockclient.NewMockClient(ctrl)
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *corev1.Secret, _ ...client.GetOption) error {
						*actual = *secret
						return nil
					})

				actual, _, err := GetClientAuthData(ctx, c, secretRef, true)

				Expect(err).NotTo(HaveOccurred())
				Expect(actual).To(Equal(clientAuth))
			})

			It("should retrieve the client auth data if DNS keys ar used", func() {
				var c = mockclient.NewMockClient(ctrl)
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *corev1.Secret, _ ...client.GetOption) error {
						*actual = *dnsSecret
						return nil
					})

				actual, _, err := GetClientAuthData(ctx, c, secretRef, true)

				Expect(err).NotTo(HaveOccurred())
				Expect(actual).To(Equal(clientAuth))
			})
		})
	})
})
