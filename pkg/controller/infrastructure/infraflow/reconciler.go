package infraflow

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow/shared"
	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	"k8s.io/utils/pointer"
)

const (
	RouteTableID  = "route_table_id"
	SGroupID      = "security_group_id"
	NatGatewayMap = "nategateway_map"
	PublicIPMap   = "public_ip_map"
)

type FlowReconciler struct {
	Factory    client.NewFactory
	reconciler *TfReconciler
}

func NewFlowReconciler(factory client.NewFactory) *FlowReconciler {
	return &FlowReconciler{
		Factory: factory,
	}
}

func (f *FlowReconciler) Delete(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig, cluster *controller.Cluster) error {
	reconciler, err := NewTfReconciler(infra, cfg, cluster, f.Factory)
	if err != nil {
		return err
	}
	return reconciler.Delete(ctx)
}

// TODO pass dummy Reconcilied struct to ensure it was called before
func (f FlowReconciler) GetInfrastructureStatus(ctx context.Context, cfg *azure.InfrastructureConfig) (*v1alpha1.InfrastructureStatus, error) {
	if f.reconciler == nil {
		return nil, fmt.Errorf("reconciler not initialized, call Reconcile before")
	}
	return f.reconciler.GetInfrastructureStatus(ctx, cfg)
}

func (f *FlowReconciler) Reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig, cluster *controller.Cluster) error {
	tfAdapter, err := NewTerraformAdapter(infra, cfg, cluster)
	if err != nil {
		return err
	}
	reconciler, err := NewTfReconciler(infra, cfg, cluster, f.Factory) // TODO pass type once into FlowReconciler constructor, decouples from tfAdapter
	f.reconciler = reconciler

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
	routeTableName := tf.RouteTableName()
	parameters := armnetwork.RouteTable{
		Location: to.Ptr(tf.Region()),
	}
	resp, err := rclient.CreateOrUpdate(ctx, tf.ResourceGroup(), routeTableName, parameters)

	return resp.RouteTable, err
}

func ReconcileSecurityGroupsFromTf(tf TerraformAdapter, rclient client.SecurityGroups, ctx context.Context) (armnetwork.SecurityGroupsClientCreateOrUpdateResponse, error) {
	name := tf.SecurityGroupName()
	parameters := armnetwork.SecurityGroup{
		Location: to.Ptr(tf.Region()),
	}
	resp, err := rclient.CreateOrUpdate(ctx, tf.ResourceGroup(), name, parameters)

	return resp, err
}

// todo copy infra.spec part
func (f FlowReconciler) buildReconcileGraph(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig, tf TerraformAdapter, reconciler *TfReconciler) *flow.Graph {
	whiteboard := shared.NewWhiteboard()

	g := flow.NewGraph("Azure infrastructure reconcilation")
	resourceGroup := f.AddTask(g, "resource group creation", flowTask(tf, f.reconcileResourceGroupFromTf))

	vnet := f.AddTask(g, "vnet creation", reconciler.Vnet, shared.Dependencies(resourceGroup))

	f.AddTask(g, "availability set creation", reconciler.AvailabilitySet, shared.Dependencies(resourceGroup))

	routeTable := f.AddTask(g, "route table creation", func(ctx context.Context) error {
		routeTable, err := reconciler.RouteTables(ctx)
		whiteboard.Set(RouteTableID, *routeTable.ID)
		return err
	}, shared.Dependencies(resourceGroup))

	securityGroup := f.AddTask(g, "security group creation", func(ctx context.Context) error {
		securityGroup, err := reconciler.SecurityGroups(ctx)
		whiteboard.Set(SGroupID, *securityGroup.ID)
		return err
	}, shared.Dependencies(resourceGroup))

	ip := f.AddTask(g, "ips creation", func(ctx context.Context) error {
		ips, err := reconciler.PublicIPs(ctx)
		// add user managed Ips for NAT association
		reconciler.EnrichResponseWithUserManagedIPs(ctx, ips)
		whiteboard.SetObject(PublicIPMap, ips)
		return err
	}, shared.Dependencies(resourceGroup))

	natGateway := f.AddTask(g, "nat gateway creation", func(ctx context.Context) error {
		ips := whiteboard.GetObject(PublicIPMap).(map[string]armnetwork.PublicIPAddress)
		resp, err := reconciler.NatGateways(ctx, ips)
		whiteboard.SetObject(NatGatewayMap, resp)
		return err
	}, shared.Dependencies(ip))

	f.AddTask(g, "subnet creation", func(ctx context.Context) error {
		routeTable := armnetwork.RouteTable{
			ID: whiteboard.Get(RouteTableID),
		}
		securityGroup := armnetwork.SecurityGroup{
			ID: whiteboard.Get(SGroupID),
		}
		natGateway := whiteboard.GetObject(NatGatewayMap).(map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse)
		return reconciler.Subnets(ctx, securityGroup, routeTable, natGateway)
	}, shared.Dependencies(resourceGroup), shared.Dependencies(securityGroup), shared.Dependencies(routeTable), shared.Dependencies(natGateway), shared.Dependencies(vnet))
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
