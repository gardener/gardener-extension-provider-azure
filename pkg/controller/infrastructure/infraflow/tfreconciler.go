package infraflow

import (
	"context"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

type TfReconciler struct {
	tf      TerraformAdapter
	factory client.NewFactory
}

func NewTfReconciler(infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig, cluster *controller.Cluster, factory client.NewFactory) (*TfReconciler, error) {
	tfAdapter, err := NewTerraformAdapter(infra, cfg, cluster)
	return &TfReconciler{tfAdapter, factory}, err
}

func (f TfReconciler) Vnet(ctx context.Context) error {
	if f.tf.isCreate(TfVnet) {
		client, err := f.factory.Vnet()
		if err != nil {
			return err
		}
		return ReconcileVnetFromTf(ctx, f.tf, client)
	} else {
		return nil
	}
}
