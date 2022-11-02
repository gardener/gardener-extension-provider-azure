package infraflow

import (
	"context"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

type FlowReconciler struct {
	Client client.ResourceGroup
}

func (f FlowReconciler) Reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig) error {
	if cfg.ResourceGroup != nil {
		return f.Client.CreateOrUpdate(ctx, cfg.ResourceGroup.Name, infra.Spec.Region)
	}
	return nil
}
