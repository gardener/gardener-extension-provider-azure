// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"
)

var _ = Describe("Credential validation helpers", func() {
	var (
		secret    *corev1.Secret
		oldSecret *corev1.Secret
		fldPath   *field.Path
		mapping   CredentialMapping

		subscriptionIDValue          = "a6ad693a-028a-422c-b064-d76a4586f2b3"
		tenantIDValue                = "ee16e593-3035-41b9-a217-958f8f75b750"
		clientIDValue                = "7fc4685e-3c33-40e6-b6bf-7857cab04390"
		regionValue                  = "westeurope"
		subscriptionIDWithWhitespace = " a6ad693a-028a-422c-b064-d76a4586f2b3 "
		invalidGUIDValue             = "invalid-guid"
		oldSubscriptionIDValue       = "old-subscription"
		oldTenantIDValue             = "old-tenant"
		newSubscriptionIDValue       = "new-subscription"
		testFieldAllowedValue        = "value2"
		testFieldInvalidValue        = "invalid-value"
	)

	BeforeEach(func() {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: "test-namespace",
			},
			Data: make(map[string][]byte),
		}
		oldSecret = nil
		fldPath = field.NewPath("data")
		mapping = CredentialMapping{
			RequiredFields: map[string]string{
				"subscriptionID": "SUBSCRIPTION_ID",
				"tenantID":       "TENANT_ID",
			},
			OptionalFields: map[string]string{
				"clientID":     "CLIENT_ID",
				"clientSecret": "CLIENT_SECRET",
				"region":       "REGION",
			},
			GUIDFields: map[string]string{
				"subscriptionID": "SUBSCRIPTION_ID",
				"tenantID":       "TENANT_ID",
				"clientID":       "CLIENT_ID",
			},
			ImmutableFields: map[string]string{
				"subscriptionID": "SUBSCRIPTION_ID",
				"tenantID":       "TENANT_ID",
			},
		}
	})

	Describe("validateRequiredCredentials", func() {
		It("should pass when all required fields are present and non-empty", func() {
			secret.Data["SUBSCRIPTION_ID"] = []byte(subscriptionIDValue)
			secret.Data["TENANT_ID"] = []byte(tenantIDValue)

			errs := ValidateRequiredCredentials(secret, mapping, fldPath)
			Expect(errs).To(BeEmpty())
		})

		It("should return error when required field is missing", func() {
			secret.Data["TENANT_ID"] = []byte(tenantIDValue)

			errs := ValidateRequiredCredentials(secret, mapping, fldPath)
			Expect(errs).To(HaveLen(1))
			expectedDetail := fmt.Sprintf("missing required field subscriptionID in secret %v/%v", secret.Namespace, secret.Name)
			Expect(errs[0].Type).To(Equal(field.ErrorTypeRequired))
			Expect(errs[0].Field).To(Equal("data[SUBSCRIPTION_ID]"))
			Expect(errs[0].Detail).To(Equal(expectedDetail))
		})

		It("should return error when required field is empty", func() {
			secret.Data["SUBSCRIPTION_ID"] = []byte("")
			secret.Data["TENANT_ID"] = []byte(tenantIDValue)

			errs := ValidateRequiredCredentials(secret, mapping, fldPath)
			Expect(errs).To(HaveLen(1))
			expectedDetail := fmt.Sprintf("field subscriptionID cannot be empty in secret %v/%v", secret.Namespace, secret.Name)
			Expect(errs[0].Type).To(Equal(field.ErrorTypeInvalid))
			Expect(errs[0].Field).To(Equal("data[SUBSCRIPTION_ID]"))
			Expect(errs[0].Detail).To(Equal(expectedDetail))
		})
	})

	Describe("validateOptionalCredentials", func() {
		It("should pass when optional fields are present and non-empty", func() {
			secret.Data["CLIENT_ID"] = []byte(clientIDValue)
			secret.Data["REGION"] = []byte(regionValue)

			errs := ValidateOptionalCredentials(secret, mapping, fldPath)
			Expect(errs).To(BeEmpty())
		})

		It("should pass when optional fields are missing", func() {
			errs := ValidateOptionalCredentials(secret, mapping, fldPath)
			Expect(errs).To(BeEmpty())
		})

		It("should return error when optional field is empty", func() {
			secret.Data["CLIENT_ID"] = []byte("")

			errs := ValidateOptionalCredentials(secret, mapping, fldPath)
			Expect(errs).To(HaveLen(1))
			expectedDetail := fmt.Sprintf("field clientID cannot be empty if specified in secret %v/%v", secret.Namespace, secret.Name)
			Expect(errs[0].Type).To(Equal(field.ErrorTypeInvalid))
			Expect(errs[0].Detail).To(Equal(expectedDetail))
		})
	})

	Describe("validateCredentialFormats", func() {
		It("should pass when GUID fields have valid format", func() {
			secret.Data["SUBSCRIPTION_ID"] = []byte(subscriptionIDValue)
			secret.Data["TENANT_ID"] = []byte(tenantIDValue)
			secret.Data["CLIENT_ID"] = []byte(clientIDValue)

			errs := ValidateCredentialFormats(secret, mapping, fldPath)
			Expect(errs).To(BeEmpty())
		})

		It("should return error when GUID field has invalid format", func() {
			secret.Data["SUBSCRIPTION_ID"] = []byte(invalidGUIDValue)

			errs := ValidateCredentialFormats(secret, mapping, fldPath)
			Expect(errs).To(HaveLen(1))
			expectedDetail := fmt.Sprintf("field subscriptionID must be a valid GUID in secret %v/%v", secret.Namespace, secret.Name)
			Expect(errs[0].Type).To(Equal(field.ErrorTypeInvalid))
			Expect(errs[0].Field).To(Equal("data[SUBSCRIPTION_ID]"))
			Expect(errs[0].BadValue).To(Equal("(hidden)"))
			Expect(errs[0].Detail).To(Equal(expectedDetail))
		})

		It("should return error when field contains leading/trailing whitespace", func() {
			secret.Data["SUBSCRIPTION_ID"] = []byte(subscriptionIDWithWhitespace)

			errs := ValidateCredentialFormats(secret, mapping, fldPath)
			Expect(errs).To(HaveLen(2))
			expectedDetailGUID := fmt.Sprintf("field subscriptionID must be a valid GUID in secret %v/%v", secret.Namespace, secret.Name)
			expectedDetailWhitespace := fmt.Sprintf("field subscriptionID must not contain leading or trailing whitespace in secret %v/%v", secret.Namespace, secret.Name)
			Expect(errs[0].Detail).To(Equal(expectedDetailGUID))
			Expect(errs[1].Type).To(Equal(field.ErrorTypeInvalid))
			Expect(errs[1].Field).To(Equal("data[SUBSCRIPTION_ID]"))
			Expect(errs[1].Detail).To(Equal(expectedDetailWhitespace))
		})
	})

	Describe("validateNoUnexpectedFields", func() {
		It("should pass when only expected fields are present", func() {
			secret.Data["SUBSCRIPTION_ID"] = []byte(subscriptionIDValue)
			secret.Data["CLIENT_ID"] = []byte(clientIDValue)

			errs := ValidateNoUnexpectedFields(secret, mapping, fldPath)
			Expect(errs).To(BeEmpty())
		})

		It("should return error when unexpected field is present", func() {
			secret.Data["SUBSCRIPTION_ID"] = []byte(subscriptionIDValue)
			secret.Data["UNEXPECTED_FIELD"] = []byte("value")

			errs := ValidateNoUnexpectedFields(secret, mapping, fldPath)
			Expect(errs).To(HaveLen(1))
			expectedDetail := fmt.Sprintf("unexpected field \"UNEXPECTED_FIELD\" in secret %v/%v", secret.Namespace, secret.Name)
			Expect(errs[0].Type).To(Equal(field.ErrorTypeForbidden))
			Expect(errs[0].Field).To(Equal("data[UNEXPECTED_FIELD]"))
			Expect(errs[0].Detail).To(Equal(expectedDetail))
		})
	})

	Describe("validateImmutableCredentials", func() {
		oldSubscriptionID := []byte(oldSubscriptionIDValue)
		oldTenantID := []byte(oldTenantIDValue)
		BeforeEach(func() {
			oldSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "old-secret",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"SUBSCRIPTION_ID": oldSubscriptionID,
					"TENANT_ID":       oldTenantID,
				},
			}
		})

		It("should pass when immutable fields haven't changed", func() {
			secret.Data["SUBSCRIPTION_ID"] = oldSubscriptionID
			secret.Data["TENANT_ID"] = oldTenantID

			errs := ValidateImmutableCredentials(secret, oldSecret, mapping, "test resources", fldPath)
			Expect(errs).To(BeEmpty())
		})

		It("should return error when immutable field has changed", func() {
			secret.Data["SUBSCRIPTION_ID"] = []byte(newSubscriptionIDValue)
			secret.Data["TENANT_ID"] = oldTenantID

			errs := ValidateImmutableCredentials(secret, oldSecret, mapping, "test resources", fldPath)
			Expect(errs).To(HaveLen(1))
			expectedDetail := fmt.Sprintf("field subscriptionID must not be changed for existing test resources in secret %v/%v", secret.Namespace, secret.Name)
			Expect(errs[0].Type).To(Equal(field.ErrorTypeInvalid))
			Expect(errs[0].Field).To(Equal("data[SUBSCRIPTION_ID]"))
			Expect(errs[0].BadValue).To(Equal("(hidden)"))
			Expect(errs[0].Detail).To(Equal(expectedDetail))
		})
	})

	Describe("validatePredefinedValues", func() {
		allowedValues := []string{"value1", "value2", "value3"}

		It("should pass when field contains allowed value", func() {
			secret.Data["TEST_FIELD"] = []byte(testFieldAllowedValue)

			errs := ValidatePredefinedValues(secret, "TEST_FIELD", allowedValues, fldPath)
			Expect(errs).To(BeEmpty())
		})

		It("should pass when field is missing", func() {
			errs := ValidatePredefinedValues(secret, "TEST_FIELD", allowedValues, fldPath)
			Expect(errs).To(BeEmpty())
		})

		It("should pass when field is empty", func() {
			secret.Data["TEST_FIELD"] = []byte("")

			errs := ValidatePredefinedValues(secret, "TEST_FIELD", allowedValues, fldPath)
			Expect(errs).To(BeEmpty())
		})

		It("should return error when field contains disallowed value", func() {
			secret.Data["TEST_FIELD"] = []byte(testFieldInvalidValue)

			errs := ValidatePredefinedValues(secret, "TEST_FIELD", allowedValues, fldPath)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Type).To(Equal(field.ErrorTypeNotSupported))
			Expect(errs[0].Field).To(Equal("data[TEST_FIELD]"))
			Expect(errs[0].BadValue).To(Equal(testFieldInvalidValue))
		})
	})
})
