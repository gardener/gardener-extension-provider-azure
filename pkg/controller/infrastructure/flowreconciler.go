package infrastructure

import (
	"context"
	"fmt"
	"strings"

	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	"github.com/go-logr/logr"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// ShouldUseFlow returns true if the new flow reconciler should be used for the reconciliation.
func ShouldUseFlow(infrastructure *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) bool {
	return (infrastructure.Annotations != nil && strings.EqualFold(infrastructure.Annotations[AnnotationKeyUseFlow], "true")) ||
		(cluster.Shoot != nil && cluster.Shoot.Annotations != nil && strings.EqualFold(cluster.Shoot.Annotations[AnnotationKeyUseFlow], "true"))
}

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
