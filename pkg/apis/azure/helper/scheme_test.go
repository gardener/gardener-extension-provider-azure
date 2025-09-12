// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
)

var _ = Describe("Scheme", func() {
	DescribeTable("#HasFlowState",
		func(state *runtime.RawExtension, expectedHasFlowState, expectedErr bool) {
			infraStatus := extensionsv1alpha1.InfrastructureStatus{
				DefaultStatus: extensionsv1alpha1.DefaultStatus{
					State: state,
				},
			}
			hasFlowState, err := HasFlowState(infraStatus)
			expectResults(hasFlowState, expectedHasFlowState, err, expectedErr)
		},
		Entry("when state is nil", nil, false, false),
		Entry("when state is invalid json", &runtime.RawExtension{Raw: []byte(`foo`)}, false, true),
		Entry("when state is not in 'azure.provider.extensions.gardener.cloud/v1alpha1' group version", &runtime.RawExtension{Raw: []byte(`{"apiVersion":"foo.bar/v1alpha1","kind":"InfrastructureState"}`)}, false, false),
		Entry("when state is in 'azure.provider.extensions.gardener.cloud/v1alpha1' group version", &runtime.RawExtension{Raw: []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1","kind":"InfrastructureState"}`)}, true, false),
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
		Entry("when config is valid but has wrong apiVersion", []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v0","kind":"WorkloadIdentityConfig"}`), &api.WorkloadIdentityConfig{}, true),
		Entry("when config is valid but has wrong kind", []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1","kind":"WorkloadIdentityConfiguration"}`), &api.WorkloadIdentityConfig{}, true),
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
