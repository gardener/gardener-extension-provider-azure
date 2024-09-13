// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"
	"encoding/json"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
			mgr = mockmanager.NewMockManager(ctrl)

			mgr.EXPECT().GetScheme().Return(scheme).Times(2)
			mgr.EXPECT().GetClient().Return(c)

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
