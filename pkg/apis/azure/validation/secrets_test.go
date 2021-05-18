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

package validation_test

import (
	. "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Secret validation", func() {

	DescribeTable("#ValidateCloudProviderSecret",
		func(data map[string][]byte, matcher gomegatypes.GomegaMatcher) {
			secret := &corev1.Secret{
				Data: data,
			}
			err := ValidateCloudProviderSecret(secret)

			Expect(err).To(matcher)
		},

		Entry("should return error when the subscription ID field is missing",
			map[string][]byte{
				azure.TenantIDKey:     []byte("tenant"),
				azure.ClientIDKey:     []byte("cliendID"),
				azure.ClientSecretKey: []byte("clientSecret"),
			},
			HaveOccurred(),
		),

		Entry("should return error when the subscription ID is empty",
			map[string][]byte{
				azure.SubscriptionIDKey: {},
				azure.TenantIDKey:       []byte("tenant"),
				azure.ClientIDKey:       []byte("cliendID"),
				azure.ClientSecretKey:   []byte("clientSecret"),
			},
			HaveOccurred(),
		),

		Entry("should return error when the tenant ID field is missing",
			map[string][]byte{
				azure.SubscriptionIDKey: []byte("subscription"),
				azure.ClientIDKey:       []byte("cliendID"),
				azure.ClientSecretKey:   []byte("clientSecret"),
			},
			HaveOccurred(),
		),

		Entry("should return error when the tenant ID is empty",
			map[string][]byte{
				azure.SubscriptionIDKey: []byte("subscription"),
				azure.TenantIDKey:       {},
				azure.ClientIDKey:       []byte("cliendID"),
				azure.ClientSecretKey:   []byte("clientSecret"),
			},
			HaveOccurred(),
		),

		Entry("should return error when the client ID field is missing",
			map[string][]byte{
				azure.SubscriptionIDKey: []byte("subscription"),
				azure.TenantIDKey:       []byte("tenant"),
				azure.ClientSecretKey:   []byte("clientSecret"),
			},
			HaveOccurred(),
		),

		Entry("should return error when the client ID is empty",
			map[string][]byte{
				azure.SubscriptionIDKey: []byte("subscription"),
				azure.TenantIDKey:       []byte("tenant"),
				azure.ClientIDKey:       {},
				azure.ClientSecretKey:   []byte("clientSecret"),
			},
			HaveOccurred(),
		),

		Entry("should return error when the client secret field is missing",
			map[string][]byte{
				azure.SubscriptionIDKey: []byte("subscription"),
				azure.TenantIDKey:       []byte("tenant"),
				azure.ClientIDKey:       []byte("cliendID"),
			},
			HaveOccurred(),
		),

		Entry("should return error when the client secret is empty",
			map[string][]byte{
				azure.SubscriptionIDKey: []byte("subscription"),
				azure.TenantIDKey:       []byte("tenant"),
				azure.ClientIDKey:       []byte("cliendID"),
				azure.ClientSecretKey:   {},
			},
			HaveOccurred(),
		),

		Entry("should succeed when the client credentials are valid",
			map[string][]byte{
				azure.SubscriptionIDKey: []byte("subscription"),
				azure.TenantIDKey:       []byte("tenant"),
				azure.ClientIDKey:       []byte("cliendID"),
				azure.ClientSecretKey:   []byte("clientSecret"),
			},
			BeNil(),
		),
	)
})
