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

	gardenContext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/pkg/extensions"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mutator struct {
	client client.Client
}

// New initializes a new topology mutator that is responsible for adjusting the node affinity of pods.
// The LabelTopologyZone label that Azure CCM adds to nodes does not contain only the zone as it appears in Azure API
// calls but also the region like "$region-$zone". When only "$zone" is present for the LabelTopologyZone selector key
// this mutator will adapt it to match the format that is used by the CCM labels.
func New() *mutator {
	return &mutator{}
}

func (m *mutator) InjectClient(client client.Client) error {
	m.client = client
	return nil
}

func (m *mutator) Mutate(ctx context.Context, new, _ client.Object) error {
	// do not try to mutate pods that are getting deleted
	if new.GetDeletionTimestamp() != nil {
		return nil
	}

	newPod, ok := new.(*v1.Pod)
	if !ok {
		return fmt.Errorf("object is not of type Pod")
	}

	gctx := gardenContext.NewGardenContext(m.client, new)
	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return err
	}
	return m.mutateNodeAffinity(newPod, cluster)
}

func (m *mutator) mutateNodeAffinity(pod *v1.Pod, cluster *extensions.Cluster) error {
	if pod.Spec.Affinity == nil {
		return nil
	}

	if pod.Spec.Affinity.NodeAffinity == nil {
		return nil
	}

	if req := pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution; req != nil {
		adaptNodeSelectorTermSlice(req.NodeSelectorTerms, cluster.Seed.Spec.Provider.Region)
	}
	if pref := pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution; pref != nil {
		for p := range pref {
			adaptNodeSelectorTerm(&pref[p].Preference, cluster.Seed.Spec.Provider.Region)
		}
	}

	return nil

}

func adaptNodeSelectorTermSlice(terms []v1.NodeSelectorTerm, region string) {
	for termsIdx := range terms {
		adaptNodeSelectorTerm(&terms[termsIdx], region)
	}
}

func adaptNodeSelectorTerm(term *v1.NodeSelectorTerm, region string) {
	for _, expr := range term.MatchExpressions {
		if expr.Key == v1.LabelTopologyZone {
			for idx, val := range expr.Values {
				if !strings.HasPrefix(val, region) {
					expr.Values[idx] = fmt.Sprintf("%s-%s", region, val)
				}
			}
		}
	}
}
