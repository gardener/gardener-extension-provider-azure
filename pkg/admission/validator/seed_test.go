// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener-extension-provider-azure/pkg/admission/validator"
)

var _ = Describe("Seed Validator", func() {
	Describe("#Validate", func() {
		var (
			ctx            context.Context
			credentialsRef *corev1.ObjectReference
			seedValidator  extensionswebhook.Validator
		)

		BeforeEach(func() {
			ctx = context.TODO()
			credentialsRef = &corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       "backup-credentials",
				Namespace:  "garden",
			}
			seedValidator = validator.NewSeedValidator()
		})

		It("should return err when obj is not a gardencore.Seed", func() {
			Expect(seedValidator.Validate(ctx, &corev1.Secret{}, nil)).To(MatchError("wrong object type *v1.Secret for new object"))
		})

		It("should return err when oldObj is not a gardencore.Seed", func() {
			Expect(seedValidator.Validate(ctx, &gardencore.Seed{}, &corev1.Secret{})).To(MatchError("wrong object type *v1.Secret for old object"))
		})

		Context("Create", func() {
			It("should succeed to create seed when backup is unset", func() {
				seed := &gardencore.Seed{
					Spec: gardencore.SeedSpec{
						Backup: nil,
					},
				}

				Expect(seedValidator.Validate(ctx, seed, nil)).To(Succeed())
			})

			It("should fail to create seed when backup has nil credentialsRef", func() {
				seed := &gardencore.Seed{
					Spec: gardencore.SeedSpec{
						Backup: &gardencore.Backup{
							CredentialsRef: nil,
						},
					},
				}

				err := seedValidator.Validate(ctx, seed, nil)
				Expect(err).To(HaveOccurred())
			})

			It("should succeed to create seed when backup has providerConfig unset", func() {
				seed := &gardencore.Seed{
					Spec: gardencore.SeedSpec{
						Backup: &gardencore.Backup{
							CredentialsRef: credentialsRef,
						},
					},
				}

				Expect(seedValidator.Validate(ctx, seed, nil)).To(Succeed())
			})

			It("should fail to create seed when backup has invalid providerConfig", func() {
				seed := &gardencore.Seed{
					Spec: gardencore.SeedSpec{
						Backup: &gardencore.Backup{
							CredentialsRef: nil,
							ProviderConfig: &runtime.RawExtension{
								Raw: []byte(`{"apiVersion": "azure.provider.extensions.gardener.cloud/v1alpha1", "kind": "invalid"}`),
							},
						},
					},
				}

				err := seedValidator.Validate(ctx, seed, nil)
				Expect(err).To(HaveOccurred())
			})

			It("should succeed to create seed when backup has valid providerConfig", func() {
				seed := &gardencore.Seed{
					Spec: gardencore.SeedSpec{
						Backup: &gardencore.Backup{
							CredentialsRef: credentialsRef,
							ProviderConfig: &runtime.RawExtension{
								Raw: []byte(`{"apiVersion": "azure.provider.extensions.gardener.cloud/v1alpha1", "kind": "BackupBucketConfig"}`),
							},
						},
					},
				}

				Expect(seedValidator.Validate(ctx, seed, nil)).To(Succeed())
			})
		})

		Context("Update", func() {
			It("should succeed when seed had empty backup config but is now updated with valid providerConfig", func() {
				seed := &gardencore.Seed{
					Spec: gardencore.SeedSpec{
						Backup: nil,
					},
				}

				newSeed := seed.DeepCopy()
				newSeed.Spec.Backup = &gardencore.Backup{
					CredentialsRef: credentialsRef,
					ProviderConfig: &runtime.RawExtension{
						Raw: []byte(`{"apiVersion": "azure.provider.extensions.gardener.cloud/v1alpha1", "kind": "BackupBucketConfig"}`),
					},
				}

				Expect(seedValidator.Validate(ctx, newSeed, seed)).To(Succeed())
			})

			It("should fail when seed had empty backup config but is now updated with invalid providerConfig", func() {
				seed := &gardencore.Seed{
					Spec: gardencore.SeedSpec{
						Backup: nil,
					},
				}

				newSeed := seed.DeepCopy()
				newSeed.Spec.Backup = &gardencore.Backup{
					CredentialsRef: credentialsRef,
					ProviderConfig: &runtime.RawExtension{
						Raw: []byte(`{"apiVersion": "azure.provider.extensions.gardener.cloud/v1alpha1", "kind": "invalid"`),
					},
				}

				Expect(seedValidator.Validate(ctx, newSeed, seed)).To(HaveOccurred())
			})

			It("should fail when seed had set invalid backup config and is now updated with valid providerConfig", func() {
				seed := &gardencore.Seed{
					Spec: gardencore.SeedSpec{
						Backup: &gardencore.Backup{
							CredentialsRef: credentialsRef,
							ProviderConfig: &runtime.RawExtension{
								Raw: []byte(`{"apiVersion": "azure.provider.extensions.gardener.cloud/v1alpha1", "kind": "invalid"}`),
							},
						},
					},
				}

				newSeed := seed.DeepCopy()
				newSeed.Spec.Backup.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion": "azure.provider.extensions.gardener.cloud/v1alpha1", "kind": "BackupBucketConfig"}`),
				}

				err := seedValidator.Validate(ctx, newSeed, seed)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("failed to decode old provider config: ")))
			})

			It("should fail when seed had set valid backup config and is now updated with invalid providerConfig", func() {
				seed := &gardencore.Seed{
					Spec: gardencore.SeedSpec{
						Backup: &gardencore.Backup{
							CredentialsRef: credentialsRef,
							ProviderConfig: &runtime.RawExtension{
								Raw: []byte(`{"apiVersion": "azure.provider.extensions.gardener.cloud/v1alpha1", "kind": "BackupBucketConfig"}`),
							},
						},
					},
				}

				newSeed := seed.DeepCopy()
				newSeed.Spec.Backup.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion": "azure.provider.extensions.gardener.cloud/v1alpha1", "kind": "invalid"}`),
				}

				err := seedValidator.Validate(ctx, newSeed, seed)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("failed to decode new provider config: ")))
			})

			It("should succeed when seed had set backup config and is now updated with valid providerConfig", func() {
				seed := &gardencore.Seed{
					Spec: gardencore.SeedSpec{
						Backup: &gardencore.Backup{
							CredentialsRef: credentialsRef,
							ProviderConfig: &runtime.RawExtension{
								Raw: []byte(`{"apiVersion": "azure.provider.extensions.gardener.cloud/v1alpha1", "kind": "BackupBucketConfig"}`),
							},
						},
					},
				}

				newSeed := seed.DeepCopy()
				newSeed.Spec.Backup.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion": "azure.provider.extensions.gardener.cloud/v1alpha1", "kind": "BackupBucketConfig", "immutability": {"retentionPeriod": "96h", "retentionType": "bucket"}}`),
				}

				Expect(seedValidator.Validate(ctx, newSeed, seed)).To(Succeed())
			})
		})
	})
})
