// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"encoding/json"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
)

func mutateNetworkConfig(newObj, _ *extensionsv1alpha1.Network) error {
	extensionswebhook.LogMutation(logger, "Network", newObj.Namespace, newObj.Name)

	var (
		networkConfig map[string]interface{}
		//backendNone   = calicov1alpha1.None
		err error
	)

	if newObj.Spec.ProviderConfig != nil {
		networkConfig, err = decodeNetworkConfig(newObj.Spec.ProviderConfig)
		if err != nil {
			return err
		}
	} else {
		networkConfig = map[string]interface{}{"kind": ""}
	}

	networkConfig["backend"] = "none"
	modifiedJSON, err := json.Marshal(networkConfig)
	if err != nil {
		return err
	}

	newObj.Spec.ProviderConfig = &runtime.RawExtension{
		Raw: modifiedJSON,
	}

	return nil
}

func decodeNetworkConfig(network *runtime.RawExtension) (map[string]interface{}, error) {
	var networkConfig map[string]interface{}
	if network == nil || network.Raw == nil {
		return map[string]interface{}{}, nil
	}
	if err := json.Unmarshal(network.Raw, &networkConfig); err != nil {
		return nil, err
	}
	return networkConfig, nil
}
