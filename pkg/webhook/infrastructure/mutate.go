package infrastructure

import (
	"context"
	"fmt"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	azuretypes "github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MutateFunc is a function that can perform a mutation on Infrastructure objects.
type MutateFunc func(ctx context.Context, logger logr.Logger, new, old *extensionsv1alpha1.Infrastructure) error

type mutator struct {
	logger     logr.Logger
	mutateFunc MutateFunc
}

// New returns a new Infrastructure mutator that uses mutateFunc to perform the mutation.
func New(logger logr.Logger, mutateFunc MutateFunc) extensionswebhook.Mutator {
	return &mutator{
		logger:     logger,
		mutateFunc: mutateFunc,
	}
}

// Mutate mutates the given object using the mutateFunc
func (m *mutator) Mutate(ctx context.Context, new, old client.Object) error {
	var (
		newInfra, oldInfra *extensionsv1alpha1.Infrastructure
		ok                 bool
	)

	if new == nil || old == nil {
		return nil
	}

	if new.GetDeletionTimestamp() != nil {
		return nil
	}

	oldInfra, ok = old.(*extensionsv1alpha1.Infrastructure)
	if !ok {
		return fmt.Errorf("could not mutate: object is not of type Infrastructure")
	}
	newInfra, ok = new.(*extensionsv1alpha1.Infrastructure)
	if !ok {
		return fmt.Errorf("could not mutate: object is not of type Infrastructure")
	}

	return m.mutateFunc(ctx, m.logger, newInfra, oldInfra)
}

// NetworkLayoutMigrationMutate annotates the infrastructure object with additonal information that are necessary during the reconciliation when migrating to a new network layout.
func NetworkLayoutMigrationMutate(ctx context.Context, logger logr.Logger, newInfra, oldInfra *extensionsv1alpha1.Infrastructure) error {
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

	// if the new one has the label then check if you need to remove it
	if z, ok := newInfra.Annotations[azuretypes.NetworkLayoutZoneMigrationAnnotation]; ok {
		findMatchingZone := false
		for _, zone := range newProviderCfg.Networks.Zones {
			if helper.InfrastructureZoneToString(zone.Name) == z {
				findMatchingZone = true
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

	if oldInfra.Status.ProviderStatus != nil {
		oldProviderStatus, err = helper.InfrastructureStatusFromInfrastructure(oldInfra)
		if err != nil {
			return fmt.Errorf("could not mutate object: %v", err)
		}
	}

	// if the new configuration is not zoned or is not using multiple-subnet layout it is not eligible for the mutation.
	if !newProviderCfg.Zoned || len(newProviderCfg.Networks.Zones) == 0 {
		return nil
	}

	// if the old configuration is not zoned and it is already using multiple-subnet layout, no mutation is necessary.
	if !oldProviderCfg.Zoned || len(oldProviderCfg.Networks.Zones) > 0 {
		return nil
	}

	// take care of clusters that have not been reconciliated for a long time (hibernated etc). In this case they may
	// not have the Layout field populated, so instead rely on the old spec. If the cluster was not a zoned one skip it.
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
