// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"encoding/json"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// IsOverlayEnabled inspects a Shoot networking provider config and reports whether an overlay CNI
// is enabled. Absence of a provider config is treated as "overlay enabled" (the historical default
// for Calico/Cilium in Gardener). Mirrors the identical helper in provider-gcp so both providers
// derive the CCM route-controller flag from the same shoot-level signal.
func IsOverlayEnabled(network *gardencorev1beta1.Networking) (bool, error) {
	if network == nil || network.ProviderConfig == nil || len(network.ProviderConfig.Raw) == 0 {
		return true, nil
	}

	var networkConfig map[string]interface{}
	if err := json.Unmarshal(network.ProviderConfig.Raw, &networkConfig); err != nil {
		return false, err
	}

	if overlay, ok := networkConfig["overlay"].(map[string]interface{}); ok {
		if enabled, ok2 := overlay["enabled"].(bool); ok2 {
			return enabled, nil
		}
		return false, fmt.Errorf("overlay.enabled is not a boolean")
	}

	return true, nil
}
