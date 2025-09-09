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
	"github.com/gardener/gardener/pkg/utils"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
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

	err = s.mutateInfrastructureNatConfig(shoot, oldShoot)
	if err != nil {
		return err
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

// mutateInfrastructureNatConfig mutates the InfrastructureConfig to enable NAT-Gateway
// preserves nat config if it was already set
func (s *shoot) mutateInfrastructureNatConfig(shoot, oldShoot *gardencorev1beta1.Shoot) error {
	if shoot.Spec.Provider.InfrastructureConfig == nil || shoot.Spec.Provider.InfrastructureConfig.Raw == nil {
		return nil
	}

	fmt.Printf("\nFOOOOOOO %v\n", string(shoot.Spec.Provider.InfrastructureConfig.Raw))
	infraConfig := v1alpha1.InfrastructureConfig{}
	if _, _, err := s.decoder.Decode(shoot.Spec.Provider.InfrastructureConfig.Raw, nil, &infraConfig); err != nil {
		return fmt.Errorf("failed to decode InfrastructureConfig: %w", err)
	}

	if !shouldMutateNatGateway(infraConfig, oldShoot) {
		return nil
	}

	// add annotation for new shoot OR preserve annotation if it was already set
	shoot.Annotations = utils.MergeStringMaps(shoot.Annotations, map[string]string{
		azure.ShootMutateNatConfig: "true",
	})

	nat := infraConfig.Networks.NatGateway
	zones := infraConfig.Networks.Zones

	// Case 1: Non-zoned setup → enable NAT-Gateway if not explicitly set
	if len(zones) == 0 && nat == nil {
		infraConfig.Networks.NatGateway = &v1alpha1.NatGatewayConfig{Enabled: true}
	}

	// Case 2: Zoned setup → enable NAT-Gateway per zone if not explicitly set
	for i := range zones {
		if zones[i].NatGateway == nil {
			zones[i].NatGateway = &v1alpha1.ZonedNatGatewayConfig{Enabled: true}
		}
	}

	// az := &v1alpha1.InfrastructureConfig{
	// 	TypeMeta: metav1.TypeMeta{
	// 		APIVersion: v1alpha1.SchemeGroupVersion.String(),
	// 		Kind:       "InfrastructureConfig",
	// 	},
	// }
	// shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Object: az}

	modifiedJSON, err := json.Marshal(infraConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal modified InfrastructureConfig: %w", err)
	}
	shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{Raw: modifiedJSON}

	// fmt.Printf("FOOOOOOO %v\n", string(shoot.Spec.Provider.InfrastructureConfig.Raw))
	return nil
}

// shouldMutateNatGateway returns true if ForceNatGateway is enabled and either it's a
// new shoot or the old shoot has the annotation to mutate nat config.
func shouldMutateNatGateway(newInfraConfig v1alpha1.InfrastructureConfig, oldShoot *gardencorev1beta1.Shoot) bool {
	if !features.ExtensionFeatureGate.Enabled(features.ForceNatGateway) {
		return false
	}
	// don't mutate shoots with existing VNet
	if newInfraConfig.Networks.VNet.Name != nil && newInfraConfig.Networks.VNet.ResourceGroup != nil {
		return false
	}
	return oldShoot == nil ||
		(oldShoot.Annotations != nil && oldShoot.Annotations[azure.ShootMutateNatConfig] == "true")
}

func (s *shoot) mutateNetworkConfig(shoot, oldShoot *gardencorev1beta1.Shoot) error {
	if shoot.Spec.Networking == nil {
		return nil
	}

	if shoot.Spec.Networking != nil && shoot.Spec.Networking.Type != nil && *shoot.Spec.Networking.Type != "cilium" {
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
