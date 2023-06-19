// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// GetNetworkPolicyMeta returns the network policy object with filled metadata.
func GetNetworkPolicyMeta(shootNamespace, extensionName string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{ObjectMeta: kubernetesutils.ObjectMeta(shootNamespace, "gardener-extension-"+extensionName)}
}

// EnsureEgressNetworkPolicy ensures that the required egress network policy is installed that allows the kube-apiserver
// running in the given shoot namespace to talk to the extension webhook.
// Deprecated: This function is deprecated and will be removed after Gardener v1.80 has been released. Extensions should
// make sure that they can be accessed via the 'all-webhook-targets' alias.
// TODO(rfranzke): Drop this after v1.80 has been released.
func EnsureEgressNetworkPolicy(ctx context.Context, c client.Client, shootNamespace, extensionNamespace, extensionName string, port int) error {
	networkPolicy := GetNetworkPolicyMeta(shootNamespace, extensionName)
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, c, networkPolicy, func() error {
		networkPolicy.Spec = networkingv1.NetworkPolicySpec{
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Port:     utils.IntStrPtrFromInt(port),
							Protocol: utils.ProtocolPtr(corev1.ProtocolTCP),
						},
					},
					To: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									corev1.LabelMetadataName: extensionNamespace,
								},
							},
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app.kubernetes.io/name": "gardener-extension-" + extensionName,
								},
							},
						},
					},
				},
			},
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
					v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer,
				},
			},
		}
		return nil
	})
	return err
}

// EnsureIngressNetworkPolicy ensures that the required ingress network policy is installed that allows the
// kube-apiservers of shoot namespaces to talk to the extension webhook.
// Deprecated: This function is deprecated and will be removed after Gardener v1.80 has been released. Extensions should
// make sure that they can be accessed via the 'all-webhook-targets' alias.
// TODO(rfranzke): Drop this after v1.80 has been released.
func EnsureIngressNetworkPolicy(ctx context.Context, c client.Client, extensionNamespace, extensionName string, port int) error {
	networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: extensionNamespace, Name: "ingress-from-all-shoots-kube-apiserver"}}
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, c, networkPolicy, func() error {
		networkPolicy.Spec = networkingv1.NetworkPolicySpec{
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Port:     utils.IntStrPtrFromInt(port),
							Protocol: utils.ProtocolPtr(corev1.ProtocolTCP),
						},
					},
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot,
								},
							},
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
									v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer,
								},
							},
						},
					},
				},
			},
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": "gardener-extension-" + extensionName,
				},
			},
		}
		return nil
	})
	return err
}
