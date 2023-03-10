package infrastructure

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow/shared"
)

type StrategySelector struct {
	Factory ReconcilerFactory
	Client  client.Client
}

type ReconcilerFactoryImpl struct {
	ctx              context.Context
	log              logr.Logger
	a                *actuator
	infra            *extensionsv1alpha1.Infrastructure
	stateInitializer terraformer.StateConfigMapInitializer
}

func (f ReconcilerFactoryImpl) Build(useFlow bool) (Reconciler, error) {
	if useFlow {
		if err := cleanupTerraform(f.ctx, f.log, f.a, f.infra); err != nil {
			return nil, fmt.Errorf("failed to cleanup terraform resources: %w", err)
		}
		reconciler, err := NewFlowReconciler(f.ctx, f.a, f.infra, f.log)
		if err != nil {
			return nil, err
		}
		return reconciler, nil
	} else {
		reconciler, err := NewTerraformReconciler(f.a, f.log, f.stateInitializer)
		if err != nil {
			return nil, fmt.Errorf("failed to init terraform reconciler: %w", err)
		}
		return reconciler, nil
	}
}

func (s *StrategySelector) ShouldReconcileWithFlow(infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensions.Cluster) (bool, error) {
	hasState, err := s.hasFlowState(infrastructure.Status)
	if err != nil {
		return false, err
	}
	return hasState || HasFlowAnnotation(infrastructure, cluster), nil
}

func (s *StrategySelector) Reconcile(useFlow bool, ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig, cluster *extensions.Cluster) error {
	reconciler, err := s.Factory.Build(useFlow)
	if err != nil {
		return err
	}
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
	return s.hasFlowState(status)
}

func (s *StrategySelector) hasFlowState(status extensionsv1alpha1.InfrastructureStatus) (bool, error) {
	if status.State == nil {
		return false, nil
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
