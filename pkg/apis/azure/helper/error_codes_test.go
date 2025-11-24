// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
)

var _ = Describe("#ErrorCodes", func() {
	Context("configurationProblemRegexp", func() {
		It("should match hypervisor generation error", func() {
			errorMsg := "The selected VM size 'Standard_D16as_v6' cannot boot Hypervisor Generation '1'."
			Expect(KnownCodes[gardencorev1beta1.ErrorConfigurationProblem](errorMsg)).To(BeTrue())
		})

		It("should not match unrelated error", func() {
			msg := "Some other error message"
			Expect(KnownCodes[gardencorev1beta1.ErrorConfigurationProblem](msg)).To(BeFalse())
		})
	})
})
