package infraflow

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow/shared"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	"k8s.io/utils/pointer"
)

type FlowReconciler struct {
	Factory client.NewFactory
}

func (f FlowReconciler) Reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig) error {
	//infrastructure.ComputeTerraformerTemplateValues(infra,cfg,) // use for migration of values..
	g := f.buildReconcileGraph(ctx, infra, cfg)
	fl := g.Compile()
	if err := fl.Run(ctx, flow.Opts{}); err != nil {
		return flow.Causes(err)
	}
	return nil
}

func (f FlowReconciler) buildReconcileGraph(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig) *flow.Graph {
	g := flow.NewGraph("Azure infrastructure reconcilation")

	fnCreateResourceGroup := func(ctx context.Context) error {
		if cfg.ResourceGroup != nil {
			rgClient, err := f.Factory.ResourceGroup()
			if err != nil {
				return err
			}
			return rgClient.CreateOrUpdate(ctx, cfg.ResourceGroup.Name, infra.Spec.Region)
		}
		return nil
	}
	resourceGroup := f.AddTask(g, "resource group creation", fnCreateResourceGroup)

	fnCreateVnet := func(ctx context.Context) error {
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
			err := vnetClient.CreateOrUpdate(ctx, cfg.ResourceGroup.Name, *cfg.Networks.VNet.Name, parameters)
			//log.Info("Created Vnet", *cfg.Networks.VNet.Name)
			return err
		}
		return nil
	}
	vnet := f.AddTask(g, "vnet creation", fnCreateVnet, shared.Dependencies(resourceGroup))

	fnCreateRoutes := func(ctx context.Context) error {
		rclient, err := f.Factory.RouteTables()
		routeTableName := "worker_route_table" // #TODO set in infraconfig? (default injection)
		if err != nil {
			return err
		}
		parameters := armnetwork.RouteTable{
			Location: to.Ptr(infra.Spec.Region),
			//Properties: &armnetwork.RouteTablePropertiesFormat{
			//},
		}
		err = rclient.CreateOrUpdate(ctx, cfg.ResourceGroup.Name, routeTableName, parameters)
		//log.Info("Created Vnet", *cfg.Networks.VNet.Name)
		return err
	}
	routeTables := f.AddTask(g, "route table creation", fnCreateRoutes, shared.Dependencies(vnet))

	fnCreateSGroup := func(ctx context.Context) error {
		rclient, err := f.Factory.SecurityGroups()
		if infra.Namespace == "" { // TODO validate before?
			return fmt.Errorf("namespace is empty")
		}
		name := infra.Namespace + "-workers" // #TODO set in infraconfig? (default injection)
		if err != nil {
			return err
		}
		parameters := armnetwork.SecurityGroup{
			Location: to.Ptr(infra.Spec.Region),
			//Properties: &armnetwork.SecurityGroupPropertiesFormat{
			//},
		}
		err = rclient.CreateOrUpdate(ctx, cfg.ResourceGroup.Name, name, parameters)
		//log.Info("Created Vnet", *cfg.Networks.VNet.Name)
		return err
	}
	f.AddTask(g, "security group creation", fnCreateSGroup, shared.Dependencies(routeTables))
	return g

}

// AddTask adds a wrapped task for the given task function and options.
func (c FlowReconciler) AddTask(g *flow.Graph, name string, fn flow.TaskFn, options ...shared.TaskOption) flow.TaskIDer {
	allOptions := shared.TaskOption{}
	for _, opt := range options {
		if len(opt.Dependencies) > 0 {
			allOptions.Dependencies = append(allOptions.Dependencies, opt.Dependencies...)
		}
		if opt.Timeout > 0 {
			allOptions.Timeout = opt.Timeout
		}
		if opt.DoIf != nil {
			condition := true
			if allOptions.DoIf != nil {
				condition = *allOptions.DoIf
			}
			condition = condition && *opt.DoIf
			allOptions.DoIf = pointer.Bool(condition)
		}
	}

	tunedFn := fn
	if allOptions.DoIf != nil {
		tunedFn = tunedFn.DoIf(*allOptions.DoIf)
		if !*allOptions.DoIf {
			name = "[Skipped] " + name
		}
	}
	if allOptions.Timeout > 0 {
		tunedFn = tunedFn.Timeout(allOptions.Timeout)
	}
	task := flow.Task{
		Name: name,
		Fn:   c.wrapTaskFn(g.Name(), name, tunedFn),
	}

	if len(allOptions.Dependencies) > 0 {
		task.Dependencies = flow.NewTaskIDs(allOptions.Dependencies...)
	}

	return g.Add(task)
}

func (c FlowReconciler) wrapTaskFn(flowName, taskName string, fn flow.TaskFn) flow.TaskFn {
	return func(ctx context.Context) error {
		//taskCtx := logf.IntoContext(ctx, c.Log.WithValues("flow", flowName, "task", taskName))
		err := fn(ctx) //fn(taskCtx)
		if err != nil {
			// don't wrap error with '%w', as otherwise the error context get lost
			err = fmt.Errorf("failed to %s: %s", taskName, err)
		}
		//if perr := c.PersistState(taskCtx, false); perr != nil {
		//	if err != nil {
		//		c.Log.Error(perr, "persisting state failed")
		//	} else {
		//		err = perr
		//	}
		//}
		return err
	}
}
