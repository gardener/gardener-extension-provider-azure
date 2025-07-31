// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"
)

var _ = Describe("Shoot validation", func() {
	Describe("#ValidateNetworking", func() {

		networkingPath := field.NewPath("spec", "networking")
		DescribeTable("",
			func(networking core.Networking, result types.GomegaMatcher) {
				errorList := ValidateNetworking(&networking, networkingPath)

				Expect(errorList).To(result)
			},

			Entry("should return no error because nodes CIDR was provided",
				core.Networking{
					Nodes: ptr.To("1.2.3.4/5"),
				},
				BeEmpty(),
			),
			Entry("should return an error because no nodes CIDR was provided",
				core.Networking{},
				ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.networking.nodes"),
					})),
				),
			),
			Entry("should return an error if calico is used with overlay network",
				core.Networking{
					Nodes:          ptr.To("1.2.3.4/5"),
					Type:           ptr.To("calico"),
					ProviderConfig: &runtime.RawExtension{Raw: []byte(`{"apiVersion":"calico.networking.extensions.gardener.cloud/v1alpha1","kind":"NetworkConfig","overlay":{"enabled":true}}`)},
				},
				ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("spec.networking.providerConfig.overlay.enabled"),
					})),
				),
			),
			Entry("should return no error if calico is used without overlay network",
				core.Networking{
					Nodes:          ptr.To("1.2.3.4/5"),
					Type:           ptr.To("calico"),
					ProviderConfig: &runtime.RawExtension{Raw: []byte(`{"apiVersion":"calico.networking.extensions.gardener.cloud/v1alpha1","kind":"NetworkConfig","overlay":{"enabled":false}}`)},
				},
				BeEmpty(),
			),
			Entry("should return no error if cilium is used with overlay network",
				core.Networking{
					Nodes:          ptr.To("1.2.3.4/5"),
					Type:           ptr.To("cilium"),
					ProviderConfig: &runtime.RawExtension{Raw: []byte(`{"apiVersion":"cilium.networking.extensions.gardener.cloud/v1alpha1","kind":"NetworkConfig","overlay":{"enabled":true}}`)},
				},
				BeEmpty(),
			),
		)
	})

	Describe("#ValidateWorkerConfig", func() {
		var (
			workers     []core.Worker
			infraConfig *api.InfrastructureConfig
		)

		BeforeEach(func() {
			workers = []core.Worker{
				{
					Name: "worker1",
					Volume: &core.Volume{
						Type:       ptr.To("Volume"),
						VolumeSize: "30G",
					},
				},
				{
					Name: "worker2",
					Volume: &core.Volume{
						Type:       ptr.To("Volume"),
						VolumeSize: "20G",
					},
				},
			}
		})

		Describe("#ValidateWorkers", func() {
			BeforeEach(func() {
				infraConfig = &api.InfrastructureConfig{}
			})

			Context("Non zoned cluster", func() {
				BeforeEach(func() {
					infraConfig.Zoned = false
				})

				It("should pass because workers are configured correctly", func() {
					errorList := ValidateWorkers(workers,
						infraConfig, field.NewPath(""))

					Expect(errorList).To(BeEmpty())
				})

				It("should forbid because zones are configured", func() {
					workers[0].Zones = []string{"1", "2"}
					errorList := ValidateWorkers(workers,
						infraConfig, field.NewPath("workers"))

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("workers[0].zones"),
						})),
					))
				})
			})

			Context("Zoned cluster", func() {
				BeforeEach(func() {
					infraConfig.Zoned = true
					workers[0].Zones = []string{"1", "2"}
					workers[1].Zones = []string{"1", "2"}
				})

				It("should pass because workers are configured correctly", func() {
					errorList := ValidateWorkers(workers, infraConfig, field.NewPath(""))

					Expect(errorList).To(BeEmpty())
				})

				It("should forbid because volume is not configured", func() {
					workers[1].Volume = nil

					errorList := ValidateWorkers(workers,
						infraConfig, field.NewPath("workers"))

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("workers[1].volume"),
						})),
					))
				})

				It("should forbid because volume type and size are not configured", func() {
					workers[0].Volume.Type = nil
					workers[0].Volume.VolumeSize = ""
					workers[0].Volume.Encrypted = ptr.To(false)
					workers[0].DataVolumes = []core.DataVolume{{Encrypted: ptr.To(true)}}

					errorList := ValidateWorkers(workers,
						infraConfig, field.NewPath("workers"))

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("workers[0].volume.type"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("workers[0].volume.size"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeNotSupported),
							"Field": Equal("workers[0].volume.encrypted"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("workers[0].dataVolumes[0].type"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("workers[0].dataVolumes[0].size"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeNotSupported),
							"Field": Equal("workers[0].dataVolumes[0].encrypted"),
						})),
					))
				})

				It("should forbid because of too many data volumes", func() {
					for i := 0; i <= 64; i++ {
						workers[0].DataVolumes = append(workers[0].DataVolumes, core.DataVolume{
							Name:       "foo",
							VolumeSize: "20Gi",
							Type:       ptr.To("foo"),
						})
					}

					errorList := ValidateWorkers(workers,
						infraConfig, field.NewPath("workers"))

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeTooMany),
							"Field": Equal("workers[0].dataVolumes"),
						})),
					))
				})

				It("should forbid because worker does not specify a zone", func() {
					workers[0].Zones = nil

					errorList := ValidateWorkers(workers,
						infraConfig, field.NewPath("workers"))

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("workers[0].zones"),
						})),
					))
				})

				It("should forbid because worker use zone twice", func() {
					workers[0].Zones[1] = workers[0].Zones[0]

					errorList := ValidateWorkers(workers,
						infraConfig, field.NewPath("workers"))

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("workers[0].zones[1]"),
						})),
					))
				})

				Context("multiple subnet network layout", func() {
					BeforeEach(func() {
						infraConfig = &api.InfrastructureConfig{
							Zoned: true,
							Networks: api.NetworkConfig{
								Zones: []api.Zone{
									{
										Name: 1,
									},
									{
										Name: 2,
									},
								},
							},
						}
					})

					It("should forbid using zones not configured in infrastructure", func() {
						workers[0].Zones[0] = "non-existent"
						errorList := ValidateWorkers(workers,
							infraConfig, field.NewPath("workers"))

						Expect(errorList).To(ConsistOf(
							PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeInvalid),
								"Field": Equal("workers[0].zones[0]"),
							})),
						))
					})

					It("should allow zones when configured in infrastructure", func() {
						errorList := ValidateWorkers(workers,
							infraConfig, field.NewPath("workers"))

						Expect(errorList).To(BeEmpty())
					})
				})
			})
		})

		Describe("#ValidateWorkersUpdate", func() {
			Context("Zoned cluster", func() {
				BeforeEach(func() {
					workers[0].Zones = []string{"1", "2"}
					workers[1].Zones = []string{"1", "2"}
				})

				It("should pass because workers are unchanged", func() {
					newWorkers := copyWorkers(workers)
					errorList := ValidateWorkersUpdate(workers, newWorkers, field.NewPath("workers"))

					Expect(errorList).To(BeEmpty())
				})

				It("should allow adding workers", func() {
					newWorkers := append(workers[:0:0], workers...)
					workers = workers[:1]
					errorList := ValidateWorkersUpdate(workers, newWorkers, field.NewPath("workers"))

					Expect(errorList).To(BeEmpty())
				})

				It("should allow adding a zone to a worker", func() {
					newWorkers := copyWorkers(workers)
					newWorkers[0].Zones = append(newWorkers[0].Zones, "another-zone")
					errorList := ValidateWorkersUpdate(workers, newWorkers, field.NewPath("workers"))

					Expect(errorList).To(BeEmpty())
				})

				It("should forbid removing a zone from a worker", func() {
					newWorkers := copyWorkers(workers)
					newWorkers[1].Zones = newWorkers[1].Zones[1:]
					errorList := ValidateWorkersUpdate(workers, newWorkers, field.NewPath("workers"))

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("workers[1].zones"),
						})),
					))
				})

				It("should forbid changing the zone order", func() {
					newWorkers := copyWorkers(workers)
					newWorkers[0].Zones[0] = workers[0].Zones[1]
					newWorkers[0].Zones[1] = workers[0].Zones[0]
					newWorkers[1].Zones[0] = workers[1].Zones[1]
					newWorkers[1].Zones[1] = workers[1].Zones[0]
					errorList := ValidateWorkersUpdate(workers, newWorkers, field.NewPath("workers"))

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("workers[0].zones"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("workers[1].zones"),
						})),
					))
				})

				It("should forbid adding a zone while changing an existing one", func() {
					newWorkers := copyWorkers(workers)
					newWorkers = append(newWorkers, core.Worker{Name: "worker3", Zones: []string{"zone1"}})
					newWorkers[1].Zones[0] = workers[1].Zones[1]
					errorList := ValidateWorkersUpdate(workers, newWorkers, field.NewPath("workers"))

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("workers[1].zones"),
						})),
					))
				})
			})

			It("should forbid changing the providerConfig if the update strategy is in-place", func() {
				workers[0].UpdateStrategy = ptr.To(core.AutoInPlaceUpdate)
				workers[0].ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"foo":"bar"}`),
				}

				workers[1].Name = "worker2"
				workers[1].UpdateStrategy = ptr.To(core.ManualInPlaceUpdate)
				workers[1].ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"zoo":"dash"}`),
				}

				// provider config changed but update strategy is not in-place
				workers = append(workers, core.Worker{
					Name:           "worker3",
					UpdateStrategy: ptr.To(core.AutoRollingUpdate),
					ProviderConfig: &runtime.RawExtension{
						Raw: []byte(`{"bar":"foo"}`),
					},
				})

				// no change in provider config
				workers = append(workers, core.Worker{
					Name:           "worker4",
					UpdateStrategy: ptr.To(core.AutoInPlaceUpdate),
					ProviderConfig: &runtime.RawExtension{
						Raw: []byte(`{"bar":"foo"}`),
					},
				})

				newWorkers := copyWorkers(workers)
				newWorkers[0].ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"foo":"baz"}`),
				}
				newWorkers[1].ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"zoo":"bash"}`),
				}
				newWorkers[2].ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"bar":"baz"}`),
				}

				Expect(ValidateWorkersUpdate(workers, newWorkers, field.NewPath("spec", "provider", "workers"))).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.provider.workers[0].providerConfig"),
						"Detail": Equal("providerConfig is immutable when update strategy is in-place"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.provider.workers[1].providerConfig"),
						"Detail": Equal("providerConfig is immutable when update strategy is in-place"),
					})),
				))
			})

			It("should forbid changing the data volumes if the update strategy is in-place", func() {
				workers[0].UpdateStrategy = ptr.To(core.AutoInPlaceUpdate)
				workers[0].DataVolumes = []core.DataVolume{
					{
						Name:       "foo",
						VolumeSize: "20Gi",
						Type:       ptr.To("foo"),
					},
				}

				workers[1].Name = "worker2"
				workers[1].UpdateStrategy = ptr.To(core.ManualInPlaceUpdate)
				workers[1].DataVolumes = []core.DataVolume{
					{
						Name:       "bar",
						VolumeSize: "30Gi",
						Type:       ptr.To("bar"),
					},
				}

				newWorkers := copyWorkers(workers)
				newWorkers[0].DataVolumes = []core.DataVolume{
					{
						Name:       "baz",
						VolumeSize: "40Gi",
						Type:       ptr.To("baz"),
					},
				}
				newWorkers[1].DataVolumes = []core.DataVolume{
					{
						Name:       "qux",
						VolumeSize: "50Gi",
						Type:       ptr.To("qux"),
					},
				}

				Expect(ValidateWorkersUpdate(workers, newWorkers, field.NewPath("spec", "provider", "workers"))).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.provider.workers[0].dataVolumes"),
						"Detail": Equal("dataVolumes are immutable when update strategy is in-place"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.provider.workers[1].dataVolumes"),
						"Detail": Equal("dataVolumes are immutable when update strategy is in-place"),
					})),
				))
			})
		})
	})
})

func copyWorkers(workers []core.Worker) []core.Worker {
	cp := append(workers[:0:0], workers...)
	for i := range cp {
		cp[i].Zones = append(workers[i].Zones[:0:0], workers[i].Zones...)
	}
	return cp
}
