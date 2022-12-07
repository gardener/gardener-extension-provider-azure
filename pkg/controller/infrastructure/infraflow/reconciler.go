package infraflow

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow/shared"
	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/go-logr/logr"
	"k8s.io/utils/pointer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// Key names for the whiteboard object to pass results between the reconcilation tasks
const (
	routeTableID  = "route_table_id"
	sGroupID      = "security_group_id"
	natGatewayMap = "nategateway_map"
	publicIPMap   = "public_ip_map"
)

// FlowReconciler is the reconciler for all managed resources
type FlowReconciler struct {
	factory    client.NewFactory
	reconciler *TfReconciler // only used to retrieve GetInfrastructureStatus after reconcilation call
	logger     logr.Logger
}

// NewFlowReconciler creates a new FlowReconciler
func NewFlowReconciler(factory client.NewFactory, logger logr.Logger) *FlowReconciler {
	return &FlowReconciler{
		factory: factory,
		logger:  logger,
	}
}

// Delete deletes all resources managed by the reconciler
func (f *FlowReconciler) Delete(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig, cluster *controller.Cluster) error {
	reconciler, err := NewTfReconciler(infra, cfg, cluster, f.factory)
	if err != nil {
		return err
	}
	return reconciler.Delete(ctx)
}

// TODO pass dummy Reconcilied struct to ensure it was called before
// GetInfrastructureStatus returns the infrastructure status
func (f FlowReconciler) GetInfrastructureStatus(ctx context.Context, cfg *azure.InfrastructureConfig) (*v1alpha1.InfrastructureStatus, error) {
	if f.reconciler == nil {
		return nil, fmt.Errorf("reconciler not initialized, call Reconcile before")
	}
	return f.reconciler.GetInfrastructureStatus(ctx, cfg)
}

// Reconcile reconciles all resources
func (f *FlowReconciler) Reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig, cluster *controller.Cluster) error {
	reconciler, err := NewTfReconciler(infra, cfg, cluster, f.factory)
	f.reconciler = reconciler
	if err != nil {
		return err
	}
	graph := f.buildReconcileGraph(ctx, infra, cfg, reconciler)
	fl := graph.Compile()
	if err := fl.Run(ctx, flow.Opts{}); err != nil {
		return flow.Causes(err)
	}
	return nil
}

// TODO copy infra.spec part
func (f FlowReconciler) buildReconcileGraph(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cfg *azure.InfrastructureConfig, reconciler *TfReconciler) *flow.Graph {
	whiteboard := shared.NewWhiteboard()

	g := flow.NewGraph("Azure infrastructure reconcilation")
	resourceGroup := f.AddTask(g, "resource group creation", reconciler.ResourceGroup)

	vnet := f.AddTask(g, "vnet creation", reconciler.Vnet, shared.Dependencies(resourceGroup))

	f.AddTask(g, "availability set creation", reconciler.AvailabilitySet, shared.Dependencies(resourceGroup))

	routeTable := f.AddTask(g, "route table creation", func(ctx context.Context) error {
		routeTable, err := reconciler.RouteTables(ctx)
		whiteboard.Set(routeTableID, *routeTable.ID)
		return err
	}, shared.Dependencies(resourceGroup))

	securityGroup := f.AddTask(g, "security group creation", func(ctx context.Context) error {
		securityGroup, err := reconciler.SecurityGroups(ctx)
		whiteboard.Set(sGroupID, *securityGroup.ID)
		return err
	}, shared.Dependencies(resourceGroup))

	ip := f.AddTask(g, "ips creation", func(ctx context.Context) error {
		ips, err := reconciler.PublicIPs(ctx)
		if err != nil {
			return err
		}
		err = reconciler.EnrichResponseWithUserManagedIPs(ctx, ips)
		if err != nil {
			return fmt.Errorf("enrichment with user managed IPs failed: %v", err)
		}
		whiteboard.SetObject(publicIPMap, ips)
		return nil
	}, shared.Dependencies(resourceGroup))

	natGateway := f.AddTask(g, "nat gateway creation", func(ctx context.Context) error {
		ips := whiteboard.GetObject(publicIPMap).(map[string][]armnetwork.PublicIPAddress)
		resp, err := reconciler.NatGateways(ctx, ips)
		whiteboard.SetObject(natGatewayMap, resp)
		return err
	}, shared.Dependencies(ip))

	f.AddTask(g, "subnet creation", func(ctx context.Context) error {
		routeTable := armnetwork.RouteTable{
			ID: whiteboard.Get(routeTableID),
		}
		securityGroup := armnetwork.SecurityGroup{
			ID: whiteboard.Get(sGroupID),
		}
		natGateway := whiteboard.GetObject(natGatewayMap).(map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse)
		return reconciler.Subnets(ctx, securityGroup, routeTable, natGateway)
	}, shared.Dependencies(resourceGroup), shared.Dependencies(securityGroup), shared.Dependencies(routeTable), shared.Dependencies(natGateway), shared.Dependencies(vnet))
	return g

}

// TODO copy from AWS PR (use taskBuilder component to share?)
// AddTask adds a wrapped task for the given task function and options.
func (f FlowReconciler) AddTask(g *flow.Graph, name string, fn flow.TaskFn, options ...shared.TaskOption) flow.TaskIDer {
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
		Fn:   f.wrapTaskFn(g.Name(), name, tunedFn),
	}

	if len(allOptions.Dependencies) > 0 {
		task.Dependencies = flow.NewTaskIDs(allOptions.Dependencies...)
	}

	return g.Add(task)
}

func (f FlowReconciler) wrapTaskFn(flowName, taskName string, fn flow.TaskFn) flow.TaskFn {
	return func(ctx context.Context) error {
		taskCtx := logf.IntoContext(ctx, f.logger.WithValues("flow", flowName, "task", taskName))
		err := fn(taskCtx)
		if err != nil {
			// don't wrap error with '%w', as otherwise the error context get lost
			err = fmt.Errorf("failed to %s: %s", taskName, err)
		}
		return err
	}
}
