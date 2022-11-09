package infraflow

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

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
	err = f.reconcileResourceGroupFromTf(ctx, tfAdapter)
	if err != nil {
		return err
	}
	securityGroupCh := make(chan armnetwork.SecurityGroupsClientCreateOrUpdateResponse)
	routeTableCh := make(chan armnetwork.RouteTable)
	natGatewayCh := make(chan map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse)
	g, ctx := errgroup.WithContext(ctx) //https://stackoverflow.com/questions/45500836/close-multiple-goroutine-if-an-error-occurs-in-one-in-go
	g.Go(func() error {
		// TODO create ddosProtectionPlan before? tf only supports reference.. no creation
		return f.reconcileVnetFromTf(ctx, tfAdapter)
	})
	g.Go(func() error {
		resp, err := f.reconcileRouteTablesFromTf(ctx, tfAdapter)
		routeTableCh <- resp
		//if err == nil {
		//	routeTableCh <- resp
		//}
		return err
	})
	g.Go(func() error {
		resp, err := f.reconcileSecurityGroupsFromTf(ctx, tfAdapter)
		securityGroupCh <- resp
		//if err == nil {
		//	securityGroupCh <- resp
		//}
		return err
	})
	g.Go(func() error {
		resp, err := f.reconcileNatGateways(ctx, tfAdapter) // TODO map in subneter
		natGatewayCh <- resp
		//err = fmt.Errorf("nat gateway error")
		//if err == nil {
		//	natGatewayCh <- resp
		//}
		return err
	})
	// TODO split dependent tasks into seperate group, use ctx to cancel?
	//if err := g.Wait(); err != nil {
	//return err
	//}
	//g, ctx = errgroup.WithContext(ctx) //https://stackoverflow.com/questions/45500836/close-multiple-goroutine-if-an-error-occurs-in-one-in-go
	g.Go(func() error {
		return f.reconcileSubnetsFromTf(ctx, tfAdapter, <-securityGroupCh, <-routeTableCh, <-natGatewayCh)
	})
	return g.Wait()
}

//func (f FlowReconciler) reconcileResourceGroup(infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig) flow.TaskFn {
//	return func(ctx context.Context) error {
//		if cfg.ResourceGroup != nil {
//			rgClient, err := f.Factory.ResourceGroup()
//			if err != nil {
//				return err
//			}
//			return rgClient.CreateOrUpdate(ctx, cfg.ResourceGroup.Name, infra.Spec.Region)
//		}
//		return nil
//	}
//}

// res: subnet to nat mapping
func (f FlowReconciler) reconcileNatGateways(ctx context.Context, tf TerraformAdapter) (res map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse, err error) {
	res = make(map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse)
	client, err := f.Factory.NatGateway()
	if err != nil {
		return res, err
	}
	nats := tf.Nats()
	for _, nat := range nats {
		if !nat.enabled {
			continue
		}
		resp, err := client.CreateOrUpdate(ctx, tf.ResourceGroup(), nat.name, armnetwork.NatGateway{})
		if err != nil {
			return res, err
		}
		res[nat.subnetName] = resp
	}
	return res, nil
}

func (f FlowReconciler) flowreconcileResourceGroupFromTf(tf TerraformAdapter) flow.TaskFn {
	return func(ctx context.Context) error {
		return f.reconcileResourceGroupFromTf(ctx, tf)
	}
}

func (f FlowReconciler) reconcileResourceGroupFromTf(ctx context.Context, tf TerraformAdapter) error {
	rgClient, err := f.Factory.ResourceGroup()
	if err != nil {
		return err
	}
	return rgClient.CreateOrUpdate(ctx, tf.ResourceGroup(), tf.Region())
}

func (f FlowReconciler) reconcileVnetFromTf(ctx context.Context, tf TerraformAdapter) error {
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

	vnetClient, err := f.Factory.Vnet()
	if err != nil {
		return err
	}
	return vnetClient.CreateOrUpdate(ctx, rgroup, vnet, parameters)
	//log.Info("Created Vnet", *cfg.Networks.VNet.Name)
}

func (f FlowReconciler) flowreconcileVnetFromTf(tf TerraformAdapter) flow.TaskFn {
	return func(ctx context.Context) error {
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

		vnetClient, err := f.Factory.Vnet()
		if err != nil {
			return err
		}
		return vnetClient.CreateOrUpdate(ctx, rgroup, vnet, parameters)
		//log.Info("Created Vnet", *cfg.Networks.VNet.Name)
	}
}

func (f FlowReconciler) reconcileVnetFromConfig(infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig) flow.TaskFn {
	return func(ctx context.Context) error {
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
}

func (f FlowReconciler) reconcileRouteTablesFromTf(ctx context.Context, tf TerraformAdapter) (armnetwork.RouteTable, error) {
	rclient, err := f.Factory.RouteTables()
	routeTableName := "worker_route_table" // #TODO set in infraconfig? (default injection)
	if err != nil {
		return armnetwork.RouteTable{}, err
	}
	parameters := armnetwork.RouteTable{
		Location: to.Ptr(tf.Region()),
		//Properties: &armnetwork.RouteTablePropertiesFormat{
		//},
	}
	resp, err := rclient.CreateOrUpdate(ctx, tf.ResourceGroup(), routeTableName, parameters)
	//log.Info("Created Vnet", *cfg.Networks.VNet.Name)
	return resp.RouteTable, err
}

func (f FlowReconciler) flowreconcileRouteTablesFromTf(tf TerraformAdapter) flow.TaskFn {
	return func(ctx context.Context) error {
		rclient, err := f.Factory.RouteTables()
		routeTableName := "worker_route_table" // #TODO set in infraconfig? (default injection)
		if err != nil {
			return err
		}
		parameters := armnetwork.RouteTable{
			Location: to.Ptr(tf.Region()),
			//Properties: &armnetwork.RouteTablePropertiesFormat{
			//},
		}
		_, err = rclient.CreateOrUpdate(ctx, tf.ResourceGroup(), routeTableName, parameters)
		//log.Info("Created Vnet", *cfg.Networks.VNet.Name)
		return err
	}
}

func (f FlowReconciler) reconcileRouteTables(infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig) flow.TaskFn {
	return func(ctx context.Context) error {
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
		_, err = rclient.CreateOrUpdate(ctx, cfg.ResourceGroup.Name, routeTableName, parameters)
		//log.Info("Created Vnet", *cfg.Networks.VNet.Name)
		return err
	}
}

func (f FlowReconciler) reconcileSecurityGroups(infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig) flow.TaskFn {
	return func(ctx context.Context) error {
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
		_, err = rclient.CreateOrUpdate(ctx, cfg.ResourceGroup.Name, name, parameters)
		//log.Info("Created Vnet", *cfg.Networks.VNet.Name)
		return err
	}
}

func (f FlowReconciler) reconcileSecurityGroupsFromTf(ctx context.Context, tf TerraformAdapter) (armnetwork.SecurityGroupsClientCreateOrUpdateResponse, error) {
	rclient, err := f.Factory.SecurityGroups()

	name := tf.ClusterName() + "-workers" // #TODO set in infraconfig? (default injection)
	if err != nil {
		return armnetwork.SecurityGroupsClientCreateOrUpdateResponse{}, err
	}
	parameters := armnetwork.SecurityGroup{
		Location: to.Ptr(tf.Region()),
		//Properties: &armnetwork.SecurityGroupPropertiesFormat{
		//},
	}
	resp, err := rclient.CreateOrUpdate(ctx, tf.ResourceGroup(), name, parameters)
	//log.Info("Created Vnet", *cfg.Networks.VNet.Name)
	return resp, err
}

func (f FlowReconciler) flowreconcileSecurityGroupsFromTf(tf TerraformAdapter) flow.TaskFn {
	return func(ctx context.Context) error {
		rclient, err := f.Factory.SecurityGroups()

		name := tf.ClusterName() + "-workers" // #TODO set in infraconfig? (default injection)
		if err != nil {
			return err
		}
		parameters := armnetwork.SecurityGroup{
			Location: to.Ptr(tf.Region()),
			//Properties: &armnetwork.SecurityGroupPropertiesFormat{
			//},
		}
		_, err = rclient.CreateOrUpdate(ctx, tf.ResourceGroup(), name, parameters)
		//log.Info("Created Vnet", *cfg.Networks.VNet.Name)
		return err
	}
}

func (f FlowReconciler) reconcileSubnetsFromTf(ctx context.Context, tf TerraformAdapter, securityGroup armnetwork.SecurityGroupsClientCreateOrUpdateResponse, routeTable armnetwork.RouteTable, nats map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse) error {
	subnetClient, err := f.Factory.Subnet()
	if err != nil {
		return err
	}
	subnets := tf.Subnets()
	for _, subnet := range subnets {
		endpoints := make([]*armnetwork.ServiceEndpointPropertiesFormat, 0)
		for _, endpoint := range subnet.serviceEndpoints {
			endpoints = append(endpoints, &armnetwork.ServiceEndpointPropertiesFormat{
				Service: to.Ptr(endpoint),
			})
		}

		parameters := armnetwork.Subnet{
			//Name: to.Ptr(subnet.name),
			Properties: &armnetwork.SubnetPropertiesFormat{
				AddressPrefix:    to.Ptr(subnet.cidr),
				ServiceEndpoints: endpoints, // TODO associate security group?, route table?
				NetworkSecurityGroup: &armnetwork.SecurityGroup{
					ID: securityGroup.ID,
				},
				RouteTable: &armnetwork.RouteTable{
					ID: routeTable.ID,
				},
				NatGateway: &armnetwork.SubResource{
					ID: nats[subnet.name].ID,
				},
			},
		}
		err = subnetClient.CreateOrUpdate(ctx, tf.ResourceGroup(), tf.Vnet().Name(), subnet.name, parameters)
	}
	//log.Info("Created Vnet", *cfg.Networks.VNet.Name)
	return err
}

func (f FlowReconciler) flowreconcileSubnetsFromTf(tf TerraformAdapter) flow.TaskFn {
	return func(ctx context.Context) error {
		subnetClient, err := f.Factory.Subnet()
		if err != nil {
			return err
		}
		name := "worker" // #TODO asset in infraconfig? (default injection)
		subnets := tf.Subnets()
		for _, subnet := range subnets {
			endpoints := make([]*armnetwork.ServiceEndpointPropertiesFormat, 0)
			for _, endpoint := range subnet.serviceEndpoints {
				endpoints = append(endpoints, &armnetwork.ServiceEndpointPropertiesFormat{
					Service: to.Ptr(endpoint),
				})
			}

			parameters := armnetwork.Subnet{
				Name: to.Ptr(subnet.name),
				Properties: &armnetwork.SubnetPropertiesFormat{
					AddressPrefix:    to.Ptr(subnet.cidr),
					ServiceEndpoints: endpoints, // TODO associate security group?, route table?
					//NetworkSecurityGroup: &armnetwork.SecurityGroup{
					//	ID: to.Ptr(tf.SecurityGroup()["id"].(string)),
					//},
					//RouteTable: &armnetwork.SubResource{
					//	ID: to.Ptr(tf.RouteTable()["id"].(string)),
					//},
				},
			}
			err = subnetClient.CreateOrUpdate(ctx, tf.ResourceGroup(), tf.Vnet()["name"].(string), name, parameters)
		}
		//log.Info("Created Vnet", *cfg.Networks.VNet.Name)
		return err
	}
}

// todo copy infra.spec part
func (f FlowReconciler) buildReconcileGraph(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig, tf TerraformAdapter) *flow.Graph {
	g := flow.NewGraph("Azure infrastructure reconcilation")
	// do not need to check if should be created? (otherwise just updates resource)
	resourceGroup := f.AddTask(g, "resource group creation", f.flowreconcileResourceGroupFromTf(tf))
	vnet := f.AddTask(g, "vnet creation", f.flowreconcileVnetFromTf(tf), shared.Dependencies(resourceGroup))
	routeTables := f.AddTask(g, "route table creation", f.flowreconcileRouteTablesFromTf(tf), shared.Dependencies(vnet)) // TODO dependencies not inherent ?
	f.AddTask(g, "security group creation", f.flowreconcileSecurityGroupsFromTf(tf), shared.Dependencies(routeTables))
	//f.AddTask(g,"subnet creation")
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
