// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package topology

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Topology Webhook Suite")
}

var _ = Describe("Topology", func() {
	var (
		ctrl *gomock.Controller

		ctx       context.Context
		pod       *corev1.Pod
		mutator   *handler
		region    = "westeurope"
		namespace = "namespace"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.Background()

		mutator = New(&admission.Decoder{}, logr.Discard(), AddOptions{
			SeedRegion:   region,
			SeedProvider: azure.Type,
		})

		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
			},
			Spec: corev1.PodSpec{
				Affinity: &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:    corev1.LabelTopologyZone,
											Values: []string{"1"},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		var err error
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("#Webhook", func() {
		It("should not mutate on update operations", func() {
			podCopy := pod.DeepCopy()

			err := mutator.Mutate(context.Background(), pod, pod)
			Expect(err).To(BeNil())
			Expect(pod).To(Equal(podCopy))
		})

		Describe("#Mutate", func() {
			BeforeEach(func() {
				pod = &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
					},
					Spec: corev1.PodSpec{
						Affinity: &corev1.Affinity{
							NodeAffinity: &corev1.NodeAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:    corev1.LabelTopologyZone,
													Values: []string{"1", "2", fmt.Sprintf("%s-%d", region, 3)},
												},
												{
													Key:    "foo",
													Values: []string{"bar"},
												},
											},
										},
									},
								},
							},
						},
					},
				}
			})
			It("it should correctly mutate required", func() {
				err := mutator.Mutate(ctx, pod, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values[0]).To(Equal(fmt.Sprintf("%s-%s", region, "1")))
				Expect(pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values[1]).To(Equal(fmt.Sprintf("%s-%s", region, "2")))
				Expect(pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values[2]).To(Equal(fmt.Sprintf("%s-%s", region, "3")))
				Expect(pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[1].Values[0]).To(Equal("bar"))
			})

			It("should correctly mutate preferred", func() {
				pod = &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
					},
					Spec: corev1.PodSpec{
						Affinity: &corev1.Affinity{
							NodeAffinity: &corev1.NodeAffinity{
								PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
									{
										Preference: corev1.NodeSelectorTerm{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:    corev1.LabelTopologyZone,
													Values: []string{"1", "2", fmt.Sprintf("%s-%d", region, 3)},
												},
												{
													Key:    "foo",
													Values: []string{"bar"},
												},
											},
										},
									},
								},
							},
						},
					},
				}

				err := mutator.Mutate(ctx, pod, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions[0].Values[0]).To(Equal(fmt.Sprintf("%s-%s", region, "1")))
				Expect(pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions[0].Values[1]).To(Equal(fmt.Sprintf("%s-%s", region, "2")))
				Expect(pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions[0].Values[2]).To(Equal(fmt.Sprintf("%s-%s", region, "3")))
				Expect(pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions[1].Values[0]).To(Equal("bar"))
			})
		})
	})
})
