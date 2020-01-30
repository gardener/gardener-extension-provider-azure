// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package internal

import (
	"context"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	mockclient "github.com/gardener/gardener-extensions/pkg/mock/controller-runtime/client"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Service Account", func() {
	var (
		ctrl *gomock.Controller

		clientAuth *ClientAuth
		secret     *corev1.Secret
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
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#ReadClientAuthDataFromSecret", func() {
		It("should read the client auth data from the secret", func() {
			actual, err := ReadClientAuthDataFromSecret(secret)
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(clientAuth))
		})
	})

	Describe("#GetClientAuthData", func() {
		It("should retrieve the client auth data", func() {
			var (
				c         = mockclient.NewMockClient(ctrl)
				namespace = "foo"
				name      = "bar"
				secretRef = corev1.SecretReference{
					Namespace: namespace,
					Name:      name,
				}
				ctx = context.TODO()
			)
			c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *corev1.Secret) error {
					*actual = *secret
					return nil
				})

			actual, err := GetClientAuthData(ctx, c, secretRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(clientAuth))
		})
	})
})
