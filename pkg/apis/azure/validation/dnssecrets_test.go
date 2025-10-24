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

var _ = Describe("DNS secrets validation", func() {
	var (
		secret  *corev1.Secret
		fldPath *field.Path

		subscriptionIDValue    = "a6ad693a-028a-422c-b064-d76a4586f2b3"
		tenantIDValue          = "ee16e593-3035-41b9-a217-958f8f75b750"
		clientIDValue          = "7fc4685e-3c33-40e6-b6bf-7857cab04390"
		clientSecretValue      = "my-dns-secret"
		clientSecretNewline    = "secret\n"
		invalidGUIDValue       = "invalid-guid"
		invalidAzureCloudValue = "InvalidCloud"
		namespace              = "test-namespace"
		secretName             = "dns-secret"
	)

	BeforeEach(func() {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
			Data: map[string][]byte{},
		}
		fldPath = field.NewPath("dnsRecord").Child("spec").Child("secretRef")
	})

	Describe("ValidateDNSProviderSecret", func() {
		It("should pass with valid minimal DNS credentials", func() {
			secret.Data[azure.DNSSubscriptionIDKey] = []byte(subscriptionIDValue)
			secret.Data[azure.DNSTenantIDKey] = []byte(tenantIDValue)
			secret.Data[azure.DNSClientIDKey] = []byte(clientIDValue)
			secret.Data[azure.DNSClientSecretKey] = []byte(clientSecretValue)

			errs := ValidateDNSProviderSecret(secret, fldPath)
			Expect(errs).To(BeEmpty())
		})

		It("should pass with valid complete DNS credentials", func() {
			secret.Data[azure.DNSSubscriptionIDKey] = []byte(subscriptionIDValue)
			secret.Data[azure.DNSTenantIDKey] = []byte(tenantIDValue)
			secret.Data[azure.DNSClientIDKey] = []byte(clientIDValue)
			secret.Data[azure.DNSClientSecretKey] = []byte(clientSecretValue)
			secret.Data[azure.DNSAzureCloud] = []byte("AzurePublic")

			errs := ValidateDNSProviderSecret(secret, fldPath)
			Expect(errs).To(BeEmpty())
		})

		It("should fail when required AZURE_SUBSCRIPTION_ID is missing", func() {
			secret.Data[azure.DNSTenantIDKey] = []byte(tenantIDValue)
			secret.Data[azure.DNSClientIDKey] = []byte(clientIDValue)
			secret.Data[azure.DNSClientSecretKey] = []byte(clientSecretValue)

			errs := ValidateDNSProviderSecret(secret, fldPath)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Type).To(Equal(field.ErrorTypeRequired))
			expected := fmt.Sprintf("missing required field subscriptionID in secret %s/%s", namespace, secretName)
			Expect(errs[0].Field).To(Equal("dnsRecord.spec.secretRef.data[AZURE_SUBSCRIPTION_ID]"))
			Expect(errs[0].Detail).To(Equal(expected))
		})

		It("should fail when required AZURE_TENANT_ID is missing", func() {
			secret.Data[azure.DNSSubscriptionIDKey] = []byte(subscriptionIDValue)
			secret.Data[azure.DNSClientIDKey] = []byte(clientIDValue)
			secret.Data[azure.DNSClientSecretKey] = []byte(clientSecretValue)

			errs := ValidateDNSProviderSecret(secret, fldPath)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Type).To(Equal(field.ErrorTypeRequired))
			expected := fmt.Sprintf("missing required field tenantID in secret %s/%s", namespace, secretName)
			Expect(errs[0].Field).To(Equal("dnsRecord.spec.secretRef.data[AZURE_TENANT_ID]"))
			Expect(errs[0].Detail).To(Equal(expected))
		})

		It("should fail when required AZURE_CLIENT_ID is missing", func() {
			secret.Data[azure.DNSSubscriptionIDKey] = []byte(subscriptionIDValue)
			secret.Data[azure.DNSTenantIDKey] = []byte(tenantIDValue)
			secret.Data[azure.DNSClientSecretKey] = []byte(clientSecretValue)

			errs := ValidateDNSProviderSecret(secret, fldPath)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Type).To(Equal(field.ErrorTypeRequired))
			expected := fmt.Sprintf("missing required field clientID in secret %s/%s", namespace, secretName)
			Expect(errs[0].Field).To(Equal("dnsRecord.spec.secretRef.data[AZURE_CLIENT_ID]"))
			Expect(errs[0].Detail).To(Equal(expected))
		})

		It("should fail when required AZURE_CLIENT_SECRET is missing", func() {
			secret.Data[azure.DNSSubscriptionIDKey] = []byte(subscriptionIDValue)
			secret.Data[azure.DNSTenantIDKey] = []byte(tenantIDValue)
			secret.Data[azure.DNSClientIDKey] = []byte(clientIDValue)

			errs := ValidateDNSProviderSecret(secret, fldPath)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Type).To(Equal(field.ErrorTypeRequired))
			expected := fmt.Sprintf("missing required field clientSecret in secret %s/%s", namespace, secretName)
			Expect(errs[0].Field).To(Equal("dnsRecord.spec.secretRef.data[AZURE_CLIENT_SECRET]"))
			Expect(errs[0].Detail).To(Equal(expected))
		})

		It("should fail when AZURE_SUBSCRIPTION_ID has invalid GUID format", func() {
			secret.Data[azure.DNSSubscriptionIDKey] = []byte(invalidGUIDValue)
			secret.Data[azure.DNSTenantIDKey] = []byte(tenantIDValue)
			secret.Data[azure.DNSClientIDKey] = []byte(clientIDValue)
			secret.Data[azure.DNSClientSecretKey] = []byte(clientSecretValue)

			errs := ValidateDNSProviderSecret(secret, fldPath)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Type).To(Equal(field.ErrorTypeInvalid))
			expected := fmt.Sprintf("field subscriptionID must be a valid GUID in secret %s/%s", namespace, secretName)
			Expect(errs[0].Field).To(Equal("dnsRecord.spec.secretRef.data[AZURE_SUBSCRIPTION_ID]"))
			Expect(errs[0].Detail).To(Equal(expected))
			Expect(errs[0].BadValue).To(Equal("(hidden)"))
		})

		It("should fail when AZURE_CLIENT_SECRET contains trailing newline (whitespace)", func() {
			secret.Data[azure.DNSSubscriptionIDKey] = []byte(subscriptionIDValue)
			secret.Data[azure.DNSTenantIDKey] = []byte(tenantIDValue)
			secret.Data[azure.DNSClientIDKey] = []byte(clientIDValue)
			secret.Data[azure.DNSClientSecretKey] = []byte(clientSecretNewline)

			errs := ValidateDNSProviderSecret(secret, fldPath)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Type).To(Equal(field.ErrorTypeInvalid))
			expected := fmt.Sprintf("field clientSecret must not contain leading or trailing whitespace in secret %s/%s", namespace, secretName)
			Expect(errs[0].Field).To(Equal("dnsRecord.spec.secretRef.data[AZURE_CLIENT_SECRET]"))
			Expect(errs[0].Detail).To(Equal(expected))
			Expect(errs[0].BadValue).To(Equal("(hidden)"))
		})

		It("should pass with each allowed AZURE_CLOUD value", func() {
			secret.Data[azure.DNSSubscriptionIDKey] = []byte(subscriptionIDValue)
			secret.Data[azure.DNSTenantIDKey] = []byte(tenantIDValue)
			secret.Data[azure.DNSClientIDKey] = []byte(clientIDValue)
			secret.Data[azure.DNSClientSecretKey] = []byte(clientSecretValue)

			for _, v := range []string{"AzurePublic", "AzureChina", "AzureGovernment"} {
				secret.Data[azure.DNSAzureCloud] = []byte(v)
				errs := ValidateDNSProviderSecret(secret, fldPath)
				Expect(errs).To(BeEmpty())
			}
		})

		It("should fail with invalid AZURE_CLOUD value", func() {
			secret.Data[azure.DNSSubscriptionIDKey] = []byte(subscriptionIDValue)
			secret.Data[azure.DNSTenantIDKey] = []byte(tenantIDValue)
			secret.Data[azure.DNSClientIDKey] = []byte(clientIDValue)
			secret.Data[azure.DNSClientSecretKey] = []byte(clientSecretValue)
			secret.Data[azure.DNSAzureCloud] = []byte(invalidAzureCloudValue)

			errs := ValidateDNSProviderSecret(secret, fldPath)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Type).To(Equal(field.ErrorTypeNotSupported))
			Expect(errs[0].Field).To(Equal("dnsRecord.spec.secretRef.data[AZURE_CLOUD]"))
			Expect(errs[0].BadValue).To(Equal(invalidAzureCloudValue))
		})

		It("should fail when unexpected field present", func() {
			secret.Data[azure.DNSSubscriptionIDKey] = []byte(subscriptionIDValue)
			secret.Data[azure.DNSTenantIDKey] = []byte(tenantIDValue)
			secret.Data[azure.DNSClientIDKey] = []byte(clientIDValue)
			secret.Data[azure.DNSClientSecretKey] = []byte(clientSecretValue)
			secret.Data["UNEXPECTED_FIELD"] = []byte("value")

			errs := ValidateDNSProviderSecret(secret, fldPath)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Type).To(Equal(field.ErrorTypeForbidden))
			expected := fmt.Sprintf("unexpected field %q in secret %s/%s", "UNEXPECTED_FIELD", namespace, secretName)
			Expect(errs[0].Field).To(Equal("dnsRecord.spec.secretRef.data[UNEXPECTED_FIELD]"))
			Expect(errs[0].Detail).To(Equal(expected))
		})
	})
})
