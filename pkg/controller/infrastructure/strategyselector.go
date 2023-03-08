package infrastructure

import (
	"context"
	"errors"
	"fmt"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow/shared"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StrategySelector struct {
	Factory ReconcilerFactory
	Client  client.Client
}

func (s *StrategySelector) Reconcile(useFlow bool, ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig, cluster *extensions.Cluster) error {
	reconciler := s.Factory.Build(useFlow)
	status, err := reconciler.Reconcile(ctx, infra, cfg, cluster)
	if err != nil {
		return err
	}
	stateBytes, err := reconciler.GetState(ctx, status)
	if err != nil {
		return fmt.Errorf("failed to get infrastructure state: %w", err)
	}
	return patchProviderStatusAndState(ctx, infra, status, stateBytes, s.Client)
}

func (s *StrategySelector) ShouldDeleteWithFlow(status extensionsv1alpha1.InfrastructureStatus) (bool, error) {
	if status.State == nil {
		return false, errors.New("no state was ever written. Make sure to reconcile before trying to delete")
	}
	state, err := shared.NewPersistentStateFromJSON(status.State.Raw)
	if err != nil {
		return false, fmt.Errorf("failed to read infrastructure state: %w", err)
	}
	if state == nil { // did not recognize flow Kind
		return false, nil
	} else {
		return true, nil
	}
}
