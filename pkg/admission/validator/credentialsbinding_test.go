// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"
	"errors"
	"fmt"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/security"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/admission/validator"
	azureapi "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	azureapiv1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

var _ = Describe("CredentialsBinding validator", func() {
	Describe("#Validate", func() {
		const (
			namespace = "garden-dev"
			name      = "my-provider-account"
		)

		var (
			credentialsBindingValidator extensionswebhook.Validator

			ctrl      *gomock.Controller
			mgr       *mockmanager.MockManager
			apiReader *mockclient.MockReader

			ctx                                = context.TODO()
			credentialsBindingSecret           *security.CredentialsBinding
			credentialsBindingWorkloadIdentity *security.CredentialsBinding

			fakeErr = fmt.Errorf("fake err")
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())

			mgr = mockmanager.NewMockManager(ctrl)

			apiReader = mockclient.NewMockReader(ctrl)
			mgr.EXPECT().GetAPIReader().Return(apiReader)
			scheme := runtime.NewScheme()
			Expect(gardencorev1beta1.AddToScheme(scheme)).To(Succeed())
			Expect(securityv1alpha1.AddToScheme(scheme)).To(Succeed())
			Expect(azureapi.AddToScheme(scheme)).To(Succeed())
			Expect(azureapiv1alpha1.AddToScheme(scheme)).To(Succeed())

			mgr.EXPECT().GetScheme().Return(scheme)

			credentialsBindingValidator = validator.NewCredentialsBindingValidator(mgr)

			credentialsBindingSecret = &security.CredentialsBinding{
				CredentialsRef: corev1.ObjectReference{
					Name:       name,
					Namespace:  namespace,
					Kind:       "Secret",
					APIVersion: "v1",
				},
			}
			credentialsBindingWorkloadIdentity = &security.CredentialsBinding{
				CredentialsRef: corev1.ObjectReference{
					Name:       name,
					Namespace:  namespace,
					Kind:       "WorkloadIdentity",
					APIVersion: "security.gardener.cloud/v1alpha1",
				},
			}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should return err when obj is not a CredentialsBinding", func() {
			err := credentialsBindingValidator.Validate(ctx, &corev1.Secret{}, nil)
			Expect(err).To(MatchError("wrong object type *v1.Secret"))
		})

		It("should return err when oldObj is not a CredentialsBinding", func() {
			err := credentialsBindingValidator.Validate(ctx, &security.CredentialsBinding{}, &corev1.Secret{})
			Expect(err).To(MatchError("wrong object type *v1.Secret for old object"))
		})

		It("should return err if the CredentialsBinding references unknown credentials type", func() {
			credentialsBindingSecret.CredentialsRef.APIVersion = "unknown"
			err := credentialsBindingValidator.Validate(ctx, credentialsBindingSecret, nil)
			Expect(err).To(MatchError(errors.New(`unsupported credentials reference: version "unknown", kind "Secret"`)))
		})

		It("should return err if it fails to get the corresponding Secret", func() {
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr)

			err := credentialsBindingValidator.Validate(ctx, credentialsBindingSecret, nil)
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

			err := credentialsBindingValidator.Validate(ctx, credentialsBindingSecret, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should succeed when the corresponding Secret is valid", func() {
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

			Expect(credentialsBindingValidator.Validate(ctx, credentialsBindingSecret, nil)).To(Succeed())
		})

		It("should return nil when the CredentialsBinding did not change", func() {
			old := credentialsBindingSecret.DeepCopy()

			Expect(credentialsBindingValidator.Validate(ctx, credentialsBindingSecret, old)).To(Succeed())
		})

		It("should succeed when the corresponding WorkloadIdentity is valid", func() {
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&securityv1alpha1.WorkloadIdentity{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *securityv1alpha1.WorkloadIdentity, _ ...client.GetOption) error {
					workloadIdentity := &securityv1alpha1.WorkloadIdentity{
						Spec: securityv1alpha1.WorkloadIdentitySpec{
							Audiences: []string{"foo"},
							TargetSystem: securityv1alpha1.TargetSystem{
								Type: "azure",
								ProviderConfig: &runtime.RawExtension{
									Raw: []byte(`
apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
kind: WorkloadIdentityConfig
clientID: "11111c4e-db61-17fa-a141-ed39b34aa561"
tenantID: "44444c4e-db61-17fa-a141-ed39b34aa561"
subscriptionID: "44444c4e-db61-17fa-a141-ed39b34aa561"
`),
								},
							},
						},
					}
					*obj = *workloadIdentity
					return nil
				})

			Expect(credentialsBindingValidator.Validate(ctx, credentialsBindingWorkloadIdentity, nil)).To(Succeed())
		})

		It("should return err if it fails to get the corresponding WorkloadIdentity", func() {
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&securityv1alpha1.WorkloadIdentity{})).Return(fakeErr)

			err := credentialsBindingValidator.Validate(ctx, credentialsBindingWorkloadIdentity, nil)
			Expect(err).To(MatchError(fakeErr))
		})

		It("should return err when the corresponding WorkloadIdentity is missing config for target system", func() {
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&securityv1alpha1.WorkloadIdentity{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *securityv1alpha1.WorkloadIdentity, _ ...client.GetOption) error {
					workloadIdentity := &securityv1alpha1.WorkloadIdentity{
						Spec: securityv1alpha1.WorkloadIdentitySpec{
							Audiences: []string{"foo"},
							TargetSystem: securityv1alpha1.TargetSystem{
								Type: "azure",
							},
						},
					}
					*obj = *workloadIdentity
					return nil
				})

			err := credentialsBindingValidator.Validate(ctx, credentialsBindingWorkloadIdentity, nil)
			Expect(err).To(MatchError("the target system is missing configuration"))
		})

		It("should return err when the corresponding WorkloadIdentity has empty config for target system", func() {
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&securityv1alpha1.WorkloadIdentity{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *securityv1alpha1.WorkloadIdentity, _ ...client.GetOption) error {
					workloadIdentity := &securityv1alpha1.WorkloadIdentity{
						Spec: securityv1alpha1.WorkloadIdentitySpec{
							Audiences: []string{"foo"},
							TargetSystem: securityv1alpha1.TargetSystem{
								Type:           "azure",
								ProviderConfig: &runtime.RawExtension{},
							},
						},
					}
					*obj = *workloadIdentity
					return nil
				})

			err := credentialsBindingValidator.Validate(ctx, credentialsBindingWorkloadIdentity, nil)
			Expect(err.Error()).To(ContainSubstring("target system's configuration is not valid"))
		})

		It("should return err when the corresponding WorkloadIdentity has invalid target system configuration", func() {
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&securityv1alpha1.WorkloadIdentity{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *securityv1alpha1.WorkloadIdentity, _ ...client.GetOption) error {
					workloadIdentity := &securityv1alpha1.WorkloadIdentity{
						Spec: securityv1alpha1.WorkloadIdentitySpec{
							Audiences: []string{"foo"},
							TargetSystem: securityv1alpha1.TargetSystem{
								Type: "azure",
								ProviderConfig: &runtime.RawExtension{
									Raw: []byte(`
apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
kind: WorkloadIdentityConfig
`),
								},
							},
						},
					}
					*obj = *workloadIdentity
					return nil
				})

			err := credentialsBindingValidator.Validate(ctx, credentialsBindingWorkloadIdentity, nil)
			Expect(err.Error()).To(ContainSubstring("referenced workload identity garden-dev/my-provider-account is not valid"))
		})
	})
})
