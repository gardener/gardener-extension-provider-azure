// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"
	"fmt"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/admission/validator"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

var _ = Describe("SecretBinding validator", func() {

	Describe("#Validate", func() {
		const (
			namespace = "garden-dev"
			name      = "my-provider-account"
		)

		var (
			secretBindingValidator extensionswebhook.Validator

			ctrl      *gomock.Controller
			apiReader *mockclient.MockReader
			mgr       *mockmanager.MockManager

			ctx           = context.TODO()
			secretBinding = &core.SecretBinding{
				SecretRef: corev1.SecretReference{
					Name:      name,
					Namespace: namespace,
				},
			}
			fakeErr = fmt.Errorf("fake err")
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			apiReader = mockclient.NewMockReader(ctrl)

			mgr = mockmanager.NewMockManager(ctrl)
			mgr.EXPECT().GetAPIReader().Return(apiReader)

			secretBindingValidator = validator.NewSecretBindingValidator(mgr)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should return err when obj is not a SecretBinding", func() {
			err := secretBindingValidator.Validate(ctx, &corev1.Secret{}, nil)
			Expect(err).To(MatchError("wrong object type *v1.Secret"))
		})

		It("should return err when oldObj is not a SecretBinding", func() {
			err := secretBindingValidator.Validate(ctx, &core.SecretBinding{}, &corev1.Secret{})
			Expect(err).To(MatchError("wrong object type *v1.Secret for old object"))
		})

		It("should return err if it fails to get the corresponding Secret", func() {
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr)

			err := secretBindingValidator.Validate(ctx, secretBinding, nil)
			Expect(err).To(MatchError(fakeErr))
		})

		It("should return err when the corresponding Secret is not valid", func() {
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
					secret := &corev1.Secret{Data: map[string][]byte{
						"foo": []byte("bar"),
					}}
					*obj = *secret
					return nil
				})

			err := secretBindingValidator.Validate(ctx, secretBinding, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should return nil when the corresponding Secret is valid", func() {
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
					secret := &corev1.Secret{Data: map[string][]byte{
						azure.SubscriptionIDKey: []byte("b7ad693a-028a-422c-b064-d76c4586f2b3"),
						azure.TenantIDKey:       []byte("ee16e592-3035-41b9-a217-958f8f75b740"),
						azure.ClientIDKey:       []byte("7fc4685d-3c33-40e6-b6bf-7857cab04300"),
						azure.ClientSecretKey:   []byte("clientSecret"),
					}}
					*obj = *secret
					return nil
				})

			err := secretBindingValidator.Validate(ctx, secretBinding, nil)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
