// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
)

var _ = Describe("Scheme", func() {
	DescribeTable("#InfrastructureStateFromRaw",
		func(raw *runtime.RawExtension, expectedState *api.InfrastructureState, expectedErr bool) {
			state, err := InfrastructureStateFromRaw(raw)
			if expectedErr {
				Expect(err).To(HaveOccurred())
				Expect(state).To(BeNil())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(state).To(Equal(expectedState))
			}
		},
		Entry("when state is nil", nil, &api.InfrastructureState{Data: map[string]string{}, ManagedItems: []api.AzureResource{}}, false),
		Entry("when state is invalid JSON", &runtime.RawExtension{Raw: []byte(`foo`)}, nil, true),
		Entry("when state has an unsupported API version", &runtime.RawExtension{Raw: []byte(`{"apiVersion":"foo.bar/v1alpha1","kind":"InfrastructureState"}`)}, nil, true),
		Entry("when state has a different registered kind", &runtime.RawExtension{Raw: []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1","kind":"InfrastructureConfig"}`)}, nil, true),
		Entry("when state is a legacy Terraform envelope", &runtime.RawExtension{Raw: []byte(`{"terraformState":{"data":"{}","encoding":"none"},"savedProviderStatus":{}}`)}, nil, true),
		Entry("when state has no type metadata", &runtime.RawExtension{Raw: []byte(`{"data":{"resources_exist":"true"}}`)}, nil, true),
		Entry("when state is empty", &runtime.RawExtension{Raw: []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1","kind":"InfrastructureState"}`)}, &api.InfrastructureState{Data: map[string]string{}, ManagedItems: []api.AzureResource{}}, false),
		Entry("when state contains resources and a saved layout annotation", &runtime.RawExtension{Raw: []byte(`{
			"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1",
			"kind":"InfrastructureState",
			"data":{"resources_exist":"true","zoneMigration":"2"},
			"managedItems":[{"kind":"Microsoft.Resources/resourceGroups","id":"/subscriptions/sub/resourceGroups/shoot"}]
		}`)}, &api.InfrastructureState{
			Data:         map[string]string{"resources_exist": "true", "zoneMigration": "2"},
			ManagedItems: []api.AzureResource{{Kind: "Microsoft.Resources/resourceGroups", ID: "/subscriptions/sub/resourceGroups/shoot"}},
		}, false),
	)

	DescribeTable("#WorkloadIdentityConfigFromBytes",
		func(config []byte, expectedConfig *api.WorkloadIdentityConfig, expectedErr bool) {
			result, err := WorkloadIdentityConfigFromBytes(config)
			if expectedErr {
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(expectedConfig))
			}
		},
		Entry("when config is empty", []byte{}, nil, true),
		Entry("when config is invalid json", []byte(`invalid-json`), nil, true),
		Entry("when config is valid but has wrong apiVersion", []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v0","kind":"WorkloadIdentityConfig"}`), nil, true),
		Entry("when config is valid but has wrong kind", []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1","kind":"WorkloadIdentityConfiguration"}`), nil, true),
		Entry("when config is valid but missing required fields", []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1","kind":"WorkloadIdentityConfig"}`), &api.WorkloadIdentityConfig{}, false),
		Entry("when config is valid with all fields", []byte(`{
			"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1",
			"kind":"WorkloadIdentityConfig",
			"clientID":"client-id-123",
			"tenantID":"tenant-id-456",
			"subscriptionID":"subscription-id-789"
		}`), &api.WorkloadIdentityConfig{
			ClientID:       "client-id-123",
			TenantID:       "tenant-id-456",
			SubscriptionID: "subscription-id-789",
		}, false),
	)

	DescribeTable("#WorkloadIdentityConfigFromRaw",
		func(raw *runtime.RawExtension, expectedConfig *api.WorkloadIdentityConfig, expectedErr bool) {
			result, err := WorkloadIdentityConfigFromRaw(raw)
			if expectedErr {
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(expectedConfig))
			}
		},
		Entry("when raw is nil", nil, nil, true),
		Entry("when raw.Raw is nil", &runtime.RawExtension{Raw: nil}, nil, true),
		Entry("when raw is valid", &runtime.RawExtension{Raw: []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1","kind":"WorkloadIdentityConfig"}`)}, &api.WorkloadIdentityConfig{}, false),
	)
})
