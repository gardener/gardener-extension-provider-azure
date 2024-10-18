// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"encoding/json"
	"strconv"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	azurev1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

const (
	workerCIDR = "10.0.0.0/16"
)

var _ = Describe("Mutate", func() {
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#NetworkLayoutMigration", func() {
		var mutator extensionswebhook.Mutator

		BeforeEach(func() {
			mutator = newLayoutMutator(logger)
		})

		Context("add migration annotation", func() {
			var workersConfig, zonesConfig *azurev1alpha1.InfrastructureConfig

			BeforeEach(func() {
				workersConfig = &azurev1alpha1.InfrastructureConfig{
					TypeMeta: metav1.TypeMeta{
						Kind:       "InfrastructureConfig",
						APIVersion: azurev1alpha1.SchemeGroupVersion.String(),
					},
					Networks: azurev1alpha1.NetworkConfig{
						Workers: ptr.To(workerCIDR),
					},
					Zoned: true,
				}

				zonesConfig = &azurev1alpha1.InfrastructureConfig{
					TypeMeta: metav1.TypeMeta{
						Kind:       "InfrastructureConfig",
						APIVersion: azurev1alpha1.SchemeGroupVersion.String(),
					},
					Networks: azurev1alpha1.NetworkConfig{
						Zones: []azurev1alpha1.Zone{
							{
								Name: int32(1),
								CIDR: "10.11.0.0/16",
							},
							{
								Name: int32(2),
								CIDR: workerCIDR,
							},
						},
					},
					Zoned: true,
				}
			})
			It("should mutate the resource when migrating network layout", func() {
				oldInfra := generateInfrastructureWithProviderConfig(workersConfig, nil)
				newInfra := generateInfrastructureWithProviderConfig(zonesConfig, nil)

				err := mutator.Mutate(context.TODO(), newInfra, oldInfra)

				Expect(err).To(BeNil())
				v, ok := getLayoutMigrationAnnotation(newInfra)
				Expect(ok).To(BeTrue())
				Expect(v).To(Equal("2"))
			})
			It("should do nothing if network setup stays the same", func() {
				newInfra := generateInfrastructureWithProviderConfig(workersConfig, nil)

				err := mutator.Mutate(context.TODO(), newInfra, newInfra)

				Expect(err).To(BeNil())
				_, ok := getLayoutMigrationAnnotation(newInfra)
				Expect(ok).To(BeFalse())
			})
			It("should do nothing if it is a create operation", func() {
				newInfra := generateInfrastructureWithProviderConfig(zonesConfig, nil)

				err := mutator.Mutate(context.TODO(), newInfra, nil)

				Expect(err).To(BeNil())
				_, ok := getLayoutMigrationAnnotation(newInfra)
				Expect(ok).To(BeFalse())
			})
			It("should do nothing if network setup stays the same with zonal layout", func() {
				newInfra := generateInfrastructureWithProviderConfig(zonesConfig, nil)

				err := mutator.Mutate(context.TODO(), newInfra, newInfra)

				Expect(err).To(BeNil())
				_, ok := getLayoutMigrationAnnotation(newInfra)
				Expect(ok).To(BeFalse())
			})
			It("should mutate the resource if the current 'gardener.cloud/operation' annotation is 'restore' and has flow state", func() {
				newInfra := generateInfrastructureWithProviderConfig(zonesConfig, nil)
				newInfra.Annotations = map[string]string{
					"gardener.cloud/operation": "restore",
				}

				state := &azurev1alpha1.InfrastructureState{
					TypeMeta: helper.InfrastructureStateTypeMeta,
					Data: map[string]string{
						azure.NetworkLayoutZoneMigrationAnnotation: "2",
					},
				}
				marshalled, err := json.Marshal(state)
				Expect(err).To(BeNil())
				newInfra.Status.State = &runtime.RawExtension{Raw: marshalled}

				err = mutator.Mutate(context.TODO(), newInfra, newInfra)
				Expect(err).To(BeNil())
				v, ok := getLayoutMigrationAnnotation(newInfra)
				Expect(ok).To(BeTrue())
				Expect(v).To(Equal("2"))
			})
			It("should mutate the resource if the current 'gardener.cloud/operation' annotation is restore and has terraform state", func() {
				newInfra := generateInfrastructureWithProviderConfig(zonesConfig, nil)
				newInfra.Annotations = map[string]string{
					"gardener.cloud/operation": "restore",
				}

				status := &azurev1alpha1.InfrastructureStatus{
					TypeMeta: metav1.TypeMeta{
						APIVersion: azurev1alpha1.SchemeGroupVersion.String(),
						Kind:       "InfrastructureStatus",
					},
					Networks: azurev1alpha1.NetworkStatus{
						Subnets: []azurev1alpha1.Subnet{
							{
								Name:     "subnet-zone1",
								Zone:     ptr.To("1"),
								Migrated: false,
							},
							{
								Name:     "subnet",
								Zone:     ptr.To("2"),
								Migrated: true,
							},
						},
					},
				}
				marshalled, err := json.Marshal(status)
				Expect(err).To(BeNil())

				state := &infrastructure.InfrastructureState{
					SavedProviderStatus: &runtime.RawExtension{
						Raw: marshalled,
					},
				}
				marshalledState, err := json.Marshal(state)
				Expect(err).To(BeNil())
				newInfra.Status.State = &runtime.RawExtension{Raw: marshalledState}

				err = mutator.Mutate(context.TODO(), newInfra, newInfra)
				Expect(err).To(BeNil())
				v, ok := getLayoutMigrationAnnotation(newInfra)
				Expect(ok).To(BeTrue())
				Expect(v).To(Equal("2"))
			})
		})

		Context("remove migration annotation", func() {
			var (
				migratedSubnet int
				zonesInfra     *extensionsv1alpha1.Infrastructure
				zonesConfig    *azurev1alpha1.InfrastructureConfig
			)

			BeforeEach(func() {
				migratedSubnet = 1

				zonesConfig = &azurev1alpha1.InfrastructureConfig{
					TypeMeta: metav1.TypeMeta{
						Kind:       "InfrastructureConfig",
						APIVersion: azurev1alpha1.SchemeGroupVersion.String(),
					},
					Zoned: true,
					Networks: azurev1alpha1.NetworkConfig{
						Zones: []azurev1alpha1.Zone{
							{
								Name: int32(1),
							},
							{
								Name: int32(2),
							},
						},
					},
				}
				zonesInfra = generateInfrastructureWithProviderConfig(zonesConfig, nil)
				addLayoutMigrationAnnotation(zonesInfra, migratedSubnet)
			})
			It("should remove the annotation when the zone is no longer in use", func() {
				zonesConfig.Networks.Zones = zonesConfig.Networks.Zones[1:]
				newZonesInfra := generateInfrastructureWithProviderConfig(zonesConfig, nil)
				addLayoutMigrationAnnotation(newZonesInfra, migratedSubnet)

				err := mutator.Mutate(context.TODO(), newZonesInfra, zonesInfra)
				Expect(err).To(BeNil())
				_, ok := getLayoutMigrationAnnotation(newZonesInfra)
				Expect(ok).To(BeFalse())
			})
			It("should keep the annotation is the zone is still in use", func() {
				err := mutator.Mutate(context.TODO(), zonesInfra, nil)
				Expect(err).To(BeNil())
				a, ok := getLayoutMigrationAnnotation(zonesInfra)
				Expect(ok).To(BeTrue())
				Expect(a).To(Equal(strconv.Itoa(migratedSubnet)))
			})
		})
	})
})

func generateInfrastructureWithProviderConfig(config *azurev1alpha1.InfrastructureConfig, status *azurev1alpha1.IdentityStatus) *extensionsv1alpha1.Infrastructure {
	infra := &extensionsv1alpha1.Infrastructure{}

	if config != nil {
		marshalled, err := json.Marshal(config)
		Expect(err).To(BeNil())

		infra.Spec.DefaultSpec.ProviderConfig = &runtime.RawExtension{
			Raw: marshalled,
		}
	}

	if status != nil {
		marshalled, err := json.Marshal(status)
		Expect(err).To(BeNil())

		infra.Status.ProviderStatus = &runtime.RawExtension{
			Raw: marshalled,
		}
	}

	return infra
}

func getLayoutMigrationAnnotation(o *extensionsv1alpha1.Infrastructure) (string, bool) {
	return getAnnotation(azure.NetworkLayoutZoneMigrationAnnotation, o)
}

func addLayoutMigrationAnnotation(o *extensionsv1alpha1.Infrastructure, zone int) {
	if o.Annotations == nil {
		o.Annotations = make(map[string]string)
	}
	o.Annotations[azure.NetworkLayoutZoneMigrationAnnotation] = strconv.Itoa(zone)
}

func getAnnotation(anno string, o extensionsv1alpha1.Object) (string, bool) {
	v, ok := o.GetAnnotations()[anno]
	return v, ok
}
