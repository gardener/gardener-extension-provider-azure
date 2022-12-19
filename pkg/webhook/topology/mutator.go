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

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var admissionNamespaceKey = struct{}{}

type handler struct {
	client  client.Client
	log     logr.Logger
	decoder *admission.Decoder
}

// New initializes a new topology handler that is responsible for adjusting the node affinity of pods.
// The LabelTopologyZone label that Azure CCM adds to nodes does not contain only the zone as it appears in Azure API
// calls but also the region like "$region-$zone". When only "$zone" is present for the LabelTopologyZone selector key
// this handler will adapt it to match the format that is used by the CCM labels.
func New(log logr.Logger) *handler {
	return &handler{
		log: log,
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

	// Process the resource
	newObj := newPod.DeepCopyObject().(client.Object)
	ctx = context.WithValue(ctx, admissionNamespaceKey, ar.Namespace)
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

func (h *handler) InjectClient(client client.Client) error {
	h.client = client
	return nil
}

func (h *handler) InjectDecoder(d *admission.Decoder) error {
	h.decoder = d
	return nil
}

func (h *handler) Mutate(ctx context.Context, new, old client.Object) error {
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

	for _, f := range []func(context.Context) (*string, error){
		h.getRegionFromShootInfo,
		h.getRegionFromCluster,
	} {
		region, err := f(ctx)
		if err != nil {
			return err
		}
		if region != nil && len(*region) > 0 {
			return h.mutateNodeAffinity(newPod, *region)
		}
	}

	h.log.Error(nil, "failed to mutate pod: seed region not found")
	return fmt.Errorf("failed to mutate pod: seed region not found")
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

// getRegionFromCluster retrieves the seed's region from the cluster object
func (h *handler) getRegionFromCluster(ctx context.Context) (*string, error) {
	h.log.Info("fetching region from Cluster object")
	ns, ok := ctx.Value(admissionNamespaceKey).(string)
	if !ok {
		return nil, nil
	}
	cluster, err := extensionscontroller.GetCluster(ctx, h.client, ns)
	if err != nil {
		return nil, fmt.Errorf("could not get cluster for namespace '%s': %w", ns, err)
	}
	if client.IgnoreNotFound(err) != nil {
		return nil, err
	} else if apierrors.IsNotFound(err) {
		h.log.Info("fetching Cluster object failed with not found error")
		return nil, nil
	}

	return &cluster.Seed.Spec.Provider.Region, nil
}

// getRegionFromShootInfo retrieves the seed's region if we reside in a ManagedSeed
func (h *handler) getRegionFromShootInfo(ctx context.Context) (*string, error) {
	h.log.Info("fetching region from shoot-info")
	cm := &corev1.ConfigMap{}
	if err := h.client.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: v1beta1constants.ConfigMapNameShootInfo}, cm); client.IgnoreNotFound(err) != nil {
		return nil, err
	} else if apierrors.IsNotFound(err) {
		return nil, nil
	}

	if region, ok := cm.Data["region"]; ok {
		return &region, nil
	}

	return nil, nil
}
