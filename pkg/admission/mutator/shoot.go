// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/features"
)

// NewShootMutator returns a new instance of a shoot mutator.
func NewShootMutator(mgr manager.Manager) extensionswebhook.Mutator {
	return &shoot{
		decoder: serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
	}
}

type shoot struct {
	decoder runtime.Decoder
}

const (
	overlayKey = "overlay"
	enabledKey = "enabled"
)

// Mutate mutates the given shoot object.
func (s *shoot) Mutate(_ context.Context, newObj, oldObj client.Object) error {
	shoot, ok := newObj.(*gardencorev1beta1.Shoot)
	if !ok {
		return fmt.Errorf("wrong object type %T", newObj)
	}

	// skip validation if it's a workerless Shoot
	if gardencorev1beta1helper.IsWorkerless(shoot) {
		return nil
	}

	if shoot.Spec.Networking != nil && shoot.Spec.Networking.Type != nil && *shoot.Spec.Networking.Type != "cilium" {
		return nil
	}

	// Skip if shoot is in restore or migration phase
	if wasShootRescheduledToNewSeed(shoot) {
		return nil
	}

	var oldShoot *gardencorev1beta1.Shoot
	if oldObj != nil {
		oldShoot, ok = oldObj.(*gardencorev1beta1.Shoot)
		if !ok {
			return fmt.Errorf("wrong object type %T", oldObj)
		}
	}

	if oldShoot != nil && isShootInMigrationOrRestorePhase(shoot) {
		return nil
	}

	// Skip if specs are matching
	if oldShoot != nil && reflect.DeepEqual(shoot.Spec, oldShoot.Spec) {
		return nil
	}

	// Skip if shoot is in deletion phase
	if shoot.DeletionTimestamp != nil || oldShoot != nil && oldShoot.DeletionTimestamp != nil {
		return nil
	}

	err := s.mutateNetworkConfig(shoot, oldShoot)
	if err != nil {
		return err
	}

	if features.ExtensionFeatureGate.Enabled(features.ForceNatGateway) {
		err = s.mutateInfrastructureNatConfig(shoot, oldShoot)
		if err != nil {
			return err
		}
	}

	// Disable TCP to upstream DNS queries by default on Azure. DNS over TCP may cause performance issues on larger clusters.
	if shoot.Spec.SystemComponents != nil {
		if shoot.Spec.SystemComponents.NodeLocalDNS != nil {
			if shoot.Spec.SystemComponents.NodeLocalDNS.Enabled {
				if shoot.Spec.SystemComponents.NodeLocalDNS.ForceTCPToUpstreamDNS == nil {
					shoot.Spec.SystemComponents.NodeLocalDNS.ForceTCPToUpstreamDNS = ptr.To(false)
				}
			}
		}
	}

	return nil
}

func (s *shoot) mutateInfrastructureNatConfig(shoot, oldShoot *gardencorev1beta1.Shoot) error {
	if shoot.Spec.Provider.InfrastructureConfig == nil {
		return nil
	}

	infraConfig := &azure.InfrastructureConfig{}
	if _, _, err := s.decoder.Decode(shoot.Spec.Provider.InfrastructureConfig.Raw, nil, infraConfig); err != nil {
		return fmt.Errorf("failed to decode InfrastructureConfig: %w", err)
	}
	nat := infraConfig.Networks.NatGateway
	zones := infraConfig.Networks.Zones

	// force enable NAT-Gateway for new shoots only
	if oldShoot == nil {
		// Case 1: Non-zoned setup → enable NAT-Gateway if not explicitly set
		if len(zones) == 0 && nat == nil {
			infraConfig.Networks.NatGateway = &azure.NatGatewayConfig{Enabled: true}
			infraConfig.Zoned = true // required if NAT-Gateway is enabled
		}

		// Case 2: Zoned setup → enable NAT-Gateway per zone if not explicitly set
		for i := range zones {
			if zones[i].NatGateway == nil {
				zones[i].NatGateway = &azure.ZonedNatGatewayConfig{Enabled: true}
			}
		}
	}

	// prevent unsetting the NAT-Gateway if it was explicitly set before - to prevent unwanted configuration
	if oldShoot != nil && oldShoot.Spec.Provider.InfrastructureConfig != nil {
		oldInfraConfig := &azure.InfrastructureConfig{}
		if _, _, err := s.decoder.Decode(oldShoot.Spec.Provider.InfrastructureConfig.Raw, nil, oldInfraConfig); err != nil {
			return fmt.Errorf("failed to decode InfrastructureConfig: %w", err)
		}
		oldNat := oldInfraConfig.Networks.NatGateway
		oldZones := oldInfraConfig.Networks.Zones

		// do nothing if switching to multi-zone setup
		if len(oldZones) > 0 && len(zones) == 0 {
			return nil
		}

		// do nothing if switching to non-multi-zone setup
		if len(zones) > 0 && len(oldZones) == 0 {
			return nil
		}

		// prevent unsetting the NAT-Gateway for non-multi-zone shoots
		if oldNat != nil && nat == nil {
			infraConfig.Networks.NatGateway = oldNat
		}

		// prevent unsetting the NAT-Gateway for multi-zone shoots
		if len(oldZones) > 0 && len(zones) > 0 {
			for i := range zones {
				if oldZones[i].NatGateway != nil && zones[i].NatGateway == nil {
					zones[i].NatGateway = oldZones[i].NatGateway
				}
			}
		}
	}

	modifiedJSON, err := json.Marshal(infraConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal modified InfrastructureConfig: %w", err)
	}
	shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: modifiedJSON}
	return nil
}

func (s *shoot) mutateNetworkConfig(shoot, oldShoot *gardencorev1beta1.Shoot) error {
	if shoot.Spec.Networking == nil {
		return nil
	}

	networkConfig, err := s.decodeNetworkConfig(shoot.Spec.Networking.ProviderConfig)
	if err != nil {
		return err
	}

	if oldShoot == nil && networkConfig[overlayKey] == nil {
		networkConfig[overlayKey] = map[string]interface{}{enabledKey: false}
	}

	if oldShoot != nil && networkConfig[overlayKey] == nil {
		oldNetworkConfig, err := s.decodeNetworkConfig(oldShoot.Spec.Networking.ProviderConfig)
		if err != nil {
			return err
		}

		if oldNetworkConfig[overlayKey] != nil {
			networkConfig[overlayKey] = oldNetworkConfig[overlayKey]
		}
	}

	modifiedJSON, err := json.Marshal(networkConfig)
	if err != nil {
		return err
	}
	shoot.Spec.Networking.ProviderConfig = &runtime.RawExtension{
		Raw: modifiedJSON,
	}
	return nil
}

func (s *shoot) decodeNetworkConfig(network *runtime.RawExtension) (map[string]interface{}, error) {
	var networkConfig map[string]interface{}
	if network == nil || network.Raw == nil {
		return map[string]interface{}{}, nil
	}
	if err := json.Unmarshal(network.Raw, &networkConfig); err != nil {
		return nil, err
	}
	return networkConfig, nil
}

// wasShootRescheduledToNewSeed returns true if the shoot.Spec.SeedName has been changed, but the migration operation has not started yet.
func wasShootRescheduledToNewSeed(shoot *gardencorev1beta1.Shoot) bool {
	return shoot.Status.LastOperation != nil &&
		shoot.Status.LastOperation.Type != gardencorev1beta1.LastOperationTypeMigrate &&
		shoot.Spec.SeedName != nil &&
		shoot.Status.SeedName != nil &&
		*shoot.Spec.SeedName != *shoot.Status.SeedName
}

// isShootInMigrationOrRestorePhase returns true if the shoot is currently being migrated or restored.
func isShootInMigrationOrRestorePhase(shoot *gardencorev1beta1.Shoot) bool {
	return shoot.Status.LastOperation != nil &&
		(shoot.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeRestore &&
			shoot.Status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded ||
			shoot.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeMigrate)
}
