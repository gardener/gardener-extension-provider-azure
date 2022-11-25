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
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

type mutator struct {
	client client.Client
	log    logr.Logger
}

// New initializes a new mutator.
func New(logger logr.Logger) *mutator {
	return &mutator{
		log: logger,
	}
}

func (m *mutator) InjectClient(client client.Client) error {
	m.client = client
	return nil
}

func (m *mutator) Mutate(ctx context.Context, new, _ client.Object) error {
	if new == nil {
		return nil
	}

	// do not try to mutate pods that are getting deleted
	if new.GetDeletionTimestamp() != nil {
		return nil
	}

	newPod, ok := new.(*v1.Pod)
	if !ok {
		return fmt.Errorf("could not mutate: object is not type Pod")
	}

	gctx := gardenContext.NewGardenContext(m.client, new)
	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		m.log.Error(err, "failed to mutate resource: %s/%s", new.GetNamespace(), new.GetName())
		return err
	}
	return m.mutate(ctx, newPod, cluster)
}

func (m *mutator) mutate(_ context.Context, pod *v1.Pod, cluster *extensions.Cluster) error {
	// probably can be omitted due to the namespace selector
	if cluster.Seed.Spec.Provider.Type != azure.Type {
		return nil
	}

	if pod.Spec.Affinity == nil {
		return nil
	}

	if pod.Spec.Affinity.NodeAffinity == nil {
		return nil
	}

	if req := pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution; req != nil {
		m.adaptNodeSelectorTermSlice(req.NodeSelectorTerms, cluster.Seed.Spec.Provider.Region)
	}
	if pref := pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution; pref != nil {
		for p := range pref {
			m.adaptNodeSelectorTerm(&pref[p].Preference, cluster.Seed.Spec.Provider.Region)
		}
	}

	return nil

}

func (m *mutator) adaptNodeSelectorTermSlice(terms []v1.NodeSelectorTerm, region string) {
	for termsIdx := range terms {
		m.adaptNodeSelectorTerm(&terms[termsIdx], region)
	}
}

func (m *mutator) adaptNodeSelectorTerm(term *v1.NodeSelectorTerm, region string) {
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
