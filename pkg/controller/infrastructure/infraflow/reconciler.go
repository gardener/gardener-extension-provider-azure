package infraflow

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow/shared"
	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	"k8s.io/utils/pointer"
)

type FlowReconciler struct {
	Factory client.NewFactory
}

func (f FlowReconciler) Reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig, cluster *controller.Cluster) error {
	tfAdapter, err := NewTerraformAdapter(infra, cfg, cluster)
	if err != nil {
		return err
	}
	reconciler, err := NewTfReconciler(infra, cfg, cluster, f.Factory) // TODO pass type once into FlowReconciler constructor, decouples from tfAdapter
	if err != nil {
		return err
	}
	graph := f.buildReconcileGraph(ctx, infra, cfg, tfAdapter, reconciler)
	fl := graph.Compile()
	if err := fl.Run(ctx, flow.Opts{}); err != nil {
		return flow.Causes(err)
	}
	return nil
}

func flowTask(tf TerraformAdapter, fn func(context.Context, TerraformAdapter) error) flow.TaskFn {
	return func(ctx context.Context) error {
		return fn(ctx, tf)
	}
}

func flowTaskWithReturn[T any](tf TerraformAdapter, fn func(context.Context, TerraformAdapter) (T, error), ch chan<- T) flow.TaskFn {
	return func(ctx context.Context) error {
		resp, err := fn(ctx, tf)
		ch <- resp
		return err
	}
}

func flowTaskWithReturnAndInput[T any, K any](tf TerraformAdapter, input <-chan K, fn func(context.Context, TerraformAdapter, K) (T, error), ch chan<- T) flow.TaskFn {
	return func(ctx context.Context) error {
		resp, err := fn(ctx, tf, <-input)
		ch <- resp
		return err
	}
}

func (f FlowReconciler) reconcileResourceGroupFromTf(ctx context.Context, tf TerraformAdapter) error {
	rgClient, err := f.Factory.ResourceGroup()
	if err != nil {
		return err
	}
	return rgClient.CreateOrUpdate(ctx, tf.ResourceGroup(), tf.Region())
}

func ReconcileVnetFromTf(ctx context.Context, tf TerraformAdapter, vnetClient client.Vnet) error {
	parameters := armnetwork.VirtualNetwork{
		Location: to.Ptr(tf.Region()),
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{},
		},
	}

	cidr, ok := tf.Vnet()["cidr"]
	if ok {
		parameters.Properties.AddressSpace.AddressPrefixes = []*string{to.Ptr(cidr.(string))}
	}

	ddosId, ok := tf.Vnet()["ddosProtectionPlanID"]
	if ok {
		ddosIdString := ddosId.(string)
		parameters.Properties.EnableDdosProtection = to.Ptr(true)
		parameters.Properties.DdosProtectionPlan = &armnetwork.SubResource{ID: to.Ptr(ddosIdString)}
	}

	rgroup := tf.ResourceGroup()
	vnet := tf.Vnet()["name"].(string)
	return vnetClient.CreateOrUpdate(ctx, rgroup, vnet, parameters)
}

//func (f FlowReconciler) reconcileVnet(infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig) flow.TaskFn {
//	return func(ctx context.Context) error {
//		vnetClient, err := f.Factory.Vnet()
//		if err != nil {
//			return err
//		}
//		if cfg.Networks.VNet.Name != nil {
//			parameters := armnetwork.VirtualNetwork{
//				Location: to.Ptr(infra.Spec.Region),
//				Properties: &armnetwork.VirtualNetworkPropertiesFormat{
//					AddressSpace: &armnetwork.AddressSpace{AddressPrefixes: []*string{cfg.Networks.VNet.CIDR}},
//				},
//			}
//			err := vnetClient.CreateOrUpdate(ctx, cfg.ResourceGroup.Name, *cfg.Networks.VNet.Name, parameters)
//			//log.Info("Created Vnet", *cfg.Networks.VNet.Name)
//			return err
//		}
//		return nil
//	}
//}

func ReconcileRouteTablesFromTf(tf TerraformAdapter, rclient client.RouteTables, ctx context.Context) (armnetwork.RouteTable, error) {
	routeTableName := "worker_route_table"
	parameters := armnetwork.RouteTable{
		Location: to.Ptr(tf.Region()),
	}
	resp, err := rclient.CreateOrUpdate(ctx, tf.ResourceGroup(), routeTableName, parameters)

	return resp.RouteTable, err
}

func ReconcileSecurityGroupsFromTf(tf TerraformAdapter, rclient client.SecurityGroups, ctx context.Context) (armnetwork.SecurityGroupsClientCreateOrUpdateResponse, error) {
	name := tf.ClusterName() + "-workers"
	parameters := armnetwork.SecurityGroup{
		Location: to.Ptr(tf.Region()),
	}
	resp, err := rclient.CreateOrUpdate(ctx, tf.ResourceGroup(), name, parameters)

	return resp, err
}

func flowTaskNew[T any](clientFn func() (T, error), reconcileFn func(ctx context.Context, client T) error) flow.TaskFn {
	return func(ctx context.Context) error {
		client, err := clientFn()
		if err != nil {
			return err
		}
		return reconcileFn(ctx, client)
	}
}

// todo copy infra.spec part
func (f FlowReconciler) buildReconcileGraph(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig, tf TerraformAdapter, reconciler *TfReconciler) *flow.Graph {
	g := flow.NewGraph("Azure infrastructure reconcilation")
	// do not need to check if should be created? (otherwise just updates resource)
	resourceGroup := f.AddTask(g, "resource group creation", flowTask(tf, f.reconcileResourceGroupFromTf))

	// or wrapped
	f.AddTask(g, "vnet creation", reconciler.Vnet, shared.Dependencies(resourceGroup))

	f.AddTask(g, "availability set creation", reconciler.AvailabilitySet, shared.Dependencies(resourceGroup))
	//flowTask(tf, f.reconcileAvailabilitySetFromTf), shared.Dependencies(resourceGroup))

	routeTableCh := make(chan armnetwork.RouteTable, 1)
	routeTable := f.AddTask(g, "route table creation", func(ctx context.Context) error {
		routeTable, err := reconciler.RouteTables(ctx)
		// TODO write to whiteboard
		routeTableCh <- routeTable
		return err
	}, shared.Dependencies(resourceGroup))
	//flowTaskWithReturn(tf, f.reconcileRouteTablesFromTf, routeTableCh), shared.Dependencies(resourceGroup))

	securityGroupCh := make(chan armnetwork.SecurityGroupsClientCreateOrUpdateResponse, 1)
	securityGroup := f.AddTask(g, "security group creation", func(ctx context.Context) error {
		securityGroup, err := reconciler.SecurityGroups(ctx)
		securityGroupCh <- securityGroup
		return err
	}, shared.Dependencies(resourceGroup))

	//flowTaskWithReturn(tf, f.reconcileSecurityGroupsFromTf, securityGroupCh), shared.Dependencies(resourceGroup))

	ipCh := make(chan map[string]armnetwork.PublicIPAddressesClientCreateOrUpdateResponse, 1) // why not working without buf number?
	f.AddTask(g, "ips creation", func(ctx context.Context) error {
		ips, err := reconciler.PublicIPs(ctx)
		ipCh <- ips
		return err
	}, shared.Dependencies(resourceGroup))
	//flowTaskWithReturn(tf, f.reconcilePublicIPsFromTf, ipCh), shared.Dependencies(resourceGroup))

	natGatewayCh := make(chan map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse, 1)
	natGateway := f.AddTask(g, "nat gateway creation", func(ctx context.Context) error {
		resp, err := reconciler.NatGateways(ctx, <-ipCh)
		natGatewayCh <- resp
		return err
	})
	//flowTaskWithReturnAndInput(tf, ipCh, f.reconcileNatGatewaysFromTf, natGatewayCh))

	f.AddTask(g, "subnet creation", func(ctx context.Context) error {
		//whiteboard["security"]
		return reconciler.Subnets(ctx, <-securityGroupCh, <-routeTableCh, <-natGatewayCh)
	}, shared.Dependencies(resourceGroup), shared.Dependencies(securityGroup), shared.Dependencies(routeTable), shared.Dependencies(natGateway)) // TODO not necessary to declare dependencies? coz channels ensure to wait
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
