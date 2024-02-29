// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"

	. "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

const (
	subscriptionID = "b7ad693a-028a-422c-b064-d76c4586f2b3"
	tenantID       = "ee16e592-3035-41b9-a217-958f8f75b740"
	clientID       = "7fc4685d-3c33-40e6-b6bf-7857cab04300"
	clientSecret   = "clientSecret"
)

var _ = Describe("Secret validation", func() {

	DescribeTable("#ValidateCloudProviderSecret",
		func(data map[string][]byte, oldData map[string][]byte, matcher gomegatypes.GomegaMatcher) {
			var oldSecret *corev1.Secret
			secret := &corev1.Secret{
				Data: data,
			}
			if oldData != nil {
				oldSecret = &corev1.Secret{
					Data: oldData,
				}
			}
			err := ValidateCloudProviderSecret(secret, oldSecret)

			Expect(err).To(matcher)
		},

		Entry("should return error when the subscription ID field is missing",
			map[string][]byte{
				azure.TenantIDKey:     []byte(tenantID),
				azure.ClientIDKey:     []byte(clientID),
				azure.ClientSecretKey: []byte(clientSecret),
			},
			nil,
			HaveOccurred(),
		),

		Entry("should return error when the subscription ID is empty",
			map[string][]byte{
				azure.SubscriptionIDKey: {},
				azure.TenantIDKey:       []byte(tenantID),
				azure.ClientIDKey:       []byte(clientID),
				azure.ClientSecretKey:   []byte(clientSecret),
			},
			nil,
			HaveOccurred(),
		),

		Entry("should return error when the subscription ID is not a valid GUID",
			map[string][]byte{
				azure.SubscriptionIDKey: append([]byte(subscriptionID), ' '),
				azure.TenantIDKey:       []byte(tenantID),
				azure.ClientIDKey:       []byte(clientID),
				azure.ClientSecretKey:   []byte(clientSecret),
			},
			nil,
			HaveOccurred(),
		),

		Entry("should return error when the tenant ID field is missing",
			map[string][]byte{
				azure.SubscriptionIDKey: []byte(subscriptionID),
				azure.ClientIDKey:       []byte(clientID),
				azure.ClientSecretKey:   []byte(clientSecret),
			},
			nil,
			HaveOccurred(),
		),

		Entry("should return error when the tenant ID is empty",
			map[string][]byte{
				azure.SubscriptionIDKey: []byte(subscriptionID),
				azure.TenantIDKey:       {},
				azure.ClientIDKey:       []byte(clientID),
				azure.ClientSecretKey:   []byte(clientSecret),
			},
			nil,
			HaveOccurred(),
		),

		Entry("should return error when the tenant ID is not a valid GUID",
			map[string][]byte{
				azure.SubscriptionIDKey: []byte(subscriptionID),
				azure.TenantIDKey:       append([]byte(" "), []byte(tenantID)...),
				azure.ClientIDKey:       []byte(clientID),
				azure.ClientSecretKey:   []byte(clientSecret),
			},
			nil,
			HaveOccurred(),
		),

		Entry("should succeed when client ID and/or client Secret is not provided",
			map[string][]byte{
				azure.SubscriptionIDKey: []byte(subscriptionID),
				azure.TenantIDKey:       []byte(tenantID),
			},
			nil,
			BeNil(),
		),

		Entry("should return error when clientID is provided but no clientSecret",
			map[string][]byte{
				azure.SubscriptionIDKey: []byte(subscriptionID),
				azure.TenantIDKey:       []byte(tenantID),
				azure.ClientIDKey:       []byte(clientID),
			},
			nil,
			HaveOccurred(),
		),

		Entry("should return error when clientSecret is provided but no clientID",
			map[string][]byte{
				azure.SubscriptionIDKey: []byte(subscriptionID),
				azure.TenantIDKey:       []byte(tenantID),
				azure.ClientSecretKey:   []byte(clientSecret),
			},
			nil,
			HaveOccurred(),
		),

		Entry("should return error when the client ID is empty",
			map[string][]byte{
				azure.SubscriptionIDKey: []byte(subscriptionID),
				azure.TenantIDKey:       []byte(tenantID),
				azure.ClientIDKey:       {},
				azure.ClientSecretKey:   []byte(clientSecret),
			},
			nil,
			HaveOccurred(),
		),

		Entry("should return error when the client ID is not a valid GUID",
			map[string][]byte{
				azure.SubscriptionIDKey: []byte(subscriptionID),
				azure.TenantIDKey:       []byte(tenantID),
				azure.ClientIDKey:       []byte("foo"),
				azure.ClientSecretKey:   []byte(clientSecret),
			},
			nil,
			HaveOccurred(),
		),

		Entry("should return error when the client secret is empty",
			map[string][]byte{
				azure.SubscriptionIDKey: []byte(subscriptionID),
				azure.TenantIDKey:       []byte(tenantID),
				azure.ClientIDKey:       []byte(clientID),
				azure.ClientSecretKey:   {},
			},
			nil,
			HaveOccurred(),
		),

		Entry("should return error when the client secret contains a trailing new line",
			map[string][]byte{
				azure.SubscriptionIDKey: []byte(subscriptionID),
				azure.TenantIDKey:       []byte(tenantID),
				azure.ClientIDKey:       []byte(clientID),
				azure.ClientSecretKey:   append([]byte(clientSecret), '\n'),
			},
			nil,
			HaveOccurred(),
		),

		Entry("should succeed when the client credentials are valid",
			map[string][]byte{
				azure.SubscriptionIDKey: []byte(subscriptionID),
				azure.TenantIDKey:       []byte(tenantID),
				azure.ClientIDKey:       []byte(clientID),
				azure.ClientSecretKey:   []byte(clientSecret),
			},
			nil,
			BeNil(),
		),

		Entry("should return error when the subscription ID is changed",
			map[string][]byte{
				azure.SubscriptionIDKey: []byte(subscriptionID),
				azure.TenantIDKey:       []byte(tenantID),
				azure.ClientIDKey:       []byte(clientID),
				azure.ClientSecretKey:   []byte(clientSecret),
			},
			map[string][]byte{
				azure.SubscriptionIDKey: []byte("otherSubscriptionID"),
				azure.TenantIDKey:       []byte(tenantID),
				azure.ClientIDKey:       []byte(clientID),
				azure.ClientSecretKey:   []byte(clientSecret),
			},
			HaveOccurred(),
		),

		Entry("should return error when the tenant ID is changed",
			map[string][]byte{
				azure.SubscriptionIDKey: []byte(subscriptionID),
				azure.TenantIDKey:       []byte(tenantID),
				azure.ClientIDKey:       []byte(clientID),
				azure.ClientSecretKey:   []byte(clientSecret),
			},
			map[string][]byte{
				azure.SubscriptionIDKey: []byte(subscriptionID),
				azure.TenantIDKey:       []byte("otherTenantID"),
				azure.ClientIDKey:       []byte(clientID),
				azure.ClientSecretKey:   []byte(clientSecret),
			},
			HaveOccurred(),
		),

		Entry("should succeed when the client ID is changed",
			map[string][]byte{
				azure.SubscriptionIDKey: []byte(subscriptionID),
				azure.TenantIDKey:       []byte(tenantID),
				azure.ClientIDKey:       []byte(clientID),
				azure.ClientSecretKey:   []byte(clientSecret),
			},
			map[string][]byte{
				azure.SubscriptionIDKey: []byte(subscriptionID),
				azure.TenantIDKey:       []byte(tenantID),
				azure.ClientIDKey:       []byte("otherClientID"),
				azure.ClientSecretKey:   []byte(clientSecret),
			},
			BeNil(),
		),

		Entry("should succeed when the client secret is changed",
			map[string][]byte{
				azure.SubscriptionIDKey: []byte(subscriptionID),
				azure.TenantIDKey:       []byte(tenantID),
				azure.ClientIDKey:       []byte(clientID),
				azure.ClientSecretKey:   []byte(clientSecret),
			},
			map[string][]byte{
				azure.SubscriptionIDKey: []byte(subscriptionID),
				azure.TenantIDKey:       []byte(tenantID),
				azure.ClientIDKey:       []byte(clientID),
				azure.ClientSecretKey:   []byte("otherClientSecret"),
			},
			BeNil(),
		),
	)
})
