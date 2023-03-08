package infrastructure

import (
	"context"
	"errors"
	"fmt"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
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
	state, err := reconciler.GetState(ctx, status)
	if err != nil {
		return fmt.Errorf("failed to get infrastructure state: %w", err)
	}
	return patchProviderStatusAndState(ctx, infra, status, state, s.Client)
}

func (s *StrategySelector) DeleteUseFlow(status extensionsv1alpha1.InfrastructureStatus) (bool, error) {
	if status.State == nil {
		return false, errors.New("no state was ever written. Make sure to reconcile before trying to delete")
	}
	if string(status.State.Raw) == "{}" { // flow has empty state
		return true, nil
	} else {
		return false, nil
	}
}
