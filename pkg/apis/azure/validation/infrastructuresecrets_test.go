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
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

var _ = Describe("Infrastructure secrets validation", func() {
	var (
		secret    *corev1.Secret
		oldSecret *corev1.Secret
		fldPath   *field.Path

		subscriptionIDValue        = "a6ad693a-028a-422c-b064-d76a4586f2b3"
		tenantIDValue              = "ee16e593-3035-41b9-a217-958f8f75b750"
		clientIDValue              = "7fc4685e-3c33-40e6-b6bf-7857cab04390"
		clientSecretValue          = "my-client-secret"
		clientSecretNewValue       = "new-secret"
		differentSubscriptionValue = "11111111-2222-3333-4444-555555555555"
		differentTenantValue       = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
		invalidGUIDValue           = "invalid-guid"
		clientSecretWithNewline    = "secret\n"
		namespace                  = "test-namespace"
		secretName                 = "test-secret"
	)

	BeforeEach(func() {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
			Data: make(map[string][]byte),
		}
		oldSecret = nil
		fldPath = field.NewPath("secret")
	})

	Describe("ValidateCloudProviderSecret", func() {
		It("should pass with valid minimal infrastructure credentials", func() {
			secret.Data[azure.SubscriptionIDKey] = []byte(subscriptionIDValue)
			secret.Data[azure.TenantIDKey] = []byte(tenantIDValue)

			errs := ValidateCloudProviderSecret(secret, nil, fldPath)
			Expect(errs).To(BeEmpty())
		})

		It("should pass with valid complete infrastructure credentials", func() {
			secret.Data[azure.SubscriptionIDKey] = []byte(subscriptionIDValue)
			secret.Data[azure.TenantIDKey] = []byte(tenantIDValue)
			secret.Data[azure.ClientIDKey] = []byte(clientIDValue)
			secret.Data[azure.ClientSecretKey] = []byte(clientSecretValue)

			errs := ValidateCloudProviderSecret(secret, nil, fldPath)
			Expect(errs).To(BeEmpty())
		})

		It("should fail when required subscriptionID is missing", func() {
			secret.Data[azure.TenantIDKey] = []byte(tenantIDValue)

			errs := ValidateCloudProviderSecret(secret, nil, fldPath)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Type).To(Equal(field.ErrorTypeRequired))
			expected := fmt.Sprintf("missing required field %q in secret %s/%s", azure.SubscriptionIDKey, namespace, secretName)
			Expect(errs[0].Field).To(Equal("secret.data[subscriptionID]"))
			Expect(errs[0].Detail).To(Equal(expected))
		})

		It("should fail when required tenantID is missing", func() {
			secret.Data[azure.SubscriptionIDKey] = []byte(subscriptionIDValue)

			errs := ValidateCloudProviderSecret(secret, nil, fldPath)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Type).To(Equal(field.ErrorTypeRequired))
			expected := fmt.Sprintf("missing required field %q in secret %s/%s", azure.TenantIDKey, namespace, secretName)
			Expect(errs[0].Field).To(Equal("secret.data[tenantID]"))
			Expect(errs[0].Detail).To(Equal(expected))
		})

		It("should fail when subscriptionID has invalid GUID format", func() {
			secret.Data[azure.SubscriptionIDKey] = []byte(invalidGUIDValue)
			secret.Data[azure.TenantIDKey] = []byte(tenantIDValue)

			errs := ValidateCloudProviderSecret(secret, nil, fldPath)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Type).To(Equal(field.ErrorTypeInvalid))
			expected := fmt.Sprintf("field %q must be a valid GUID in secret %s/%s", azure.SubscriptionIDKey, namespace, secretName)
			Expect(errs[0].Field).To(Equal("secret.data[subscriptionID]"))
			Expect(errs[0].Detail).To(Equal(expected))
			Expect(errs[0].BadValue).To(Equal("(hidden)"))
		})

		It("should fail when tenantID has invalid GUID format", func() {
			secret.Data[azure.SubscriptionIDKey] = []byte(subscriptionIDValue)
			secret.Data[azure.TenantIDKey] = []byte(invalidGUIDValue)

			errs := ValidateCloudProviderSecret(secret, nil, fldPath)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Type).To(Equal(field.ErrorTypeInvalid))
			expected := fmt.Sprintf("field %q must be a valid GUID in secret %s/%s", azure.TenantIDKey, namespace, secretName)
			Expect(errs[0].Field).To(Equal("secret.data[tenantID]"))
			Expect(errs[0].Detail).To(Equal(expected))
			Expect(errs[0].BadValue).To(Equal("(hidden)"))
		})

		It("should fail when clientSecret contains trailing newline (whitespace)", func() {
			secret.Data[azure.SubscriptionIDKey] = []byte(subscriptionIDValue)
			secret.Data[azure.TenantIDKey] = []byte(tenantIDValue)
			secret.Data[azure.ClientIDKey] = []byte(clientIDValue)
			secret.Data[azure.ClientSecretKey] = []byte(clientSecretWithNewline)

			errs := ValidateCloudProviderSecret(secret, nil, fldPath)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Type).To(Equal(field.ErrorTypeInvalid))
			expected := fmt.Sprintf("field %q must not contain leading or trailing whitespace in secret %s/%s", azure.ClientSecretKey, namespace, secretName)
			Expect(errs[0].Field).To(Equal("secret.data[clientSecret]"))
			Expect(errs[0].Detail).To(Equal(expected))
			Expect(errs[0].BadValue).To(Equal("(hidden)"))
		})

		It("should fail when unexpected field is present", func() {
			secret.Data[azure.SubscriptionIDKey] = []byte(subscriptionIDValue)
			secret.Data[azure.TenantIDKey] = []byte(tenantIDValue)
			secret.Data["UNEXPECTED_FIELD"] = []byte("value")

			errs := ValidateCloudProviderSecret(secret, nil, fldPath)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Type).To(Equal(field.ErrorTypeForbidden))
			expected := fmt.Sprintf("unexpected field %q in secret %s/%s", "UNEXPECTED_FIELD", namespace, secretName)
			Expect(errs[0].Field).To(Equal("secret.data[UNEXPECTED_FIELD]"))
			Expect(errs[0].Detail).To(Equal(expected))
		})

		It("should fail when trying to change immutable subscriptionID", func() {
			oldSecret = &corev1.Secret{
				Data: map[string][]byte{
					azure.SubscriptionIDKey: []byte(subscriptionIDValue),
					azure.TenantIDKey:       []byte(tenantIDValue),
				},
			}
			secret.Data[azure.SubscriptionIDKey] = []byte(differentSubscriptionValue)
			secret.Data[azure.TenantIDKey] = []byte(tenantIDValue)

			errs := ValidateCloudProviderSecret(secret, oldSecret, fldPath)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Type).To(Equal(field.ErrorTypeInvalid))
			expected := fmt.Sprintf("field %q must not be changed for existing shoot clusters in secret %s/%s", azure.SubscriptionIDKey, namespace, secretName)
			Expect(errs[0].Field).To(Equal("secret.data[subscriptionID]"))
			Expect(errs[0].Detail).To(Equal(expected))
			Expect(errs[0].BadValue).To(Equal("(hidden)"))
		})

		It("should fail when trying to change immutable tenantID", func() {
			oldSecret = &corev1.Secret{
				Data: map[string][]byte{
					azure.SubscriptionIDKey: []byte(subscriptionIDValue),
					azure.TenantIDKey:       []byte(tenantIDValue),
				},
			}
			secret.Data[azure.SubscriptionIDKey] = []byte(subscriptionIDValue)
			secret.Data[azure.TenantIDKey] = []byte(differentTenantValue)

			errs := ValidateCloudProviderSecret(secret, oldSecret, fldPath)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Type).To(Equal(field.ErrorTypeInvalid))
			expected := fmt.Sprintf("field %q must not be changed for existing shoot clusters in secret %s/%s", azure.TenantIDKey, namespace, secretName)
			Expect(errs[0].Field).To(Equal("secret.data[tenantID]"))
			Expect(errs[0].Detail).To(Equal(expected))
			Expect(errs[0].BadValue).To(Equal("(hidden)"))
		})

		It("should allow changing mutable fields like clientSecret", func() {
			oldSecret = &corev1.Secret{
				Data: map[string][]byte{
					azure.SubscriptionIDKey: []byte(subscriptionIDValue),
					azure.TenantIDKey:       []byte(tenantIDValue),
					azure.ClientIDKey:       []byte(clientIDValue),
					azure.ClientSecretKey:   []byte(clientSecretValue),
				},
			}
			secret.Data[azure.SubscriptionIDKey] = []byte(subscriptionIDValue)
			secret.Data[azure.TenantIDKey] = []byte(tenantIDValue)
			secret.Data[azure.ClientIDKey] = []byte(clientIDValue)
			secret.Data[azure.ClientSecretKey] = []byte(clientSecretNewValue)

			errs := ValidateCloudProviderSecret(secret, oldSecret, fldPath)
			Expect(errs).To(BeEmpty())
		})
	})
})
