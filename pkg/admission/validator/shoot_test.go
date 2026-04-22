// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"
	"encoding/json"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"go.uber.org/mock/gomock"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/admission/validator"
	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	apisazurev1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

var _ = Describe("Shoot validator", func() {
	Describe("#Validate", func() {
		const namespace = "garden-dev"

		var (
			shootValidator extensionswebhook.Validator

			ctrl                   *gomock.Controller
			mgr                    *mockmanager.MockManager
			c                      *mockclient.MockClient
			reader                 *mockclient.MockReader
			cloudProfile           *gardencorev1beta1.CloudProfile
			namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile
			shoot                  *core.Shoot

			ctx                       = context.Background()
			cloudProfileKey           = client.ObjectKey{Name: "azure"}
			namespacedCloudProfileKey = client.ObjectKey{Name: "azure-nscpfl", Namespace: namespace}

			regionName   = "westus"
			imageName    = "Foo"
			imageVersion = "1.0.0"
			architecture = ptr.To("analog")
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())

			scheme := runtime.NewScheme()
			Expect(apisazure.AddToScheme(scheme)).To(Succeed())
			Expect(apisazurev1alpha1.AddToScheme(scheme)).To(Succeed())
			Expect(gardencorev1beta1.AddToScheme(scheme)).To(Succeed())

			c = mockclient.NewMockClient(ctrl)
			reader = mockclient.NewMockReader(ctrl)
			mgr = mockmanager.NewMockManager(ctrl)

			mgr.EXPECT().GetScheme().Return(scheme).Times(2)
			mgr.EXPECT().GetClient().Return(c)
			mgr.EXPECT().GetAPIReader().Return(reader)

			shootValidator = validator.NewShootValidator(mgr)

			cloudProfile = &gardencorev1beta1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "azure",
				},
				Spec: gardencorev1beta1.CloudProfileSpec{
					Regions: []gardencorev1beta1.Region{
						{
							Name: regionName,
							Zones: []gardencorev1beta1.AvailabilityZone{
								{
									Name: "1",
								},
								{
									Name: "2",
								},
							},
						},
					},
					ProviderConfig: &runtime.RawExtension{
						Raw: encode(&apisazurev1alpha1.CloudProfileConfig{
							TypeMeta: metav1.TypeMeta{
								APIVersion: apisazurev1alpha1.SchemeGroupVersion.String(),
								Kind:       "CloudProfileConfig",
							},
							MachineImages: []apisazurev1alpha1.MachineImages{
								{
									Name: imageName,
									Versions: []apisazurev1alpha1.MachineImageVersion{
										{
											Version: imageVersion,
										},
									},
								},
							},
						}),
					},
				},
			}

			namespacedCloudProfile = &gardencorev1beta1.NamespacedCloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "azure-nscpfl",
				},
				Spec: gardencorev1beta1.NamespacedCloudProfileSpec{
					Parent: gardencorev1beta1.CloudProfileReference{
						Kind: "CloudProfile",
						Name: "azure",
					},
				},
				Status: gardencorev1beta1.NamespacedCloudProfileStatus{
					CloudProfileSpec: cloudProfile.Spec,
				},
			}

			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: namespace,
				},
				Spec: core.ShootSpec{
					CloudProfile: &core.CloudProfileReference{
						Kind: "CloudProfile",
						Name: "azure",
					},
					SeedName: ptr.To("azure"),
					Provider: core.Provider{
						Type: azure.Type,
						InfrastructureConfig: &runtime.RawExtension{
							Raw: encode(&apisazurev1alpha1.InfrastructureConfig{
								TypeMeta: metav1.TypeMeta{
									APIVersion: apisazurev1alpha1.SchemeGroupVersion.String(),
									Kind:       "InfrastructureConfig",
								},
								Networks: apisazurev1alpha1.NetworkConfig{
									Workers: ptr.To("10.250.0.0/16"),
								},
								Zoned: true,
							}),
						},
						Workers: []core.Worker{
							{
								Name: "worker-1",
								Volume: &core.Volume{
									VolumeSize: "50Gi",
									Type:       ptr.To("Volume"),
								},
								Zones: []string{"1"},
								Machine: core.Machine{
									Image: &core.ShootMachineImage{
										Name:    imageName,
										Version: imageVersion,
									},
									Architecture: architecture,
								},
							},
						},
					},
					Region: regionName,
					Networking: &core.Networking{
						Nodes: ptr.To("10.250.0.0/16"),
						Type:  ptr.To("cilium"),
					},
				},
			}
		})

		Context("Shoot creation (old is nil)", func() {
			It("should return err when new is not a Shoot", func() {
				err := shootValidator.Validate(ctx, &corev1.Pod{}, nil)
				Expect(err).To(MatchError("wrong object type *v1.Pod"))
			})

			It("should return err when networking is configured to use dual-stack", func() {
				c.EXPECT().Get(ctx, cloudProfileKey, &gardencorev1beta1.CloudProfile{}).SetArg(2, *cloudProfile)
				shoot.Spec.Networking.IPFamilies = []core.IPFamily{core.IPFamilyIPv4, core.IPFamilyIPv6}

				err := shootValidator.Validate(ctx, shoot, nil)
				Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.networking.ipFamilies"),
				}))))
			})

			It("should return err when networking is configured to use IPv6-only", func() {
				c.EXPECT().Get(ctx, cloudProfileKey, &gardencorev1beta1.CloudProfile{}).SetArg(2, *cloudProfile)
				shoot.Spec.Networking.IPFamilies = []core.IPFamily{core.IPFamilyIPv6}

				err := shootValidator.Validate(ctx, shoot, nil)
				Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.networking.ipFamilies"),
				}))))
			})

			It("should return err when infrastructureConfig is nil", func() {
				c.EXPECT().Get(ctx, cloudProfileKey, &gardencorev1beta1.CloudProfile{}).SetArg(2, *cloudProfile)
				shoot.Spec.Provider.InfrastructureConfig = nil

				err := shootValidator.Validate(ctx, shoot, nil)
				Expect(err).To(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.provider.infrastructureConfig"),
				})))
			})

			It("should return err when infrastructureConfig fails to be decoded", func() {
				c.EXPECT().Get(ctx, cloudProfileKey, &gardencorev1beta1.CloudProfile{}).SetArg(2, *cloudProfile)
				shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: []byte("foo")}

				err := shootValidator.Validate(ctx, shoot, nil)
				Expect(err).To(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.provider.infrastructureConfig"),
				})))
			})

			It("should return err when worker is invalid", func() {
				c.EXPECT().Get(ctx, cloudProfileKey, &gardencorev1beta1.CloudProfile{}).SetArg(2, *cloudProfile)
				shoot.Spec.Provider.Workers = []core.Worker{
					{
						Name:   "worker-1",
						Volume: nil,
						Zones:  nil,
					},
				}

				err := shootValidator.Validate(ctx, shoot, nil)
				Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.provider.workers[0].volume"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.provider.workers[0].zones"),
				}))))
			})

			It("should succeed for valid Shoot", func() {
				c.EXPECT().Get(ctx, cloudProfileKey, &gardencorev1beta1.CloudProfile{}).SetArg(2, *cloudProfile)

				err := shootValidator.Validate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should also work for cloudProfileName instead of CloudProfile reference in Shoot", func() {
				shoot.Spec.CloudProfileName = ptr.To("azure")
				shoot.Spec.CloudProfile = nil
				c.EXPECT().Get(ctx, cloudProfileKey, &gardencorev1beta1.CloudProfile{}).SetArg(2, *cloudProfile)

				err := shootValidator.Validate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should also work for NamespacedCloudProfile referenced from Shoot", func() {
				shoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: "azure-nscpfl",
				}
				c.EXPECT().Get(ctx, namespacedCloudProfileKey, &gardencorev1beta1.NamespacedCloudProfile{}).SetArg(2, *namespacedCloudProfile)

				err := shootValidator.Validate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("Shoot with custom DNS provider", func() {
				BeforeEach(func() {
					c.EXPECT().Get(ctx, cloudProfileKey, &gardencorev1beta1.CloudProfile{}).SetArg(2, *cloudProfile)
				})

				Context("#secretName", func() {
					It("should return error when azure-dns provider has no secretName", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{Type: ptr.To(azure.DNSType), Primary: ptr.To(true)}, // secretName missing
							},
						}

						Expect(shootValidator.Validate(ctx, shoot, nil)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("spec.dns.providers[0].secretName"),
						}))))
					})

					It("should return error when azure-dns provider secret not found", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{Type: ptr.To(azure.DNSType), SecretName: ptr.To("dns-secret"), Primary: ptr.To(true)},
							},
						}
						reader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "dns-secret"},
							&corev1.Secret{}).
							Return(apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, "dns-secret"))

						Expect(shootValidator.Validate(ctx, shoot, nil)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.dns.providers[0].secretName"),
						}))))
					})

					It("should return error when azure-dns secret is invalid (missing subscriptionID)", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{Type: ptr.To(azure.DNSType), SecretName: ptr.To("dns-secret"), Primary: ptr.To(true)},
							},
						}
						invalidSecret := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{Name: "dns-secret", Namespace: namespace},
							Data: map[string][]byte{
								azure.DNSTenantIDKey:     []byte("ee16e593-3035-41b9-a217-958f8f75b750"),
								azure.DNSClientIDKey:     []byte("7fc4685e-3c33-40e6-b6bf-7857cab04390"),
								azure.DNSClientSecretKey: []byte("secret"),
							},
						}
						reader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "dns-secret"},
							&corev1.Secret{}).
							SetArg(2, *invalidSecret).
							Return(nil)

						Expect(shootValidator.Validate(ctx, shoot, nil)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("spec.dns.providers[0].secretName"),
							"Detail": ContainSubstring("missing required field \"AZURE_SUBSCRIPTION_ID\" in secret"),
						}))))
					})

					It("should succeed with valid azure-dns provider secret", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{Type: ptr.To(azure.DNSType), SecretName: ptr.To("dns-secret"), Primary: ptr.To(true)},
							},
						}
						validSecret := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{Name: "dns-secret", Namespace: namespace},
							Data: map[string][]byte{
								azure.DNSSubscriptionIDKey: []byte("a6ad693a-028a-422c-b064-d76a4586f2b3"),
								azure.DNSTenantIDKey:       []byte("ee16e593-3035-41b9-a217-958f8f75b750"),
								azure.DNSClientIDKey:       []byte("7fc4685e-3c33-40e6-b6bf-7857cab04390"),
								azure.DNSClientSecretKey:   []byte("secret"),
							},
						}
						reader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "dns-secret"},
							&corev1.Secret{}).
							DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
								*obj = *validSecret
								return nil
							})

						Expect(shootValidator.Validate(ctx, shoot, nil)).To(Succeed())
					})

					It("should skip validation for non-primary azure-dns provider", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{Type: ptr.To(azure.DNSType), SecretName: ptr.To("dns-secret"), Primary: ptr.To(false)},
							},
						}
						// No secret lookup should happen for non-primary providers

						Expect(shootValidator.Validate(ctx, shoot, nil)).To(Succeed())
					})

					It("should validate only primary provider when multiple azure-dns providers exist", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{Type: ptr.To(azure.DNSType), SecretName: ptr.To("dns-secret-1"), Primary: ptr.To(true)},
								{Type: ptr.To(azure.DNSType), SecretName: ptr.To("dns-secret-2"), Primary: ptr.To(false)},
							},
						}
						validSecret := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{Name: "dns-secret-1", Namespace: namespace},
							Data: map[string][]byte{
								azure.DNSSubscriptionIDKey: []byte("a6ad693a-028a-422c-b064-d76a4586f2b3"),
								azure.DNSTenantIDKey:       []byte("ee16e593-3035-41b9-a217-958f8f75b750"),
								azure.DNSClientIDKey:       []byte("7fc4685e-3c33-40e6-b6bf-7857cab04390"),
								azure.DNSClientSecretKey:   []byte("secret"),
							},
						}
						// Only the primary provider's secret should be validated
						reader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "dns-secret-1"},
							&corev1.Secret{}).
							SetArg(2, *validSecret).
							Return(nil)

						Expect(shootValidator.Validate(ctx, shoot, nil)).To(Succeed())
					})

					It("should skip all providers when none are primary", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{Type: ptr.To(azure.DNSType), SecretName: ptr.To("dns-secret-1"), Primary: ptr.To(false)},
								{Type: ptr.To(azure.DNSType), SecretName: ptr.To("dns-secret-2"), Primary: ptr.To(false)},
							},
						}
						// No secret lookups should happen

						Expect(shootValidator.Validate(ctx, shoot, nil)).To(Succeed())
					})

					It("should validate mixed provider types with primary azure-dns", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{Type: ptr.To("aws-route53"), SecretName: ptr.To("aws-secret"), Primary: ptr.To(false)},
								{Type: ptr.To(azure.DNSType), SecretName: ptr.To("dns-secret"), Primary: ptr.To(true)},
								{Type: ptr.To("google-clouddns"), SecretName: ptr.To("gcp-secret"), Primary: ptr.To(false)},
							},
						}
						validSecret := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{Name: "dns-secret", Namespace: namespace},
							Data: map[string][]byte{
								azure.DNSSubscriptionIDKey: []byte("a6ad693a-028a-422c-b064-d76a4586f2b3"),
								azure.DNSTenantIDKey:       []byte("ee16e593-3035-41b9-a217-958f8f75b750"),
								azure.DNSClientIDKey:       []byte("7fc4685e-3c33-40e6-b6bf-7857cab04390"),
								azure.DNSClientSecretKey:   []byte("secret"),
							},
						}
						// Only azure-dns primary provider should be validated
						reader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "dns-secret"},
							&corev1.Secret{}).
							SetArg(2, *validSecret).
							Return(nil)

						Expect(shootValidator.Validate(ctx, shoot, nil)).To(Succeed())
					})
				})

				Context("#credentialsRef", func() {
					It("should return error when azure-dns provider Secret not found", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{
									Type:    ptr.To(azure.DNSType),
									Primary: ptr.To(true),
									CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
										APIVersion: "v1",
										Kind:       "Secret",
										Name:       "dns-secret",
									},
								},
							},
						}
						reader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "dns-secret"},
							&corev1.Secret{}).
							Return(apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, "dns-secret"))

						Expect(shootValidator.Validate(ctx, shoot, nil)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeNotFound),
							"Field": Equal("spec.dns.providers[0].credentialsRef"),
						}))))
					})

					It("should return error when azure-dns provider WorkloadIdentity not found", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{
									Type:    ptr.To(azure.DNSType),
									Primary: ptr.To(true),
									CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
										APIVersion: "security.gardener.cloud/v1alpha1",
										Kind:       "WorkloadIdentity",
										Name:       "dns-workload-identity",
									},
								},
							},
						}
						reader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "dns-workload-identity"},
							&securityv1alpha1.WorkloadIdentity{}).
							Return(apierrors.NewNotFound(schema.GroupResource{Resource: "workloadidentities"}, "dns-workload-identity"))

						Expect(shootValidator.Validate(ctx, shoot, nil)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeNotFound),
							"Field": Equal("spec.dns.providers[0].credentialsRef"),
						}))))
					})

					It("should succeed with valid azure-dns Secret", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{
									Type:    ptr.To(azure.DNSType),
									Primary: ptr.To(true),
									CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
										APIVersion: "v1",
										Kind:       "Secret",
										Name:       "dns-secret",
									},
								},
							},
						}
						validSecret := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{Name: "dns-secret", Namespace: namespace},
							Data: map[string][]byte{
								azure.DNSSubscriptionIDKey: []byte("a6ad693a-028a-422c-b064-d76a4586f2b3"),
								azure.DNSTenantIDKey:       []byte("ee16e593-3035-41b9-a217-958f8f75b750"),
								azure.DNSClientIDKey:       []byte("7fc4685e-3c33-40e6-b6bf-7857cab04390"),
								azure.DNSClientSecretKey:   []byte("secret"),
							},
						}
						reader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "dns-secret"},
							&corev1.Secret{}).
							SetArg(2, *validSecret).
							Return(nil)

						Expect(shootValidator.Validate(ctx, shoot, nil)).To(Succeed())
					})

					It("should return error with invalid azure-dns Secret", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{
									Type:    ptr.To(azure.DNSType),
									Primary: ptr.To(true),
									CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
										APIVersion: "v1",
										Kind:       "Secret",
										Name:       "dns-secret",
									},
								},
							},
						}
						invalidSecret := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{Name: "dns-secret", Namespace: namespace},
							Data: map[string][]byte{
								"foo": []byte("bar"),
							},
						}
						reader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "dns-secret"},
							&corev1.Secret{}).
							SetArg(2, *invalidSecret).
							Return(nil)

						Expect(shootValidator.Validate(ctx, shoot, nil)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.dns.providers[0].credentialsRef"),
							"Detail": And(
								ContainSubstring("missing required field \"subscriptionID\""),
								ContainSubstring("missing required field \"tenantID\""),
								ContainSubstring("missing required field \"clientID\""),
								ContainSubstring("missing required field \"clientSecret\""),
							),
						}))))
					})

					It("should succeed with valid azure-dns WorkloadIdentity", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{
									Type:    ptr.To(azure.DNSType),
									Primary: ptr.To(true),
									CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
										APIVersion: "security.gardener.cloud/v1alpha1",
										Kind:       "WorkloadIdentity",
										Name:       "dns-workload-identity",
									},
								},
							},
						}
						validWorkloadIdentity := &securityv1alpha1.WorkloadIdentity{
							ObjectMeta: metav1.ObjectMeta{Name: "dns-workload-identity", Namespace: namespace},
							Spec: securityv1alpha1.WorkloadIdentitySpec{
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
						reader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "dns-workload-identity"},
							&securityv1alpha1.WorkloadIdentity{}).
							SetArg(2, *validWorkloadIdentity).
							Return(nil)

						Expect(shootValidator.Validate(ctx, shoot, nil)).To(Succeed())
					})

					It("should return error with invalid azure-dns WorkloadIdentity", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{
									Type:    ptr.To(azure.DNSType),
									Primary: ptr.To(true),
									CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
										APIVersion: "security.gardener.cloud/v1alpha1",
										Kind:       "WorkloadIdentity",
										Name:       "dns-workload-identity",
									},
								},
							},
						}
						invalidWorkloadIdentity := &securityv1alpha1.WorkloadIdentity{
							ObjectMeta: metav1.ObjectMeta{Name: "dns-workload-identity", Namespace: namespace},
							Spec: securityv1alpha1.WorkloadIdentitySpec{
								TargetSystem: securityv1alpha1.TargetSystem{
									Type: "azure",
									ProviderConfig: &runtime.RawExtension{
										Raw: []byte(`
apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
kind: WorkloadIdentityConfig
clientID: "client-id"
tenantID: "tenant-id"
subscriptionID: "subscription-id"
`),
									},
								},
							},
						}
						reader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "dns-workload-identity"},
							&securityv1alpha1.WorkloadIdentity{}).
							SetArg(2, *invalidWorkloadIdentity).
							Return(nil)

						Expect(shootValidator.Validate(ctx, shoot, nil)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.dns.providers[0].credentialsRef"),
							"Detail": And(
								ContainSubstring("clientID should be a valid GUID"),
								ContainSubstring("tenantID should be a valid GUID"),
								ContainSubstring("subscriptionID should be a valid GUID"),
							),
						}))))
					})

					It("should return error with unsupported credentials type", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{
									Type:    ptr.To(azure.DNSType),
									Primary: ptr.To(true),
									CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
										APIVersion: "foo.bar/v1",
										Kind:       "Baz",
										Name:       "dns-baz-ref",
									},
								},
							},
						}

						Expect(shootValidator.Validate(ctx, shoot, nil)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInternal),
							"Field":  Equal("spec.dns.providers[0].credentialsRef"),
							"Detail": ContainSubstring("unsupported credentials reference: garden-dev/dns-baz-ref, foo.bar/v1, Kind=Baz"),
						}))))
					})

					It("should return error with InternalSecret type", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{
									Type:    ptr.To(azure.DNSType),
									Primary: ptr.To(true),
									CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
										APIVersion: "core.gardener.cloud/v1beta1",
										Kind:       "InternalSecret",
										Name:       "dns-internal-secret-ref",
									},
								},
							},
						}
						internalSecret := &gardencorev1beta1.InternalSecret{
							ObjectMeta: metav1.ObjectMeta{Name: "dns-internal-secret-ref", Namespace: namespace},
							Data: map[string][]byte{
								"foo": []byte("bar"),
							},
						}
						reader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: "dns-internal-secret-ref"},
							&gardencorev1beta1.InternalSecret{}).
							SetArg(2, *internalSecret).
							Return(nil)

						Expect(shootValidator.Validate(ctx, shoot, nil)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("spec.dns.providers[0].credentialsRef"),
							"Detail": ContainSubstring("supported credentials types are Secret and WorkloadIdentity"),
						}))))
					})
				})
			})
		})

		Context("Workerless Shoot", func() {
			BeforeEach(func() {
				shoot.Spec.Provider.Workers = nil
			})

			It("should not validate", func() {
				err := shootValidator.Validate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}
