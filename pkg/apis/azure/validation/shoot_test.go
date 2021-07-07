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
	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"
	"github.com/gardener/gardener/pkg/apis/core"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

var _ = Describe("Shoot validation", func() {
	Describe("#ValidateNetworking", func() {
		var networkingPath = field.NewPath("spec", "networking")

		It("should return no error because nodes CIDR was provided", func() {
			networking := core.Networking{
				Nodes: pointer.StringPtr("1.2.3.4/5"),
			}

			errorList := ValidateNetworking(networking, networkingPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should return an error because no nodes CIDR was provided", func() {
			networking := core.Networking{}

			errorList := ValidateNetworking(networking, networkingPath)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.networking.nodes"),
				})),
			))
		})
	})

	Describe("#ValidateWorkerConfig", func() {
		var (
			workers     []core.Worker
			infraConfig *apisazure.InfrastructureConfig
		)

		BeforeEach(func() {
			workers = []core.Worker{
				{
					Name: "worker1",
					Volume: &core.Volume{
						Type:       pointer.StringPtr("Volume"),
						VolumeSize: "30G",
					},
				},
				{
					Name: "worker2",
					Volume: &core.Volume{
						Type:       pointer.StringPtr("Volume"),
						VolumeSize: "20G",
					},
				},
			}
		})

		Describe("#ValidateWorkers", func() {
			Context("Non zoned cluster", func() {
				BeforeEach(func() {
					infraConfig = &apisazure.InfrastructureConfig{
						Zoned: false,
					}
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
					infraConfig = &apisazure.InfrastructureConfig{
						Zoned: true,
					}
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
					workers[0].Volume.Encrypted = pointer.BoolPtr(false)
					workers[0].DataVolumes = []core.DataVolume{{Encrypted: pointer.BoolPtr(true)}}

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
							Type:       pointer.StringPtr("foo"),
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

				Context("multi-zone subnets", func() {
					BeforeEach(func() {
						infraConfig = &apisazure.InfrastructureConfig{
							Zoned: true,
							Networks: apisazure.NetworkConfig{
								Zones: []apisazure.Zone{
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
	copy := append(workers[:0:0], workers...)
	for i := range copy {
		copy[i].Zones = append(workers[i].Zones[:0:0], workers[i].Zones...)
	}
	return copy
}
