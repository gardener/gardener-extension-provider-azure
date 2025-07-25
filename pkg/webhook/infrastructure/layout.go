// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	azuretypes "github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

type layoutMutator struct {
	logger logr.Logger
}

// newLayoutMutator returns a new Infrastructure layoutMutator
func newLayoutMutator(logger logr.Logger) extensionswebhook.Mutator {
	return &layoutMutator{
		logger: logger,
	}
}

// Mutate mutates the given object using the mutateFunc
func (m *layoutMutator) Mutate(ctx context.Context, newObj, oldObj client.Object) error {
	var (
		newInfra, oldInfra *extensionsv1alpha1.Infrastructure
		ok                 bool
	)

	if newObj == nil || oldObj == nil {
		return nil
	}

	if newObj.GetDeletionTimestamp() != nil {
		return nil
	}

	oldInfra, ok = oldObj.(*extensionsv1alpha1.Infrastructure)
	if !ok {
		return fmt.Errorf("could not mutate: object is not of type Infrastructure")
	}
	newInfra, ok = newObj.(*extensionsv1alpha1.Infrastructure)
	if !ok {
		return fmt.Errorf("could not mutate: object is not of type Infrastructure")
	}

	return mutate(ctx, m.logger, newInfra, oldInfra)
}

// mutate annotates the infrastructure object with additional information that are necessary during the reconciliation when migrating to a new network layout.
func mutate(_ context.Context, logger logr.Logger, newInfra, oldInfra *extensionsv1alpha1.Infrastructure) error {
	var (
		newProviderCfg, oldProviderCfg *azure.InfrastructureConfig
		oldProviderStatus              *azure.InfrastructureStatus
		err                            error
	)

	if newInfra.Spec.ProviderConfig == nil {
		return nil
	}

	newProviderCfg, err = helper.InfrastructureConfigFromInfrastructure(newInfra)
	if err != nil {
		return fmt.Errorf("could not mutate object: %v", err)
	}

	// if newInfra already contains the zone migration annotation, check if it is still necessary.
	// Otherwise, remove the annotation.
	if z, ok := newInfra.Annotations[azuretypes.NetworkLayoutZoneMigrationAnnotation]; ok {
		findMatchingZone := false
		for _, zone := range newProviderCfg.Networks.Zones {
			if helper.InfrastructureZoneToString(zone.Name) == z {
				findMatchingZone = true
				break
			}
		}

		if !findMatchingZone {
			delete(newInfra.Annotations, azuretypes.NetworkLayoutZoneMigrationAnnotation)
		}
		return nil
	}

	if oldInfra.Spec.ProviderConfig == nil {
		return nil
	}

	oldProviderCfg, err = helper.InfrastructureConfigFromInfrastructure(oldInfra)
	if err != nil {
		return fmt.Errorf("could not mutate object: %v", err)
	}

	// if the new configuration is using zones or it is not using multi-subnet layout it is not eligible for the mutation.
	if !newProviderCfg.Zoned || len(newProviderCfg.Networks.Zones) == 0 {
		return nil
	}

	// If the current operation is restore, then mutation might be necessary as the zone migration annotation is not preserved during control plane migration.
	if operation, ok := newInfra.Annotations[v1beta1constants.GardenerOperation]; ok && operation == v1beta1constants.GardenerOperationRestore {
		extensionswebhook.LogMutation(logger, newInfra.Kind, newInfra.Namespace, newInfra.Name)
		return addMigratedZoneAnnotationDuringRestore(newInfra)
	}

	// if the old configuration is not using zones or if it is already using a multi-subnet layout, no mutation is necessary.
	if !oldProviderCfg.Zoned || len(oldProviderCfg.Networks.Zones) > 0 {
		return nil
	}

	if oldInfra.Status.ProviderStatus != nil {
		oldProviderStatus, err = helper.InfrastructureStatusFromRaw(oldInfra.Status.ProviderStatus)
		if err != nil {
			return fmt.Errorf("could not mutate object: %v", err)
		}
	}

	// take care of clusters that have not been reconciliated for a long time (hibernated etc). In this case they may
	// not have the Layout field populated.
	if oldProviderStatus != nil &&
		oldProviderStatus.Networks.Layout != "" &&
		oldProviderStatus.Networks.Layout != azure.NetworkLayoutSingleSubnet {
		return nil
	}

	for _, z := range newProviderCfg.Networks.Zones {
		if z.CIDR == *oldProviderCfg.Networks.Workers {
			extensionswebhook.LogMutation(logger, newInfra.Kind, newInfra.Namespace, newInfra.Name)
			if newInfra.Annotations == nil {
				newInfra.Annotations = make(map[string]string)
			}
			newInfra.Annotations[azuretypes.NetworkLayoutZoneMigrationAnnotation] = helper.InfrastructureZoneToString(z.Name)
			return nil
		}
	}

	return nil
}

func addMigratedZoneAnnotationDuringRestore(infra *extensionsv1alpha1.Infrastructure) error {
	if infra.Status.State == nil || infra.Status.State.Raw == nil {
		return nil
	}

	fsOk, err := helper.HasFlowState(infra.Status)
	if err != nil {
		return err
	}

	if fsOk {
		infraState, err := helper.InfrastructureStateFromRaw(infra.Status.State)
		if err != nil {
			return err
		}

		if migratedZone, ok := infraState.Data[azuretypes.NetworkLayoutZoneMigrationAnnotation]; ok {
			metav1.SetMetaDataAnnotation(&infra.ObjectMeta, azuretypes.NetworkLayoutZoneMigrationAnnotation, migratedZone)
		}

		return nil
	}

	infraState := &infrastructure.InfrastructureState{}
	if err := json.Unmarshal(infra.Status.State.Raw, infraState); err != nil {
		return err
	}

	infrastructureStatus, err := helper.InfrastructureStatusFromRaw(infraState.SavedProviderStatus)
	if err != nil {
		return err
	}

	for _, subnet := range infrastructureStatus.Networks.Subnets {
		if subnet.Migrated && subnet.Zone != nil {
			metav1.SetMetaDataAnnotation(&infra.ObjectMeta, azuretypes.NetworkLayoutZoneMigrationAnnotation, *subnet.Zone)
			break
		}
	}
	return nil
}
