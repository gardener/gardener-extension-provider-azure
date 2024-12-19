// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package namespace

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

// handler can mutate namespaces having the "high-availability-config.resources.gardener.cloud/zones" annotation. Because gardener uses azure zones differently that Azure controllers,
// there are installations where the zones in the seed and shoot spec are inconsistent ("zone" vs "region-zone" format). This webhook handler translates the latter format to the first to make it
// consistent between the different gardener resources.
type handler struct {
	log      logr.Logger
	region   string
	provider string
	decoder  admission.Decoder
}

// New initializes a new namespace handler.
// The LabelTopologyZone label that Azure CCM adds to nodes does not contain only the zone as it appears in Azure API
// calls but also the region like "$region-$zone". When only "$zone" is present for the LabelTopologyZone selector key
// this handler will adapt it to match the format that is used by the CCM labels.
func New(decoder admission.Decoder, log logr.Logger, opts AddOptions) *handler {
	return &handler{
		decoder:  decoder,
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
		newNamespace corev1.Namespace
		obj          client.Object = &newNamespace
	)

	err := h.decoder.DecodeRaw(req.Object, &newNamespace)
	if err != nil {
		logger.Error(err, "Could not decode request", "request", ar)
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("could not decode request %v: %w", ar, err))
	}

	// // skip mutation if the seed's region has not been provided
	if len(h.region) == 0 {
		return admission.ValidationResponse(true, "")
	}
	// This webhook is only useful if our seed is running on azure. Therefore, skip mutation if the seed's provider is
	// not of type Azure.
	if !strings.EqualFold(h.provider, azure.Type) {
		return admission.ValidationResponse(true, "")
	}

	// Process the resource
	newObj := newNamespace.DeepCopyObject().(client.Object)
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

func (h *handler) Mutate(_ context.Context, newObj, _ client.Object) error {
	// do not try to mutate if deletion timestamp is present.
	if newObj.GetDeletionTimestamp() != nil {
		return nil
	}

	annotations := newObj.GetAnnotations()
	if v, ok := annotations[v1alpha1.HighAvailabilityConfigFailureToleranceType]; !ok || v != string(v1beta1.FailureToleranceTypeZone) {
		return nil
	}

	var zonesRaw []string
	if zoneAnnotation, ok := annotations[v1alpha1.HighAvailabilityConfigZones]; !ok {
		return nil
	} else {
		zonesRaw = strings.Split(zoneAnnotation, ",")
	}

	for i := range zonesRaw {
		zonesRaw[i], _ = strings.CutPrefix(zonesRaw[i], fmt.Sprintf("%s-", h.region))
	}
	zoneSet := sets.New(zonesRaw...).UnsortedList()
	sort.Strings(zoneSet)
	annotations[v1alpha1.HighAvailabilityConfigZones] = strings.Join(zoneSet, ",")
	newObj.SetAnnotations(annotations)

	return nil
}
