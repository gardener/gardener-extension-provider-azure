// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validation_test

import (
	"github.com/gardener/gardener-extension-networking-calico/pkg/apis/calico/install"
	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"
)

var _ = Describe("Shoot validation", func() {
	Describe("#ValidateNetworking", func() {
		networkingPath := field.NewPath("spec", "networking")
		scheme := runtime.NewScheme()
		utilruntime.Must(install.AddToScheme(scheme))
		decoder := serializer.NewCodecFactory(scheme, serializer.EnableStrict).UniversalDecoder()

		DescribeTable("",
			func(networking core.Networking, result types.GomegaMatcher) {
				errorList := ValidateNetworking(decoder, &networking, networkingPath)

				Expect(errorList).To(result)
			},

			Entry("should return no error because nodes CIDR was provided",
				core.Networking{
					Nodes: pointer.String("1.2.3.4/5"),
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
					Nodes:          pointer.String("1.2.3.4/5"),
					Type:           pointer.String("calico"),
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
					Nodes:          pointer.String("1.2.3.4/5"),
					Type:           pointer.String("calico"),
					ProviderConfig: &runtime.RawExtension{Raw: []byte(`{"apiVersion":"calico.networking.extensions.gardener.cloud/v1alpha1","kind":"NetworkConfig","overlay":{"enabled":false}}`)},
				},
				BeEmpty(),
			),
			Entry("should return no error if cilium is used with overlay network",
				core.Networking{
					Nodes:          pointer.String("1.2.3.4/5"),
					Type:           pointer.String("cilium"),
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
						Type:       pointer.String("Volume"),
						VolumeSize: "30G",
					},
				},
				{
					Name: "worker2",
					Volume: &core.Volume{
						Type:       pointer.String("Volume"),
						VolumeSize: "20G",
					},
				},
			}
		})

		Describe("#ValidateWorkers", func() {
			BeforeEach(func() {
				infraConfig = &api.InfrastructureConfig{}
			})

			It("should pass when the kubernetes version is equal to the CSI migration version", func() {
				workers[0].Kubernetes = &core.WorkerKubernetes{Version: pointer.String("1.21.0")}

				errorList := ValidateWorkers(workers, infraConfig, field.NewPath(""))

				Expect(errorList).To(BeEmpty())
			})

			It("should pass when the kubernetes version is higher to the CSI migration version", func() {
				workers[0].Kubernetes = &core.WorkerKubernetes{Version: pointer.String("1.22.0")}

				errorList := ValidateWorkers(workers, infraConfig, field.NewPath(""))

				Expect(errorList).To(BeEmpty())
			})

			It("should not allow when the kubernetes version is lower than the CSI migration version", func() {
				workers[0].Kubernetes = &core.WorkerKubernetes{Version: pointer.String("1.20.0")}

				errorList := ValidateWorkers(workers, infraConfig, field.NewPath("workers"))

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("workers[0].kubernetes.version"),
					})),
				))
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
					errorList := ValidateWorkers(workers,
						infraConfig, field.NewPath(""))

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
					workers[0].Volume.Encrypted = pointer.Bool(false)
					workers[0].DataVolumes = []core.DataVolume{{Encrypted: pointer.Bool(true)}}

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
							Type:       pointer.String("foo"),
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
