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
// is enabled. When overlay.enabled is not specified the default depends on the CNI: Calico on Azure
// never runs in overlay mode (the admission webhook rejects overlay.enabled: true), so an absent key
// keeps the CCM route controller on; Cilium's historical default is overlay-on. Mirrors the helper
// in provider-gcp so both providers derive the CCM route-controller flag from the same shoot-level
// signal.
func IsOverlayEnabled(network *gardencorev1beta1.Networking) (bool, error) {
	if network == nil {
		return true, nil
	}

	if network.ProviderConfig == nil || len(network.ProviderConfig.Raw) == 0 {
		return overlayDefaultEnabled(network), nil
	}

	var networkConfig map[string]interface{}
	if err := json.Unmarshal(network.ProviderConfig.Raw, &networkConfig); err != nil {
		return false, err
	}

	if overlay, ok := networkConfig["overlay"].(map[string]interface{}); ok {
		enabledValue, present := overlay["enabled"]
		if !present {
			return overlayDefaultEnabled(network), nil
		}
		enabled, isBool := enabledValue.(bool)
		if !isBool {
			return false, fmt.Errorf("overlay.enabled is not a boolean")
		}
		return enabled, nil
	}

	return overlayDefaultEnabled(network), nil
}

// overlayDefaultEnabled returns the assumed overlay state when overlay.enabled is not set in the
// shoot's networking provider config. Calico on Azure is always non-overlay, so its route controller
// must stay enabled; every other CNI keeps the historical overlay-on default.
func overlayDefaultEnabled(network *gardencorev1beta1.Networking) bool {
	return network.Type == nil || *network.Type != "calico"
}
