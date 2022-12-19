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
	"encoding/json"
	"fmt"
	"testing"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Topology Webhook Suite")
}

var _ = Describe("Topology", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		pod       *corev1.Pod
		mutator   *mutator
		region    = "westeurope"
		namespace = "namespace"
		seed      = &v1beta1.Seed{
			Spec: v1beta1.SeedSpec{
				Provider: v1beta1.SeedProvider{
					Region: region,
					Type:   azure.Type,
				},
			},
		}
		seedJson []byte
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		mutator = New(logr.Discard())

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
		seedJson, err = json.Marshal(seed)
		Expect(err).NotTo(HaveOccurred())
		Expect(mutator.InjectClient(c)).NotTo(HaveOccurred())
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

		It("should try to fetch region from shoot-info", func() {
			cm := &corev1.ConfigMap{
				Data: map[string]string{
					"region": region,
				},
			}
			c.EXPECT().Get(gomock.Any(), ctrclient.ObjectKey{Namespace: metav1.NamespaceSystem, Name: constants.ConfigMapNameShootInfo}, &corev1.ConfigMap{}).DoAndReturn(clientGet(cm))

			err := mutator.Mutate(context.Background(), pod, nil)
			Expect(err).To(BeNil())
			Expect(pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values[0]).To(Equal(fmt.Sprintf("%s-%s", region, "1")))
		})

		It("should try to fetch region from cluster", func() {

			cluster := &v1alpha1.Cluster{Spec: v1alpha1.ClusterSpec{Seed: runtime.RawExtension{Raw: seedJson}}}
			gomock.InOrder(
				c.EXPECT().Get(gomock.Any(), ctrclient.ObjectKey{Namespace: metav1.NamespaceSystem, Name: constants.ConfigMapNameShootInfo}, &corev1.ConfigMap{}).Return(&apierrors.StatusError{
					ErrStatus: metav1.Status{
						Reason: metav1.StatusReasonNotFound,
					}}),
				c.EXPECT().Get(gomock.Any(), kutil.Key(pod.Namespace), &v1alpha1.Cluster{}).DoAndReturn(clientGet(cluster)),
			)
			err := mutator.Mutate(context.Background(), pod, nil)
			Expect(err).To(BeNil())

			Expect(pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values[0]).To(Equal(fmt.Sprintf("%s-%s", region, "1")))
		})
	})

	Context("#Mutator", func() {
		Describe("#MutatePodTopology", func() {
			It("it should correctly mutate required", func() {
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

				err := mutator.mutateNodeAffinity(pod, region)
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

				err := mutator.mutateNodeAffinity(pod, region)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions[0].Values[0]).To(Equal(fmt.Sprintf("%s-%s", region, "1")))
				Expect(pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions[0].Values[1]).To(Equal(fmt.Sprintf("%s-%s", region, "2")))
				Expect(pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions[0].Values[2]).To(Equal(fmt.Sprintf("%s-%s", region, "3")))
				Expect(pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions[1].Values[0]).To(Equal("bar"))
			})
		})
	})
})

func clientGet(result runtime.Object) interface{} {
	return func(ctx context.Context, key ctrclient.ObjectKey, obj runtime.Object, _ ...ctrclient.GetOption) error {
		switch obj.(type) {
		case *corev1.ConfigMap:
			*obj.(*corev1.ConfigMap) = *result.(*corev1.ConfigMap)
		case *v1alpha1.Cluster:
			*obj.(*v1alpha1.Cluster) = *result.(*v1alpha1.Cluster)
		}
		return nil
	}
}
