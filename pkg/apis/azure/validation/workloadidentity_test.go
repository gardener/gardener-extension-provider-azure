// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"
)

var _ = Describe("#ValidateWorkloadIdentityConfig", func() {
	var (
		workloadIdentityConfig *apisazure.WorkloadIdentityConfig
	)

	BeforeEach(func() {
		workloadIdentityConfig = &apisazure.WorkloadIdentityConfig{
			ClientID:       uuid.NewString(),
			TenantID:       uuid.NewString(),
			SubscriptionID: uuid.NewString(),
		}
	})

	It("should validate the config successfully", func() {
		Expect(validation.ValidateWorkloadIdentityConfig(workloadIdentityConfig, field.NewPath(""))).To(BeEmpty())
	})

	It("should contain all expected validation errors", func() {
		workloadIdentityConfig.ClientID = ""
		workloadIdentityConfig.TenantID = ""
		workloadIdentityConfig.SubscriptionID = ""
		errorList := validation.ValidateWorkloadIdentityConfig(workloadIdentityConfig, field.NewPath("providerConfig"))
		Expect(errorList).To(ConsistOfFields(
			Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("providerConfig.clientID"),
				"Detail": Equal("clientID is required"),
			},
			Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("providerConfig.tenantID"),
				"Detail": Equal("tenantID is required"),
			},
			Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("providerConfig.subscriptionID"),
				"Detail": Equal("subscriptionID is required"),
			},
			Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal("providerConfig.clientID"),
				"BadValue": Equal(""),
				"Detail":   Equal("clientID should be a valid GUID"),
			},
			Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal("providerConfig.tenantID"),
				"BadValue": Equal(""),
				"Detail":   Equal("tenantID should be a valid GUID"),
			},
			Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal("providerConfig.subscriptionID"),
				"BadValue": Equal(""),
				"Detail":   Equal("subscriptionID should be a valid GUID"),
			},
		))
	})

	It("should validate the config successfully during update", func() {
		newConfig := workloadIdentityConfig.DeepCopy()
		Expect(validation.ValidateWorkloadIdentityConfigUpdate(workloadIdentityConfig, newConfig, field.NewPath(""))).To(BeEmpty())
	})

	It("should allow changing the clientID during update", func() {
		newConfig := workloadIdentityConfig.DeepCopy()
		newConfig.ClientID = uuid.NewString()
		errorList := validation.ValidateWorkloadIdentityConfigUpdate(workloadIdentityConfig, newConfig, field.NewPath("providerConfig"))
		Expect(errorList).To(BeEmpty())
	})

	It("should not allow changing the tenantID or the subscriptionID during update", func() {
		newConfig := workloadIdentityConfig.DeepCopy()
		newConfig.TenantID = uuid.NewString()
		newConfig.SubscriptionID = uuid.NewString()
		errorList := validation.ValidateWorkloadIdentityConfigUpdate(workloadIdentityConfig, newConfig, field.NewPath("providerConfig"))
		Expect(errorList).To(ConsistOfFields(
			Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("providerConfig.tenantID"),
				"Detail": Equal("field is immutable"),
			},
			Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("providerConfig.subscriptionID"),
				"Detail": Equal("field is immutable"),
			},
		))
	})
})
