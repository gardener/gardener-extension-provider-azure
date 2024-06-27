// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
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
})
