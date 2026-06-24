// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/security"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	testutils "github.com/gardener/gardener/pkg/utils/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

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

			scheme *runtime.Scheme

			ctx                                = context.TODO()
			credentialsBindingSecret           *security.CredentialsBinding
			credentialsBindingWorkloadIdentity *security.CredentialsBinding
		)

		newValidator := func(objs ...client.Object) {
			apiReader := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
			mgr := testutils.FakeManager{APIReader: apiReader}
			credentialsBindingValidator = validator.NewCredentialsBindingValidator(mgr)
		}

		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(gardencorev1beta1.AddToScheme(scheme)).To(Succeed())
			Expect(securityv1alpha1.AddToScheme(scheme)).To(Succeed())
			Expect(azureapi.AddToScheme(scheme)).To(Succeed())
			Expect(azureapiv1alpha1.AddToScheme(scheme)).To(Succeed())
			Expect(corev1.AddToScheme(scheme)).To(Succeed())

			newValidator()

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
			Expect(err).To(MatchError(ContainSubstring("unsupported credentials reference")))
		})

		It("should return err if it fails to get the corresponding Secret", func() {
			// Secret not pre-populated → fake client returns not-found
			err := credentialsBindingValidator.Validate(ctx, credentialsBindingSecret, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should return err when the corresponding Secret is not valid", func() {
			newValidator(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Data:       map[string][]byte{"foo": []byte("bar")},
			})

			err := credentialsBindingValidator.Validate(ctx, credentialsBindingSecret, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should succeed when the corresponding Secret is valid", func() {
			newValidator(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Data: map[string][]byte{
					azure.SubscriptionIDKey: []byte("b7ad693a-028a-422c-b064-d76c4586f2b3"),
					azure.TenantIDKey:       []byte("ee16e592-3035-41b9-a217-958f8f75b740"),
					azure.ClientIDKey:       []byte("7fc4685d-3c33-40e6-b6bf-7857cab04300"),
					azure.ClientSecretKey:   []byte("clientSecret"),
				},
			})

			Expect(credentialsBindingValidator.Validate(ctx, credentialsBindingSecret, nil)).To(Succeed())
		})

		It("should return nil when the CredentialsBinding did not change", func() {
			old := credentialsBindingSecret.DeepCopy()

			Expect(credentialsBindingValidator.Validate(ctx, credentialsBindingSecret, old)).To(Succeed())
		})

		Context("InternalSecret", func() {
			var credentialsBindingInternalSecret *security.CredentialsBinding

			BeforeEach(func() {
				credentialsBindingInternalSecret = &security.CredentialsBinding{
					CredentialsRef: corev1.ObjectReference{
						Name:       name,
						Namespace:  namespace,
						Kind:       "InternalSecret",
						APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
					},
				}
			})

			It("should return err if it fails to get the corresponding InternalSecret", func() {
				// InternalSecret not pre-populated → fake client returns not-found
				err := credentialsBindingValidator.Validate(ctx, credentialsBindingInternalSecret, nil)
				Expect(err).To(HaveOccurred())
			})

			It("should return err when the corresponding InternalSecret is not valid", func() {
				newValidator(&gardencorev1beta1.InternalSecret{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
					Data:       map[string][]byte{"foo": []byte("bar")},
				})

				err := credentialsBindingValidator.Validate(ctx, credentialsBindingInternalSecret, nil)
				Expect(err).To(HaveOccurred())
			})

			It("should succeed when the corresponding InternalSecret is valid", func() {
				newValidator(&gardencorev1beta1.InternalSecret{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
					Data: map[string][]byte{
						azure.SubscriptionIDKey: []byte("b7ad693a-028a-422c-b064-d76c4586f2b3"),
						azure.TenantIDKey:       []byte("ee16e592-3035-41b9-a217-958f8f75b740"),
					},
				})

				Expect(credentialsBindingValidator.Validate(ctx, credentialsBindingInternalSecret, nil)).To(Succeed())
			})
		})

		It("should succeed when the corresponding WorkloadIdentity is valid", func() {
			newValidator(&securityv1alpha1.WorkloadIdentity{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Spec: securityv1alpha1.WorkloadIdentitySpec{
					Audiences: []string{"foo"},
					TargetSystem: securityv1alpha1.TargetSystem{
						Type: "azure",
						ProviderConfig: &runtime.RawExtension{
							Raw: []byte(`{
								"apiVersion": "azure.provider.extensions.gardener.cloud/v1alpha1",
								"kind": "WorkloadIdentityConfig",
								"clientID": "11111c4e-db61-17fa-a141-ed39b34aa561",
								"tenantID": "44444c4e-db61-17fa-a141-ed39b34aa561",
								"subscriptionID": "44444c4e-db61-17fa-a141-ed39b34aa561"
							}`),
						},
					},
				},
			})

			Expect(credentialsBindingValidator.Validate(ctx, credentialsBindingWorkloadIdentity, nil)).To(Succeed())
		})

		It("should return err if it fails to get the corresponding WorkloadIdentity", func() {
			// WorkloadIdentity not pre-populated → fake client returns not-found
			err := credentialsBindingValidator.Validate(ctx, credentialsBindingWorkloadIdentity, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should return err when the corresponding WorkloadIdentity is missing config for target system", func() {
			newValidator(&securityv1alpha1.WorkloadIdentity{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Spec: securityv1alpha1.WorkloadIdentitySpec{
					Audiences: []string{"foo"},
					TargetSystem: securityv1alpha1.TargetSystem{
						Type: "azure",
					},
				},
			})

			err := credentialsBindingValidator.Validate(ctx, credentialsBindingWorkloadIdentity, nil)
			Expect(err).To(MatchError("the target system is missing configuration"))
		})

		It("should return err when the corresponding WorkloadIdentity has empty config for target system", func() {
			newValidator(&securityv1alpha1.WorkloadIdentity{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Spec: securityv1alpha1.WorkloadIdentitySpec{
					Audiences: []string{"foo"},
					TargetSystem: securityv1alpha1.TargetSystem{
						Type:           "azure",
						ProviderConfig: &runtime.RawExtension{Raw: []byte(`{}`)},
					},
				},
			})

			err := credentialsBindingValidator.Validate(ctx, credentialsBindingWorkloadIdentity, nil)
			Expect(err.Error()).To(ContainSubstring("target system's configuration is not valid"))
		})

		It("should return err when the corresponding WorkloadIdentity has invalid target system configuration", func() {
			newValidator(&securityv1alpha1.WorkloadIdentity{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Spec: securityv1alpha1.WorkloadIdentitySpec{
					Audiences: []string{"foo"},
					TargetSystem: securityv1alpha1.TargetSystem{
						Type: "azure",
						ProviderConfig: &runtime.RawExtension{
							Raw: []byte(`{
								"apiVersion": "azure.provider.extensions.gardener.cloud/v1alpha1",
								"kind": "WorkloadIdentityConfig"
							}`),
						},
					},
				},
			})

			err := credentialsBindingValidator.Validate(ctx, credentialsBindingWorkloadIdentity, nil)
			Expect(err.Error()).To(ContainSubstring("referenced workload identity garden-dev/my-provider-account is not valid"))
		})
	})
})
