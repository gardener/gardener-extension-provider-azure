// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"github.com/gardener/gardener/extensions/pkg/util"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
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
