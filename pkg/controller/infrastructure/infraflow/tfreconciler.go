package infraflow

import (
	"context"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

type TfReconciler struct {
	tf TerraformAdapter
}

func NewTfReconciler(infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig, cluster *controller.Cluster) (*TfReconciler, error) {
	tfAdapter, err := NewTerraformAdapter(infra, cfg, cluster)
	return &TfReconciler{tfAdapter}, err
}

func (f TfReconciler) Vnet(ctx context.Context, vnetClient client.Vnet) error {
	return ReconcileVnetFromTf(ctx, f.tf, vnetClient)
}
