// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"
	"fmt"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener-extension-provider-azure/pkg/admission/validator"
	azurevalidation "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

var _ = Describe("Secret validator", func() {
	Describe("#Validate", func() {
		const (
			namespace = "garden-dev"
			name      = "my-secret"
		)

		var (
			secretValidator extensionswebhook.Validator
			ctx             = context.TODO()

			secret    *corev1.Secret
			oldSecret *corev1.Secret

			subscriptionIDValue     = "a6ad693a-028a-422c-b064-d76a4586f2b3"
			tenantIDValue           = "ee16e593-3035-41b9-a217-958f8f75b750"
			changedSubscriptionGUID = "11111111-2222-3333-4444-555555555555"
		)

		BeforeEach(func() {
			secretValidator = validator.NewSecretValidator()
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Data: make(map[string][]byte),
			}
			oldSecret = nil
		})

		It("should error if newObj is not a Secret", func() {
			err := secretValidator.Validate(ctx, &corev1.ConfigMap{}, nil)
			Expect(err).To(MatchError("wrong object type *v1.ConfigMap"))
		})

		It("should error if oldObj is not a Secret", func() {
			err := secretValidator.Validate(ctx, &corev1.Secret{}, &corev1.ConfigMap{})
			Expect(err).To(MatchError("wrong object type *v1.ConfigMap for old object"))
		})

		It("should skip validation when data unchanged (early exit)", func() {
			secret.Data["FOO"] = []byte("bar") // invalid but unchanged
			oldSecret = secret.DeepCopy()
			Expect(secretValidator.Validate(ctx, secret, oldSecret)).To(Succeed())
		})

		It("should fail with aggregated errors when required fields missing", func() {
			err := secretValidator.Validate(ctx, secret, nil)
			Expect(err).To(HaveOccurred())
			msg := err.Error()
			Expect(msg).To(ContainSubstring("missing required field subscriptionID"))
			Expect(msg).To(ContainSubstring("missing required field tenantID"))
			Expect(msg[0]).To(Equal(byte('[')))
		})

		It("should fail when immutable subscriptionID changes", func() {
			oldSecret = secret.DeepCopy()
			oldSecret.Data[azure.SubscriptionIDKey] = []byte(subscriptionIDValue)
			oldSecret.Data[azure.TenantIDKey] = []byte(tenantIDValue)
			secret.Data[azure.SubscriptionIDKey] = []byte(changedSubscriptionGUID)
			secret.Data[azure.TenantIDKey] = []byte(tenantIDValue)

			err := secretValidator.Validate(ctx, secret, oldSecret)
			Expect(err).To(HaveOccurred())
			expected := fmt.Sprintf(
				"secret.data[subscriptionID]: Invalid value: \"(hidden)\": field subscriptionID must not be changed for existing shoot clusters in secret %s/%s",
				namespace, name,
			)
			Expect(err.Error()).To(Equal(expected))
		})

		It("should succeed with valid minimal secret (user-assigned identity)", func() {
			secret.Data[azure.SubscriptionIDKey] = []byte(subscriptionIDValue)
			secret.Data[azure.TenantIDKey] = []byte(tenantIDValue)
			Expect(secretValidator.Validate(ctx, secret, nil)).To(Succeed())
		})

		It("should yield the same error when called via the validator or directly", func() {
			// Missing subscription only -> single error
			secret.Data[azure.TenantIDKey] = []byte(tenantIDValue)
			directErrs := azurevalidation.ValidateCloudProviderSecret(secret, nil, field.NewPath("secret"))

			err := secretValidator.Validate(ctx, secret, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(directErrs.ToAggregate().Error()))
		})
	})
})
