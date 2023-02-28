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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

type handler struct {
	log      logr.Logger
	region   string
	provider string
	decoder  *admission.Decoder
}

// New initializes a new topology handler that is responsible for adjusting the node affinity of pods.
// The LabelTopologyZone label that Azure CCM adds to nodes does not contain only the zone as it appears in Azure API
// calls but also the region like "$region-$zone". When only "$zone" is present for the LabelTopologyZone selector key
// this handler will adapt it to match the format that is used by the CCM labels.
func New(log logr.Logger, opts AddOptions) *handler {
	return &handler{
		log:      log,
		region:   opts.SeedRegion,
		provider: opts.SeedProvider,
	}
}

func (h *handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	ar := req.AdmissionRequest
	logger = h.log

	// Decode object
	var (
		newPod corev1.Pod
		obj    client.Object = &newPod
	)

	err := h.decoder.DecodeRaw(req.Object, &newPod)
	if err != nil {
		logger.Error(err, "Could not decode request", "request", ar)
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("could not decode request %v: %w", ar, err))
	}

	if len(req.OldObject.Raw) != 0 {
		return admission.ValidationResponse(true, "")
	}
	// skip mutation if the seed's region has not been provided
	if len(h.region) == 0 {
		return admission.ValidationResponse(true, "")
	}
	// This webhook is only useful if our seed is running on azure. Therefore, skip mutation if the seed's provider is
	// not of type Azure.
	if !strings.EqualFold(h.provider, azure.Type) {
		return admission.ValidationResponse(true, "")
	}

	// Process the resource
	newObj := newPod.DeepCopyObject().(client.Object)
	if err = h.Mutate(ctx, newObj, nil); err != nil {
		logger.Error(fmt.Errorf("could not process: %w", err), "Admission denied", "kind", ar.Kind.Kind, "namespace", obj.GetNamespace(), "name", obj.GetName())
		return admission.Errored(http.StatusUnprocessableEntity, err)
	}

	// Return a patch response if the resource should be changed
	if !equality.Semantic.DeepEqual(obj, newObj) {
		oldObjMarshaled, err := json.Marshal(obj)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		newObjMarshaled, err := json.Marshal(newObj)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}

		return admission.PatchResponseFromRaw(oldObjMarshaled, newObjMarshaled)
	}

	// Return a validation response if the resource should not be changed
	return admission.ValidationResponse(true, "")
}

func (h *handler) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

func (h *handler) Mutate(_ context.Context, new, old client.Object) error {
	// do not try to mutate pods that are getting deleted
	if new.GetDeletionTimestamp() != nil {
		return nil
	}

	newPod, ok := new.(*corev1.Pod)
	if !ok {
		return fmt.Errorf("object is not of type Pod")
	}

	// do not mutate on update/delete operations
	if old != nil {
		return nil
	}

	return h.mutateNodeAffinity(newPod, h.region)
}

func (h *handler) mutateNodeAffinity(pod *corev1.Pod, region string) error {
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
