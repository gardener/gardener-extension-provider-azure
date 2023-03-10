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
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener/extensions/pkg/util"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"k8s.io/apimachinery/pkg/runtime"
)

func decodeWorkerConfig(decoder runtime.Decoder, worker *runtime.RawExtension) (*azure.WorkerConfig, error) {
	if worker == nil {
		return nil, nil
	}

	workerConfig := &azure.WorkerConfig{}
	if err := util.Decode(decoder, worker.Raw, workerConfig); err != nil {
		return nil, err
	}

	return workerConfig, nil
}

func decodeControlPlaneConfig(decoder runtime.Decoder, cp *runtime.RawExtension) (*azure.ControlPlaneConfig, error) {
	controlPlaneConfig := &azure.ControlPlaneConfig{}
	if err := util.Decode(decoder, cp.Raw, controlPlaneConfig); err != nil {
		return nil, err
	}

	return controlPlaneConfig, nil
}

func decodeInfrastructureConfig(decoder runtime.Decoder, infra *runtime.RawExtension) (*azure.InfrastructureConfig, error) {
	infraConfig := &azure.InfrastructureConfig{}
	if err := util.Decode(decoder, infra.Raw, infraConfig); err != nil {
		return nil, err
	}

	return infraConfig, nil
}

func checkAndDecodeInfrastructureConfig(decoder runtime.Decoder, config *runtime.RawExtension, fldPath *field.Path) (*azure.InfrastructureConfig, error) {
	if config == nil {
		return nil, field.Required(fldPath, "InfrastructureConfig must be set for Azure shoots")
	}

	infraConfig, err := decodeInfrastructureConfig(decoder, config)
	if err != nil {
		return nil, field.Forbidden(infraConfigPath, "not allowed to configure an unsupported infrastructureConfig")
	}
	return infraConfig, nil
}

func decodeCloudProfileConfig(decoder runtime.Decoder, config *runtime.RawExtension) (*azure.CloudProfileConfig, error) {
	cloudProfileConfig := &azure.CloudProfileConfig{}
	if err := util.Decode(decoder, config.Raw, cloudProfileConfig); err != nil {
		return nil, err
	}
	return cloudProfileConfig, nil
}
