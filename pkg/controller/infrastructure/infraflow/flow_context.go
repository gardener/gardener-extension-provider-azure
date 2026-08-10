// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/go-logr/logr"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow/shared"
	infrainternal "github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

const (
	defaultTimeout     = 2 * time.Minute
	defaultLongTimeout = 4 * time.Minute
)

var setSeparator = sync.OnceFunc(func() {
	shared.Separator = "|"
})

func init() {
	setSeparator()
}

// FlowContext is the reconciler for all managed resources
type FlowContext struct {
	log            logr.Logger
	client         k8sclient.Client
	cfg            *azure.InfrastructureConfig
	factory        client.Factory
	auth           *client.ClientAuth
	infra          *extensionsv1alpha1.Infrastructure
	state          *azure.InfrastructureState
	status         *azure.InfrastructureStatus
	cluster        *controller.Cluster
	whiteboard     shared.Whiteboard
	adapter        *InfrastructureAdapter
	providerAccess Access
	inventory      *Inventory

	*shared.BasicFlowContext
}

// Opts contains the options to initialize a FlowContext.
type Opts struct {
	Client  k8sclient.Client
	Factory client.Factory
	Auth    *client.ClientAuth
	Logger  logr.Logger
	Infra   *extensionsv1alpha1.Infrastructure
	Cluster *controller.Cluster
	State   *azure.InfrastructureState
}

// NewFlowContext creates a new FlowContext.
func NewFlowContext(opts Opts) (*FlowContext, error) {
	wb := shared.NewWhiteboard()
	wb.ImportFromFlatMap(opts.State.Data)

	cfg, err := helper.InfrastructureConfigFromInfrastructure(opts.Infra)
	if err != nil {
		return nil, err
	}

	status, err := helper.InfrastructureStatusFromInfrastructure(opts.Infra)
	if err != nil {
		return nil, err
	}

	cloudProfileCfg, err := helper.CloudProfileConfigFromCluster(opts.Cluster)
	if err != nil {
		return nil, err
	}

	inv := NewSimpleInventory(wb)
	for _, r := range opts.State.ManagedItems {
		if err := inv.Insert(r.ID); err != nil {
			return nil, err
		}
	}

	adapter, err := NewInfrastructureAdapter(
		opts.Infra,
		cfg,
		status,
		cloudProfileCfg,
		opts.Cluster,
	)
	if err != nil {
		return nil, err
	}

	fc := &FlowContext{
		factory:    opts.Factory,
		client:     opts.Client,
		auth:       opts.Auth,
		log:        opts.Logger,
		infra:      opts.Infra,
		state:      opts.State,
		cluster:    opts.Cluster,
		cfg:        cfg,
		status:     status,
		whiteboard: wb,
		providerAccess: &access{
			opts.Factory,
		},
		adapter:   adapter,
		inventory: inv,
	}

	return fc, nil
}

// Reconcile reconciles target infrastructure.
func (fctx *FlowContext) Reconcile(ctx context.Context) error {
	graph := fctx.buildReconcileGraph()
	fl := graph.Compile()
	if err := fl.Run(ctx, flow.Opts{
		Log: fctx.log,
	}); err != nil {
		// even if the run ends with an error we should still update our state.
		err = flow.Causes(err)
		fctx.log.Error(err, "flow reconciliation failed")
		return errors.Join(err, fctx.persistState(ctx))
	}

	status, err := fctx.GetInfrastructureStatus(ctx)
	state := fctx.GetInfrastructureState()
	egressCidrs := fctx.GetEgressIpCidrs()
	if err != nil {
		return err
	}
	return infrainternal.PatchProviderStatusAndState(ctx, fctx.client, fctx.infra, status, state, egressCidrs)
}

func (fctx *FlowContext) buildReconcileGraph() *flow.Graph {
	fctx.BasicFlowContext = shared.NewBasicFlowContext().WithSpan().WithLogger(fctx.log).WithPersist(fctx.persistState)
	g := flow.NewGraph("Azure infrastructure reconciliation")

	resourceGroup, vnet := fctx.addCommonReconcileTasks(g)

	if helper.IsUsingUserManagedEgress(fctx.cfg) {
		fctx.addUserManagedEgressReconcileTasks(g, vnet)
	} else {
		fctx.addManagedReconcileTasks(g, resourceGroup, vnet)
	}
	return g
}

// addCommonReconcileTasks registers the tasks that both managed-mode and BYO-subnet shoots share:
// the shoot's resource group, the VNet (managed or BYO), and the optional managed identity. It
// returns the resource-group and VNet task IDs so the mode-specific builders can wire further
// dependencies on top of them.
func (fctx *FlowContext) addCommonReconcileTasks(g *flow.Graph) (resourceGroup, vnet flow.TaskIDer) {
	resourceGroup = fctx.AddTask(g, "ensure resource group",
		fctx.EnsureResourceGroup, shared.Timeout(defaultTimeout))

	vnet = fctx.AddTask(g, "ensure vnet",
		fctx.EnsureVirtualNetwork, shared.Timeout(defaultTimeout), shared.Dependencies(resourceGroup))

	_ = fctx.AddTask(g, "ensure managed identity",
		fctx.EnsureManagedIdentity, shared.DoIf(fctx.cfg.Identity != nil))

	return resourceGroup, vnet
}

// addManagedReconcileTasks registers the tasks that create the Gardener-managed network stack:
// route table, worker network security group, worker public IPs, NAT gateway (if enabled), and
// the worker subnet(s). All of these are skipped in BYO-subnet mode.
func (fctx *FlowContext) addManagedReconcileTasks(g *flow.Graph, resourceGroup, vnet flow.TaskIDer) {
	routeTable := fctx.AddTask(g, "ensure route table",
		fctx.EnsureRouteTable, shared.Timeout(defaultTimeout), shared.Dependencies(resourceGroup))

	securityGroup := fctx.AddTask(g, "ensure security group",
		fctx.EnsureSecurityGroup, shared.Timeout(defaultTimeout), shared.Dependencies(resourceGroup))

	ip := fctx.AddTask(g, "ensure public IPs",
		fctx.EnsurePublicIps, shared.Timeout(defaultLongTimeout), shared.Dependencies(resourceGroup))

	nat := fctx.AddTask(g, "ensure nats",
		fctx.EnsureNatGateways, shared.Timeout(defaultLongTimeout), shared.Dependencies(resourceGroup, ip))

	_ = fctx.AddTask(g, "ensure subnets", fctx.EnsureSubnets,
		shared.Timeout(defaultLongTimeout), shared.Dependencies(vnet, routeTable, securityGroup, nat))
}

// addUserManagedEgressReconcileTasks registers the tasks that support user-managed-egress mode: a
// read-only discovery pass over the BYO subnet that populates status with the discovered NSG and
// route table. No mutating call is ever issued against the user's network resources.
func (fctx *FlowContext) addUserManagedEgressReconcileTasks(g *flow.Graph, vnet flow.TaskIDer) {
	_ = fctx.AddTask(g, "ensure BYO subnet (read-only discovery)",
		fctx.EnsureUserSubnet, shared.Timeout(defaultTimeout), shared.Dependencies(vnet))
}

// Delete deletes all resources managed by the reconciler
func (fctx *FlowContext) Delete(ctx context.Context) error {
	if len(fctx.state.ManagedItems) == 0 {
		// special case where the credentials were invalid from the beginning
		if _, ok := fctx.state.Data[CreatedResourcesExistKey]; !ok {
			fctx.log.Info("No created resources found. Skipping deletion.")
			return nil
		}
	}

	fctx.BasicFlowContext = shared.NewBasicFlowContext().WithSpan().WithLogger(fctx.log).WithPersist(fctx.persistState)
	managedVnet := fctx.adapter.VirtualNetworkConfig().Managed
	byo := helper.IsUsingUserManagedEgress(fctx.cfg)
	g := flow.NewGraph("Azure infrastructure deletion")

	loadBalancers := fctx.AddTask(g, "delete load balancers",
		fctx.DeleteLoadBalancers, shared.Timeout(defaultLongTimeout), shared.DoIf(!managedVnet))
	foreignSubnets := fctx.AddTask(g, "delete subnets in foreign resource group",
		fctx.DeleteSubnetsInForeignGroup, shared.Timeout(defaultLongTimeout),
		shared.Dependencies(loadBalancers), shared.DoIf(!managedVnet && !byo))

	fctx.AddTask(g, "delete resource group",
		fctx.DeleteResourceGroup, shared.Dependencies(foreignSubnets), shared.Timeout(defaultLongTimeout))

	fl := g.Compile()
	if err := fl.Run(ctx, flow.Opts{}); err != nil {
		return flow.Causes(err)
	}

	return nil
}

func (fctx *FlowContext) persistState(ctx context.Context) error {
	return infrainternal.PatchProviderStatusAndState(ctx, fctx.client, fctx.infra, nil, fctx.GetInfrastructureState(), fctx.GetEgressIpCidrs())
}
