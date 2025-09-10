// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-azure/pkg/admission/mutator"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/features"
)

var _ = Describe("Shoot mutator", func() {

	Describe("#Mutate", func() {
		const namespace = "garden-dev"

		var (
			ctrl         *gomock.Controller
			mgr          *mockmanager.MockManager
			shootMutator extensionswebhook.Mutator
			shoot        *gardencorev1beta1.Shoot
			oldShoot     *gardencorev1beta1.Shoot
			ctx          = context.TODO()
			now          = metav1.Now()
		)

		BeforeEach(func() {
			scheme := runtime.NewScheme()
			Expect(gardencorev1beta1.AddToScheme(scheme)).To(Succeed())

			ctrl = gomock.NewController(GinkgoT())
			mgr = mockmanager.NewMockManager(ctrl)

			mgr.EXPECT().GetScheme().Return(scheme)

			shootMutator = mutator.NewShootMutator(mgr)

			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: namespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					SeedName: ptr.To("azure"),
					Provider: gardencorev1beta1.Provider{
						Type:    azure.Type,
						Workers: []gardencorev1beta1.Worker{{Name: "test"}},
					},
					Region: "eastus",
					Networking: &gardencorev1beta1.Networking{
						Nodes: ptr.To("10.250.0.0/16"),
						Type:  ptr.To("cilium"),
					},
				},
			}

			oldShoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: namespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					SeedName: ptr.To("azure"),
					Provider: gardencorev1beta1.Provider{
						Type: azure.Type,
					},
					Region: "eastus",
					Networking: &gardencorev1beta1.Networking{
						Nodes: ptr.To("10.250.0.0/16"),
						Type:  ptr.To("cilium"),
					},
				},
			}
		})
		Context("Workerless Shoot", func() {
			BeforeEach(func() {
				shoot.Spec.Provider.Workers = nil
			})

			It("should return without mutation when shoot is in scheduled to new seed phase", func() {
				shootExpected := shoot.DeepCopy()
				err := shootMutator.Mutate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot).To(DeepEqual(shootExpected))
			})
		})

		Context("Mutate shoot networking providerconfig for type cilium", func() {
			It("should return without mutation when shoot is in scheduled to new seed phase", func() {
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Description:    "test",
					LastUpdateTime: metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
					Progress:       0,
					Type:           gardencorev1beta1.LastOperationTypeReconcile,
					State:          gardencorev1beta1.LastOperationStateProcessing,
				}
				shoot.Status.SeedName = ptr.To("aws")
				shootExpected := shoot.DeepCopy()
				err := shootMutator.Mutate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot).To(DeepEqual(shootExpected))
			})

			It("should return without mutation when shoot is in migration or restore phase", func() {
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Description:    "test",
					LastUpdateTime: metav1.Time{Time: metav1.Now().Add(time.Second * -1000)},
					Progress:       0,
					Type:           gardencorev1beta1.LastOperationTypeMigrate,
					State:          gardencorev1beta1.LastOperationStateProcessing,
				}
				shoot.Status.SeedName = ptr.To("azure")
				shootExpected := shoot.DeepCopy()
				err := shootMutator.Mutate(ctx, shoot, shoot)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot).To(DeepEqual(shootExpected))
			})

			It("should return without mutation when shoot is in deletion phase", func() {
				shoot.DeletionTimestamp = &now
				shootExpected := shoot.DeepCopy()
				err := shootMutator.Mutate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot).To(DeepEqual(shootExpected))
			})

			It("should return without mutation when shoot specs have not changed", func() {
				shootWithAnnotations := shoot.DeepCopy()
				shootWithAnnotations.Annotations = map[string]string{"foo": "bar"}
				shootExpected := shootWithAnnotations.DeepCopy()

				err := shootMutator.Mutate(ctx, shootWithAnnotations, shoot)
				Expect(err).ToNot(HaveOccurred())
				Expect(shootWithAnnotations).To(DeepEqual(shootExpected))
			})

			It("should disable overlay for a new shoot", func() {
				err := shootMutator.Mutate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.Networking.ProviderConfig).To(Equal(&runtime.RawExtension{
					Raw: []byte(`{"overlay":{"enabled":false}}`),
				}))
			})

			It("should disable overlay for a new shoot with non empty network config", func() {
				shoot.Spec.Networking.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"foo":{"enabled":true}}`),
				}
				err := shootMutator.Mutate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.Networking.ProviderConfig).To(Equal(&runtime.RawExtension{
					Raw: []byte(`{"foo":{"enabled":true},"overlay":{"enabled":false}}`),
				}))
			})

			It("should take overlay field value from old shoot when unspecified in new shoot", func() {
				oldShoot.Spec.Networking.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"overlay":{"enabled":true}}`),
				}
				err := shootMutator.Mutate(ctx, shoot, oldShoot)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.Networking.ProviderConfig).To(Equal(&runtime.RawExtension{
					Raw: []byte(`{"overlay":{"enabled":true}}`),
				}))
			})
			It("should not add the overlay field for when unspecified in new and old shoot", func() {
				err := shootMutator.Mutate(ctx, shoot, oldShoot)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.Networking.ProviderConfig).To(Equal(&runtime.RawExtension{
					Raw: []byte(`{}`),
				}))
			})
		})

		Context("Mutate shoot NodeLocalDNS default for ForceTCPToUpstreamDNS property", func() {
			BeforeEach(func() {
				shoot.Spec.SystemComponents = &gardencorev1beta1.SystemComponents{
					NodeLocalDNS: &gardencorev1beta1.NodeLocalDNS{
						Enabled: true,
					},
				}
			})

			It("should not touch the ForceTCPToUpstreamDNS property if NodeLocalDNS is disabled", func() {
				shoot.Spec.SystemComponents.NodeLocalDNS.Enabled = false
				err := shootMutator.Mutate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.SystemComponents.NodeLocalDNS.ForceTCPToUpstreamDNS).To(BeNil())
			})

			It("should not touch the ForceTCPToUpstreamDNS property if it is already set", func() {
				shoot.Spec.SystemComponents.NodeLocalDNS.ForceTCPToUpstreamDNS = ptr.To(true)
				err := shootMutator.Mutate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.SystemComponents.NodeLocalDNS.ForceTCPToUpstreamDNS).ToNot(BeNil())
				Expect(*shoot.Spec.SystemComponents.NodeLocalDNS.ForceTCPToUpstreamDNS).To(BeTrue())
			})

			It("should set the ForceTCPToUpstreamDNS property to false by default", func() {
				err := shootMutator.Mutate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.SystemComponents.NodeLocalDNS.ForceTCPToUpstreamDNS).ToNot(BeNil())
				Expect(*shoot.Spec.SystemComponents.NodeLocalDNS.ForceTCPToUpstreamDNS).To(BeFalse())
			})
		})

		Context("Mutate shoot infrastructure config", func() {
			var infraConfig *v1alpha1.InfrastructureConfig

			Context("NAT-Gateway", func() {
				BeforeEach(func() {
					infraConfig = &v1alpha1.InfrastructureConfig{
						Networks: v1alpha1.NetworkConfig{},
					}
					err := features.ExtensionFeatureGate.Set(fmt.Sprintf("%s=%s", features.ForceNatGateway, "true"))
					Expect(err).NotTo(HaveOccurred(), "Failed to enable feature gate")
				})

				It("should prevent removing nat annotation", func() {
					oldShoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: encode(infraConfig)}
					oldShoot.Annotations = map[string]string{azure.ShootMutateNatConfig: "true"}
					shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: encode(infraConfig)}
					shoot.Annotations = nil
					err := shootMutator.Mutate(ctx, shoot, oldShoot)
					Expect(err).NotTo(HaveOccurred())
					Expect(shoot.Annotations).To(Equal(oldShoot.Annotations))
				})

				It("should not mutate if user brings VNET", func() {
					infraConfig.Networks.NatGateway = nil
					infraConfig.Networks.VNet.Name = ptr.To("my-vnet")
					infraConfig.Networks.VNet.ResourceGroup = ptr.To("resource-group")
					shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: encode(infraConfig)}
					err := shootMutator.Mutate(ctx, shoot, nil)
					Expect(err).NotTo(HaveOccurred())
					Expect(shoot.Spec.Provider.InfrastructureConfig.Raw).To(Equal(encode(infraConfig)))
					Expect(shoot.Annotations).To(BeNil())
				})

				It("should not mutate infrastructure config or annotations when old shoot is provided", func() {
					infraConfig.Networks.NatGateway = nil
					shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: encode(infraConfig)}
					err := shootMutator.Mutate(ctx, shoot, shoot)
					Expect(err).NotTo(HaveOccurred())
					Expect(shoot.Spec.Provider.InfrastructureConfig.Raw).To(Equal(encode(infraConfig)))
					Expect(shoot.Annotations).To(BeNil())
				})

				It("should mutate new shoot if NAT-Gateway is nil", func() {
					infraConfig.Networks.NatGateway = nil
					mutatedInfraConfig := infraConfig.DeepCopy()
					mutatedInfraConfig.Networks.NatGateway = &v1alpha1.NatGatewayConfig{Enabled: true}
					shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: encode(infraConfig)}
					err := shootMutator.Mutate(ctx, shoot, nil)
					Expect(err).NotTo(HaveOccurred())
					Expect(shoot.Spec.Provider.InfrastructureConfig.Raw).To(Equal(encode(mutatedInfraConfig)))
					Expect(shoot.Annotations).To(Not(BeNil()))
					Expect(shoot.Annotations[azure.ShootMutateNatConfig]).To(Equal("true"))
				})

				It("should not mutate if feature gate is disabled", func() {
					err := features.ExtensionFeatureGate.Set(fmt.Sprintf("%s=%s", features.ForceNatGateway, "false"))
					Expect(err).NotTo(HaveOccurred(), "Failed to enable feature gate")
					infraConfig.Networks.NatGateway = nil
					shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: encode(infraConfig)}
					err = shootMutator.Mutate(ctx, shoot, nil)
					Expect(err).NotTo(HaveOccurred())
					Expect(shoot.Spec.Provider.InfrastructureConfig.Raw).To(Equal(encode(infraConfig)))
					Expect(shoot.Annotations).To(BeNil())
				})

				It("should not mutate if NAT-Gateway is disabled", func() {
					infraConfig.Networks.NatGateway = &v1alpha1.NatGatewayConfig{Enabled: false}
					shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: encode(infraConfig)}
					err := shootMutator.Mutate(ctx, shoot, nil)
					Expect(err).NotTo(HaveOccurred())
					Expect(shoot.Spec.Provider.InfrastructureConfig.Raw).To(Equal(encode(infraConfig)))
					Expect(shoot.Annotations).ToNot(BeNil())
					Expect(shoot.Annotations[azure.ShootMutateNatConfig]).To(Equal("true"))
				})

				It("should mutate if NAT-Gateway is nil in a multi zones setup", func() {
					infraConfig.Networks.Zones = []v1alpha1.Zone{
						{
							Name: 1,
						},
						{
							Name: 2,
						},
					}
					mutatedInfraConfig := infraConfig.DeepCopy()
					mutatedInfraConfig.Networks.Zones[0].NatGateway = &v1alpha1.ZonedNatGatewayConfig{Enabled: true}
					mutatedInfraConfig.Networks.Zones[1].NatGateway = &v1alpha1.ZonedNatGatewayConfig{Enabled: true}
					shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: encode(infraConfig)}
					err := shootMutator.Mutate(ctx, shoot, nil)
					Expect(err).NotTo(HaveOccurred())
					Expect(shoot.Spec.Provider.InfrastructureConfig.Raw).To(Equal(encode(mutatedInfraConfig)))
					Expect(shoot.Annotations[azure.ShootMutateNatConfig]).To(Equal("true"))
				})

				It("should not mutate if NAT-Gateway is explicitly disabled in a multi zones setup", func() {
					infraConfig.Networks.Zones = []v1alpha1.Zone{
						{
							Name: 1,
							NatGateway: &v1alpha1.ZonedNatGatewayConfig{
								Enabled: false,
							},
						},
						{
							Name: 2,
							NatGateway: &v1alpha1.ZonedNatGatewayConfig{
								Enabled: false,
							},
						},
					}
					shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: encode(infraConfig)}
					err := shootMutator.Mutate(ctx, shoot, nil)
					Expect(err).NotTo(HaveOccurred())
					Expect(shoot.Spec.Provider.InfrastructureConfig.Raw).To(Equal(encode(infraConfig)))
					Expect(shoot.Annotations[azure.ShootMutateNatConfig]).To(Equal("true"))
				})

				Context("Prevent unsetting NAT-Gateway", func() {
					It("should keep old nat when trying to remove explicit NAT-Gateway and annotation is set", func() {
						oldShoot.Annotations = map[string]string{azure.ShootMutateNatConfig: "true"}
						oldInfra := infraConfig.DeepCopy()
						oldInfra.Networks.NatGateway = &v1alpha1.NatGatewayConfig{Enabled: true}
						infraConfig.Networks.NatGateway = nil
						shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: encode(infraConfig)}
						oldShoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: encode(oldInfra)}
						err := shootMutator.Mutate(ctx, shoot, oldShoot)
						Expect(err).NotTo(HaveOccurred())
						Expect(shoot.Spec.Provider.InfrastructureConfig.Raw).To(Equal(encode(oldInfra)))
					})

					It("should be able to unset nat if force-nat-annotation is not set", func() {
						oldInfra := infraConfig.DeepCopy()
						oldInfra.Networks.NatGateway = &v1alpha1.NatGatewayConfig{Enabled: true}
						infraConfig.Networks.NatGateway = nil
						shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: encode(infraConfig)}
						oldShoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: encode(oldInfra)}
						err := shootMutator.Mutate(ctx, shoot, oldShoot)
						Expect(err).NotTo(HaveOccurred())
						Expect(shoot.Spec.Provider.InfrastructureConfig.Raw).To(Equal(encode(infraConfig)))
						Expect(shoot.Annotations).To(BeNil())
					})

					It("should keep old explicitly set nats when trying to remove them in multi-zone setup", func() {
						oldShoot.Annotations = map[string]string{azure.ShootMutateNatConfig: "true"}
						infraConfig.Networks.Zones = []v1alpha1.Zone{{
							Name:       1,
							NatGateway: nil,
						}}
						oldInfra := infraConfig.DeepCopy()
						oldInfra.Networks.Zones[0].NatGateway = &v1alpha1.ZonedNatGatewayConfig{Enabled: true}
						shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: encode(infraConfig)}
						oldShoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: encode(oldInfra)}
						err := shootMutator.Mutate(ctx, shoot, oldShoot)
						Expect(err).NotTo(HaveOccurred())
						Expect(shoot.Spec.Provider.InfrastructureConfig.Raw).To(Equal(encode(oldInfra)))
					})

					It("should allow unsetting NAT in multi zone setup if force-nat-annotation is not set", func() {
						infraConfig.Networks.Zones = []v1alpha1.Zone{{
							Name:       1,
							NatGateway: nil,
						}}
						oldInfra := infraConfig.DeepCopy()
						oldInfra.Networks.Zones[0].NatGateway = &v1alpha1.ZonedNatGatewayConfig{Enabled: true}
						shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: encode(infraConfig)}
						oldShoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: encode(oldInfra)}
						err := shootMutator.Mutate(ctx, shoot, oldShoot)
						Expect(err).NotTo(HaveOccurred())
						Expect(shoot.Spec.Provider.InfrastructureConfig.Raw).To(Equal(encode(infraConfig)))
						Expect(shoot.Annotations).To(BeNil())
					})

					It("should allow removing nat if switching to multi-zone setup", func() {
						oldInfra := infraConfig.DeepCopy()
						oldInfra.Networks.NatGateway = &v1alpha1.NatGatewayConfig{Enabled: true}
						oldInfra.Networks.Zones = []v1alpha1.Zone{}
						infraConfig.Networks.NatGateway = nil
						infraConfig.Networks.Zones = []v1alpha1.Zone{{
							Name:       1,
							NatGateway: &v1alpha1.ZonedNatGatewayConfig{Enabled: true},
						}}
						shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: encode(infraConfig)}
						oldShoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: encode(oldInfra)}
						err := shootMutator.Mutate(ctx, shoot, oldShoot)
						Expect(err).NotTo(HaveOccurred())
						Expect(shoot.Spec.Provider.InfrastructureConfig.Raw).To(Equal(encode(infraConfig)))
					})

					It("should allow removing nat if switching to non-multi-zone setup", func() {
						oldInfra := infraConfig.DeepCopy()
						oldInfra.Networks.NatGateway = nil
						oldInfra.Zoned = true
						oldInfra.Networks.Zones = []v1alpha1.Zone{{
							Name:       1,
							NatGateway: &v1alpha1.ZonedNatGatewayConfig{Enabled: true},
						}}
						infraConfig.Networks.NatGateway = &v1alpha1.NatGatewayConfig{Enabled: true}
						infraConfig.Networks.Zones = []v1alpha1.Zone{}
						infraConfig.Zoned = true
						shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: encode(infraConfig)}
						oldShoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: encode(oldInfra)}
						err := shootMutator.Mutate(ctx, shoot, oldShoot)
						Expect(err).NotTo(HaveOccurred())
						Expect(shoot.Spec.Provider.InfrastructureConfig.Raw).To(Equal(encode(infraConfig)))
					})
				})
			})
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}
