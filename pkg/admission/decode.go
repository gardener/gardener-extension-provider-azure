// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package admission

import (
	"github.com/gardener/gardener/extensions/pkg/util"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
)

// DecodeWorkloadIdentityConfig decodes the `WorkloadIdentityConfig` from the given `RawExtension`.
func DecodeWorkloadIdentityConfig(decoder runtime.Decoder, config *runtime.RawExtension) (*azure.WorkloadIdentityConfig, error) {
	workloadIdentityConfig := &azure.WorkloadIdentityConfig{}
	if err := util.Decode(decoder, config.Raw, workloadIdentityConfig); err != nil {
		return nil, err
	}

	return workloadIdentityConfig, nil
}
