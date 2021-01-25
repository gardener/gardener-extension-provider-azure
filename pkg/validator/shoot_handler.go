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

package validator

import (
	"context"
	"net/http"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"

	"github.com/gardener/gardener/extensions/pkg/util"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Shoot validates shoots
type Shoot struct {
	decoder runtime.Decoder
	Logger  logr.Logger
}

// Handle implements Handler.Handle
func (v *Shoot) Handle(ctx context.Context, req admission.Request) admission.Response {
	shoot := &core.Shoot{}
	if err := util.Decode(v.decoder, req.Object.Raw, shoot); err != nil {
		v.Logger.Error(err, "failed to decode shoot", "shoot", string(req.Object.Raw))
		return admission.Errored(http.StatusBadRequest, err)
	}

	if shoot.Spec.Provider.Type != azure.Type {
		return admission.Allowed("webhook not responsible for this provider")
	}

	switch req.Operation {
	case admissionv1.Create:
		if err := v.validateShootCreation(shoot); err != nil {
			v.Logger.Error(err, "denied request")
			return admission.Errored(http.StatusBadRequest, err)
		}
	case admissionv1.Update:
		oldShoot := &core.Shoot{}
		if err := util.Decode(v.decoder, req.OldObject.Raw, oldShoot); err != nil {
			v.Logger.Error(err, "failed to decode old shoot", "old shoot", string(req.OldObject.Raw))
			return admission.Errored(http.StatusBadRequest, err)
		}

		if err := v.validateShootUpdate(oldShoot, shoot); err != nil {
			v.Logger.Error(err, "denied request")
			return admission.Errored(http.StatusBadRequest, err)
		}
	default:
		v.Logger.Info("Webhook not responsible", "operation", req.Operation)
	}

	return admission.Allowed("validations succeeded")
}

// InjectScheme injects the scheme.
func (v *Shoot) InjectScheme(s *runtime.Scheme) error {
	v.decoder = serializer.NewCodecFactory(s).UniversalDecoder()
	return nil
}
