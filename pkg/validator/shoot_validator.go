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
	"reflect"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	azurevalidation "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"

	"github.com/gardener/gardener/pkg/apis/core"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var (
	specPath        = field.NewPath("spec")
	nwPath          = specPath.Child("networking")
	providerPath    = specPath.Child("provider")
	infraConfigPath = providerPath.Child("infrastructureConfig")
	cpConfigPath    = providerPath.Child("controlPlaneConfig")
	workersPath     = providerPath.Child("workers")
)

func (v *Shoot) validateShoot(shoot *core.Shoot, infraConfig *azure.InfrastructureConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	// ControlPlaneConfig
	if shoot.Spec.Provider.ControlPlaneConfig != nil {
		if _, err := decodeControlPlaneConfig(v.decoder, shoot.Spec.Provider.ControlPlaneConfig); err != nil {
			allErrs = append(allErrs, field.Forbidden(cpConfigPath, "not allowed to configure an unsupported controlPlaneConfig"))
		}
	}

	// Network validation
	allErrs = append(allErrs, azurevalidation.ValidateNetworking(shoot.Spec.Networking, nwPath)...)

	// Provider validation
	allErrs = append(allErrs, azurevalidation.ValidateInfrastructureConfig(infraConfig, shoot.Spec.Networking.Nodes, shoot.Spec.Networking.Pods, shoot.Spec.Networking.Services, infraConfigPath)...)

	// Shoot workers
	allErrs = append(allErrs, azurevalidation.ValidateWorkers(shoot.Spec.Provider.Workers, infraConfig.Zoned, workersPath)...)

	return allErrs
}

func (v *Shoot) validateShootUpdate(oldShoot, shoot *core.Shoot) error {
	// InfrastructureConfig update
	infraConfig, err := checkAndDecodeInfrastructureConfig(v.decoder, shoot.Spec.Provider.InfrastructureConfig, infraConfigPath)
	if err != nil {
		return err
	}

	oldInfraConfig, err := checkAndDecodeInfrastructureConfig(v.decoder, shoot.Spec.Provider.InfrastructureConfig, infraConfigPath)
	if err != nil {
		return err
	}

	allErrs := field.ErrorList{}

	if !reflect.DeepEqual(oldInfraConfig, infraConfig) {
		allErrs = append(allErrs, azurevalidation.ValidateInfrastructureConfigUpdate(oldInfraConfig, infraConfig, infraConfigPath)...)
	}

	allErrs = append(allErrs, azurevalidation.ValidateWorkersUpdate(oldShoot.Spec.Provider.Workers, shoot.Spec.Provider.Workers, workersPath)...)

	allErrs = append(allErrs, v.validateShoot(shoot, infraConfig)...)

	return allErrs.ToAggregate()
}

func (v *Shoot) validateShootCreation(shoot *core.Shoot) error {
	infraConfig, err := checkAndDecodeInfrastructureConfig(v.decoder, shoot.Spec.Provider.InfrastructureConfig, infraConfigPath)
	if err != nil {
		return err
	}

	return v.validateShoot(shoot, infraConfig).ToAggregate()
}
