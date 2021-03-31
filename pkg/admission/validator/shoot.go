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
	"fmt"
	"reflect"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	azurevalidation "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"
)

var (
	metaDataPath    = field.NewPath("metadata")
	specPath        = field.NewPath("spec")
	nwPath          = specPath.Child("networking")
	providerPath    = specPath.Child("provider")
	infraConfigPath = providerPath.Child("infrastructureConfig")
	cpConfigPath    = providerPath.Child("controlPlaneConfig")
	workersPath     = providerPath.Child("workers")
)

// shoot validates shoots
type shoot struct {
	decoder        runtime.Decoder
	lenientDecoder runtime.Decoder
}

// NewShootValidator returns a new instance of a shoot validator.
func NewShootValidator() extensionswebhook.Validator {
	return &shoot{}
}

// InjectScheme injects the given scheme into the validator.
func (s *shoot) InjectScheme(scheme *runtime.Scheme) error {
	s.decoder = serializer.NewCodecFactory(scheme, serializer.EnableStrict).UniversalDecoder()
	s.lenientDecoder = serializer.NewCodecFactory(scheme).UniversalDecoder()
	return nil
}

// Validate validates the given shoot object
func (s *shoot) Validate(ctx context.Context, new, old client.Object) error {
	shoot, ok := new.(*core.Shoot)
	if !ok {
		return fmt.Errorf("wrong object type %T", new)
	}

	if old != nil {
		oldShoot, ok := old.(*core.Shoot)
		if !ok {
			return fmt.Errorf("wrong object type %T for old object", old)
		}
		return s.validateUpdate(oldShoot, shoot)
	}

	return s.validateCreation(shoot)
}

func (s *shoot) validateCreation(shoot *core.Shoot) error {
	infraConfig, err := checkAndDecodeInfrastructureConfig(s.decoder, shoot.Spec.Provider.InfrastructureConfig, infraConfigPath)
	if err != nil {
		return err
	}

	return s.validateShoot(shoot, infraConfig).ToAggregate()
}

func (s *shoot) validateShoot(shoot *core.Shoot, infraConfig *azure.InfrastructureConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	// ControlPlaneConfig
	if shoot.Spec.Provider.ControlPlaneConfig != nil {
		if _, err := decodeControlPlaneConfig(s.decoder, shoot.Spec.Provider.ControlPlaneConfig); err != nil {
			allErrs = append(allErrs, field.Forbidden(cpConfigPath, "not allowed to configure an unsupported controlPlaneConfig"))
		}
	}

	// Network validation
	allErrs = append(allErrs, azurevalidation.ValidateNetworking(shoot.Spec.Networking, nwPath)...)

	// Provider validation
	allErrs = append(allErrs, azurevalidation.ValidateInfrastructureConfig(infraConfig, shoot.Spec.Networking.Nodes, shoot.Spec.Networking.Pods, shoot.Spec.Networking.Services, helper.HasShootVmoAlphaAnnotation(shoot.Annotations), infraConfigPath)...)

	// Shoot workers
	allErrs = append(allErrs, azurevalidation.ValidateWorkers(shoot.Spec.Provider.Workers, infraConfig.Zoned, workersPath)...)

	return allErrs
}

func (s *shoot) validateUpdate(oldShoot, shoot *core.Shoot) error {
	// Decode the new infrastructure config.
	if shoot.Spec.Provider.InfrastructureConfig == nil {
		return field.Required(infraConfigPath, "InfrastructureConfig must be set for Azure shoots")
	}
	infraConfig, err := checkAndDecodeInfrastructureConfig(s.decoder, shoot.Spec.Provider.InfrastructureConfig, infraConfigPath)
	if err != nil {
		return err
	}

	// Decode the old infrastructure config.
	if oldShoot.Spec.Provider.InfrastructureConfig == nil {
		return field.InternalError(infraConfigPath, fmt.Errorf("InfrastructureConfig is not available on old shoot"))
	}
	oldInfraConfig, err := checkAndDecodeInfrastructureConfig(s.lenientDecoder, oldShoot.Spec.Provider.InfrastructureConfig, infraConfigPath)
	if err != nil {
		return err
	}

	var allErrs = field.ErrorList{}
	if !reflect.DeepEqual(oldInfraConfig, infraConfig) {
		allErrs = append(allErrs, azurevalidation.ValidateInfrastructureConfigUpdate(oldInfraConfig, infraConfig, metaDataPath)...)
	}

	allErrs = append(allErrs, azurevalidation.ValidateVmoConfigUpdate(helper.HasShootVmoAlphaAnnotation(oldShoot.Annotations), helper.HasShootVmoAlphaAnnotation(shoot.Annotations), metaDataPath)...)
	allErrs = append(allErrs, azurevalidation.ValidateWorkersUpdate(oldShoot.Spec.Provider.Workers, shoot.Spec.Provider.Workers, workersPath)...)

	allErrs = append(allErrs, s.validateShoot(shoot, infraConfig)...)

	return allErrs.ToAggregate()
}
