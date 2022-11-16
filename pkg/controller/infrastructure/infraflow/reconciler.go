package infraflow

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow/shared"
	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	"golang.org/x/sync/errgroup"
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
	reconciler, err := NewTfReconciler(infra, cfg, cluster) // TODO pass type once into FlowReconciler constructor, decouples from tfAdapter
	if err != nil {
		return err
	}
	graph := f.buildReconcileGraph(ctx, infra, cfg, tfAdapter, reconciler)
	fl := graph.Compile()
	if err := fl.Run(ctx, flow.Opts{}); err != nil {
		return flow.Causes(err)
	}
	return nil
	// other approach
	//return f.buildGoRoutineFlow(err, ctx, tfAdapter)

}

func (f FlowReconciler) buildGoRoutineFlow(err error, ctx context.Context, tfAdapter TerraformAdapter) error {
	err = f.reconcileResourceGroupFromTf(ctx, tfAdapter)
	if err != nil {
		return err
	}
	securityGroupCh := make(chan armnetwork.SecurityGroupsClientCreateOrUpdateResponse)
	routeTableCh := make(chan armnetwork.RouteTable)
	ipCh := make(chan map[string]armnetwork.PublicIPAddressesClientCreateOrUpdateResponse)
	natGatewayCh := make(chan map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse, 2)
	g, ctx := errgroup.WithContext(ctx) //https://stackoverflow.com/questions/45500836/close-multiple-goroutine-if-an-error-occurs-in-one-in-go
	g.Go(func() error {
		resp, err := f.reconcilePublicIPsFromTf(ctx, tfAdapter)
		ipCh <- resp
		return err
	})
	g.Go(func() error {
		// TODO create ddosProtectionPlan before? tf only supports reference.. no creation
		return f.reconcileVnetFromTf(ctx, tfAdapter)
	})
	g.Go(func() error {
		resp, err := f.reconcileRouteTablesFromTf(ctx, tfAdapter)
		routeTableCh <- resp
		return err
	})
	g.Go(func() error {
		resp, err := f.reconcileSecurityGroupsFromTf(ctx, tfAdapter)
		securityGroupCh <- resp
		return err
	})
	g.Go(func() error {
		return f.reconcileAvailabilitySetFromTf(ctx, tfAdapter)
	})
	g.Go(func() error {
		resp, err := f.reconcileNatGatewaysFromTf(ctx, tfAdapter, <-ipCh)
		natGatewayCh <- resp // TODO use https://betterprogramming.pub/how-to-broadcast-messages-in-go-using-channels-b68f42bdf32e https://stackoverflow.com/questions/36417199/how-to-broadcast-message-using-channel
		//err = fmt.Errorf("nat gateway error")
		return err
	})
	// TODO split dependent tasks into seperate group, use ctx to cancel?
	//if err := g.Wait(); err != nil {
	//return err
	//}
	//g, ctx := errgroup.WithContext(ctx) //https://stackoverflow.com/questions/45500836/close-multiple-goroutine-if-an-error-occurs-in-one-in-go
	//err = fmt.Errorf("nat gateway error")
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

func (f FlowReconciler) reconcileAvailabilitySetFromTf(ctx context.Context, tf TerraformAdapter) error {
	if tf.isCreate(TfAvailabilitySet) {
		asClient, err := f.Factory.AvailabilitySet()
		if err != nil {
			return err
		}
		parameters := armcompute.AvailabilitySet{
			Location: to.Ptr(tf.Region()),
			Properties: &armcompute.AvailabilitySetProperties{
				PlatformFaultDomainCount:  to.Ptr(tf.CountFaultDomains()),
				PlatformUpdateDomainCount: to.Ptr(tf.CountUpdateDomains()),
			},
			SKU: &armcompute.SKU{Name: to.Ptr(string(armcompute.AvailabilitySetSKUTypesAligned))}, // equal to managed = True in tf
		}
		_, err = asClient.CreateOrUpdate(ctx, tf.ResourceGroup(), tf.AvailabilitySetName(), parameters)
		return err
	} else {
		return nil
	}
}

// res: subnet to ip mapping
func (f FlowReconciler) reconcilePublicIPsFromTf(ctx context.Context, tf TerraformAdapter) (map[string]armnetwork.PublicIPAddressesClientCreateOrUpdateResponse, error) {
	res := make(map[string]armnetwork.PublicIPAddressesClientCreateOrUpdateResponse)
	client, err := f.Factory.PublicIP()
	if err != nil {
		return res, err
	}
	for _, ip := range tf.IPs() {
		resp, err := client.CreateOrUpdate(ctx, tf.ResourceGroup(), ip.name, armnetwork.PublicIPAddress{
			Location: to.Ptr(tf.Region()),
			SKU:      &armnetwork.PublicIPAddressSKU{Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard)},
			Properties: &armnetwork.PublicIPAddressPropertiesFormat{
				PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
			},
			// TODO zones prop?
		})
		if err != nil {
			return res, err
		}
		res[ip.subnetName] = resp
	}
	return res, nil
}

// res: subnet to nat mapping
func (f FlowReconciler) reconcileNatGatewaysFromTf(ctx context.Context, tf TerraformAdapter, ips map[string]armnetwork.PublicIPAddressesClientCreateOrUpdateResponse) (res map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse, err error) {
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
		resp, err := client.CreateOrUpdate(ctx, tf.ResourceGroup(), nat.name, armnetwork.NatGateway{
			Properties: &armnetwork.NatGatewayPropertiesFormat{
				PublicIPAddresses: []*armnetwork.SubResource{{ID: ips[nat.subnetName].ID}},
			},
		})
		if err != nil {
			return res, err
		}
		res[nat.subnetName] = resp
	}
	return res, nil
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

func (f FlowReconciler) reconcileVnetFromTf(ctx context.Context, tf TerraformAdapter) error {
	vnetClient, err := f.Factory.Vnet()
	if err != nil {
		return err
	}
	return ReconcileVnetFromTf(ctx, tf, vnetClient)
	//log.Info("Created Vnet", *cfg.Networks.VNet.Name)
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

	//vnetClient, err := f.Factory.Vnet() // TODO: good - fail before starting graph??
	//if err != nil {
	//	panic(err)
	//}
	//f.AddTask(g, "vnet creation", func(ctx context.Context) error {
	//	return reconciler.Vnet(ctx, vnetClient)
	//}, shared.Dependencies(resourceGroup))
	// or wrapped
	f.AddTask(g, "vnet creation", flowTaskNew(f.Factory.Vnet, reconciler.Vnet), shared.Dependencies(resourceGroup))

	f.AddTask(g, "availability set creation", flowTask(tf, f.reconcileAvailabilitySetFromTf), shared.Dependencies(resourceGroup))

	routeTableCh := make(chan armnetwork.RouteTable, 1)
	routeTable := f.AddTask(g, "route table creation", flowTaskWithReturn(tf, f.reconcileRouteTablesFromTf, routeTableCh), shared.Dependencies(resourceGroup)) // TODO dependencies not inherent ?

	securityGroupCh := make(chan armnetwork.SecurityGroupsClientCreateOrUpdateResponse, 1)
	securityGroup := f.AddTask(g, "security group creation", flowTaskWithReturn(tf, f.reconcileSecurityGroupsFromTf, securityGroupCh), shared.Dependencies(resourceGroup))

	ipCh := make(chan map[string]armnetwork.PublicIPAddressesClientCreateOrUpdateResponse, 1) // why not working without buf number?
	f.AddTask(g, "ips creation", flowTaskWithReturn(tf, f.reconcilePublicIPsFromTf, ipCh), shared.Dependencies(resourceGroup))

	natGatewayCh := make(chan map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse, 1)
	natGateway := f.AddTask(g, "nat gateway creation", func(ctx context.Context) error {
		resp, err := f.reconcileNatGatewaysFromTf(ctx, tf, <-ipCh)
		natGatewayCh <- resp
		return err
	})
	//flowTaskWithReturnAndInput(tf, ipCh, f.reconcileNatGatewaysFromTf, natGatewayCh))

	f.AddTask(g, "subnet creation", func(ctx context.Context) error {
		return f.reconcileSubnetsFromTf(ctx, tf, <-securityGroupCh, <-routeTableCh, <-natGatewayCh)
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
