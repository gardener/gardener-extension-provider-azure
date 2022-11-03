package infraflow

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type FlowReconciler struct {
	Factory client.NewFactory
}

func (f FlowReconciler) Reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig) error {
	//infrastructure.ComputeTerraformerTemplateValues(infra,cfg,) // use for migration of values..
	if cfg.ResourceGroup != nil {
		rgClient, err := f.Factory.ResourceGroup()
		if err != nil {
			return err
		}
		err = rgClient.CreateOrUpdate(ctx, cfg.ResourceGroup.Name, infra.Spec.Region)
		var log = logf.Log.WithName("gardener-extension-admission-azure")
		log.Info("Created resource group", *cfg.Networks.VNet.Name)

		if err == nil {
			vnetClient, err := f.Factory.Vnet()
			if err != nil {
				return err
			}
			if cfg.Networks.VNet.Name != nil {
				parameters := armnetwork.VirtualNetwork{
					Location: to.Ptr(infra.Spec.Region),
					Properties: &armnetwork.VirtualNetworkPropertiesFormat{
						AddressSpace: &armnetwork.AddressSpace{AddressPrefixes: []*string{cfg.Networks.VNet.CIDR}},
					},
				}
				err := vnetClient.Create(ctx, cfg.ResourceGroup.Name, *cfg.Networks.VNet.Name, parameters)
				log.Info("Created Vnet", *cfg.Networks.VNet.Name)
				return err
			}

		}
	}
	return nil
}
