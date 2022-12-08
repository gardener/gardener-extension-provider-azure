package infrastructure

import (
	"context"
	"fmt"

	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	"github.com/go-logr/logr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// NewFlowReconciler creates a new flow reconciler.
func NewFlowReconciler(ctx context.Context, a *actuator, infra *extensionsv1alpha1.Infrastructure, logger logr.Logger) (*infraflow.FlowReconciler, error) {
	client := a.Client()
	if client == nil {
		return nil, fmt.Errorf("infrastructure actuator has no client set")
	}
	auth, err := internal.GetClientAuthData(ctx, client, infra.Spec.SecretRef, false)
	if err != nil {
		return nil, err
	}
	factory, err := azureclient.NewAzureClientFactoryWithAuthAndClient(auth, client)
	if err != nil {
		return nil, err
	}
	reconciler := infraflow.NewFlowReconciler(factory, logger)
	return reconciler, nil
}
