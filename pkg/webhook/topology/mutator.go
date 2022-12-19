//  Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package topology

import (
	"context"
	"fmt"
	"strings"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mutator struct {
	client client.Client
	log    logr.Logger
}

// New initializes a new topology mutator that is responsible for adjusting the node affinity of pods.
// The LabelTopologyZone label that Azure CCM adds to nodes does not contain only the zone as it appears in Azure API
// calls but also the region like "$region-$zone". When only "$zone" is present for the LabelTopologyZone selector key
// this mutator will adapt it to match the format that is used by the CCM labels.
func New(log logr.Logger) *mutator {
	return &mutator{
		log: log,
	}
}

func (m *mutator) InjectClient(client client.Client) error {
	m.client = client
	return nil
}

func (m *mutator) Mutate(ctx context.Context, new, old client.Object) error {
	// do not try to mutate pods that are getting deleted
	if new.GetDeletionTimestamp() != nil {
		return nil
	}

	// Check if this is a create or update operation and perform no-op for updates
	// Because the NodeAffinity of pods is an immutable field, we don't want to try and mutate it if the pod is already existing.
	// TODO(KA): Remove once there is support for specifying webhook verbs
	if old != nil {
		return nil
	}

	newPod, ok := new.(*corev1.Pod)
	if !ok {
		return fmt.Errorf("object is not of type Pod")
	}

	for _, f := range []func(context.Context, *corev1.Pod) (*string, error){
		m.getRegionFromShootInfo,
		m.getRegionFromCluster,
	} {
		region, err := f(ctx, newPod)
		if err != nil {
			return err
		}
		if region != nil && len(*region) > 0 {
			return m.mutateNodeAffinity(newPod, *region)
		}
	}

	m.log.Error(nil, "failed to mutate pod: seed region not found")
	return fmt.Errorf("failed to mutate pod: seed region not found")
}

func (m *mutator) mutateNodeAffinity(pod *corev1.Pod, region string) error {
	if pod.Spec.Affinity == nil {
		return nil
	}

	if pod.Spec.Affinity.NodeAffinity == nil {
		return nil
	}

	if req := pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution; req != nil {
		adaptNodeSelectorTermSlice(req.NodeSelectorTerms, region)
	}
	if pref := pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution; pref != nil {
		for p := range pref {
			adaptNodeSelectorTerm(&pref[p].Preference, region)
		}
	}

	return nil

}

func adaptNodeSelectorTermSlice(terms []corev1.NodeSelectorTerm, region string) {
	for termsIdx := range terms {
		adaptNodeSelectorTerm(&terms[termsIdx], region)
	}
}

func adaptNodeSelectorTerm(term *corev1.NodeSelectorTerm, region string) {
	for _, expr := range term.MatchExpressions {
		if expr.Key == corev1.LabelTopologyZone {
			for idx, val := range expr.Values {
				if !strings.HasPrefix(val, region) {
					expr.Values[idx] = fmt.Sprintf("%s-%s", region, val)
				}
			}
		}
	}
}

// getRegionFromCluster retrieves the seed's region from the cluster object
func (m *mutator) getRegionFromCluster(ctx context.Context, pod *corev1.Pod) (*string, error) {
	m.log.Info("fetching region from Cluster object")
	cluster, err := extensionscontroller.GetCluster(ctx, m.client, pod.GetNamespace())
	if err != nil {
		return nil, fmt.Errorf("could not get cluster for namespace '%s': %w", pod.GetNamespace(), err)
	}
	if client.IgnoreNotFound(err) != nil {
		return nil, err
	} else if apierrors.IsNotFound(err) {
		m.log.Info("fetching Cluster object failed with not found error")
		return nil, nil
	}

	return &cluster.Seed.Spec.Provider.Region, nil
}

// getRegionFromShootInfo retrieves the seed's region if we reside in a ManagedSeed
func (m *mutator) getRegionFromShootInfo(ctx context.Context, _ *corev1.Pod) (*string, error) {
	m.log.Info("fetching region from shoot-info")
	cm := &corev1.ConfigMap{}
	if err := m.client.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: constants.ConfigMapNameShootInfo}, cm); client.IgnoreNotFound(err) != nil {
		return nil, err
	} else if apierrors.IsNotFound(err) {
		m.log.Error(nil, "fetching shoot-info object failed with not found error")
		return nil, nil
	}

	if region, ok := cm.Data["region"]; ok {
		return &region, nil
	}

	m.log.Error(nil, "shoot-info configMap does not contain field \"region\"")
	return nil, nil
}
