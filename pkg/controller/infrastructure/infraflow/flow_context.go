//  Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package infraflow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow/shared"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

const (
	defaultTimeout     = 2 * time.Minute
	defaultLongTimeout = 4 * time.Minute
)

// PersistStateFunc is a callback function that is used to persist the state during the reconciliation.
type PersistStateFunc func(ctx context.Context, state *runtime.RawExtension) error

// FlowContext is the reconciler for all managed resources
type FlowContext struct {
	*shared.BasicFlowContext
	logger logr.Logger

	persistFunc PersistStateFunc
	cfg         *azure.InfrastructureConfig
	factory     client.Factory
	auth        *internal.ClientAuth
	infra       *extensionsv1alpha1.Infrastructure
	state       *azure.InfrastructureState
	cluster     *controller.Cluster
	whiteboard  shared.Whiteboard
	adapter     *InfrastructureAdapter
	provider    Access
	inventory   *SimpleInventory
}

// NewFlowContext creates a new FlowContext.
func NewFlowContext(factory client.Factory,
	auth *internal.ClientAuth,
	logger logr.Logger,
	infra *extensionsv1alpha1.Infrastructure,
	cluster *controller.Cluster,
	state *azure.InfrastructureState,
	persistFunc PersistStateFunc,
) (*FlowContext, error) {
	wb := shared.NewWhiteboard()
	for k, v := range state.Data {
		wb.Set(k, v)
	}

	cfg, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return nil, err
	}

	profile, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return nil, err
	}

	// var status *azure.InfrastructureStatus
	// if infra.Status.ProviderStatus != nil {
	// 	status, err = helper.InfrastructureStatusFromRaw(infra.Status.ProviderStatus)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }

	inv := NewSimpleInventory()
	for _, r := range state.Items {
		if err := inv.Insert(r.ID); err != nil {
			return nil, err
		}
	}

	adapter, err := NewInfrastructureAdapter(
		infra,
		cfg,
		state,
		profile,
		cluster,
	)
	if err != nil {
		return nil, err
	}

	fc := &FlowContext{
		BasicFlowContext: shared.NewBasicFlowContext(logger, wb, nil),
		factory:          factory,
		auth:             auth,
		logger:           logger,
		infra:            infra,
		state:            state,
		cluster:          cluster,
		cfg:              cfg,
		whiteboard:       wb,
		provider: &access{
			factory,
		},
		adapter:   adapter,
		inventory: inv,
	}

	if persistFunc != nil {
		fc.persistFunc = persistFunc
		fc.BasicFlowContext = shared.NewBasicFlowContext(logger, wb, fc.persist)
	}
	return fc, nil
}

// Reconcile reconciles target infrastructure.
func (f *FlowContext) Reconcile(ctx context.Context) (*v1alpha1.InfrastructureStatus, *runtime.RawExtension, error) {
	defer func() {
		if r := recover(); r != nil {
			err, ok := r.(error)
			if !ok {
				err = fmt.Errorf("panic: %v", r)
			}
			f.LogFromContext(ctx).Error(err, "recovered from panic")
		}
	}()

	graph := f.buildReconcileGraph()
	fl := graph.Compile()
	if err := fl.Run(ctx, flow.Opts{
		Log:              f.Log,
		ProgressReporter: nil,
		ErrorCleaner:     nil,
		ErrorContext:     nil,
	}); err != nil {
		// even if the run ends with an error we should still update our state.
		err = flow.Causes(err)
		f.forceGen()
		errInternal := f.PersistState(ctx, true)
		return nil, nil, errors.Join(err, errInternal)
	}

	status, err := f.GetInfrastructureStatus(ctx)
	state, err2 := f.GetInfrastructureState()
	err = errors.Join(err, err2)
	return status, state, err
}

func (f *FlowContext) buildReconcileGraph() *flow.Graph {
	g := flow.NewGraph("Azure infrastructure reconciliation")
	resourceGroup := f.AddTask(g, "ensure resource group",
		f.EnsureResourceGroup, shared.Timeout(defaultTimeout))

	vnet := f.AddTask(g, "ensure vnet",
		f.EnsureVirtualNetwork, shared.Timeout(defaultTimeout), shared.Dependencies(resourceGroup))

	_ = f.AddTask(g, "ensure availability set",
		f.EnsureAvailabilitySet, shared.DoIf(f.adapter.AvailabilitySetConfig() != nil),
		shared.Timeout(defaultTimeout), shared.Dependencies(resourceGroup))

	_ = f.AddTask(g, "ensure managed identity",
		f.EnsureManagedIdentity, shared.DoIf(f.cfg.Identity != nil))

	routeTable := f.AddTask(g, "ensure route table",
		f.EnsureRouteTable, shared.Timeout(defaultTimeout), shared.Dependencies(resourceGroup))

	securityGroup := f.AddTask(g, "ensure security group",
		f.EnsureSecurityGroup, shared.Timeout(defaultTimeout), shared.Dependencies(resourceGroup))

	ip := f.AddTask(g, "ensure public IPs",
		f.EnsurePublicIps, shared.Timeout(defaultLongTimeout), shared.Dependencies(resourceGroup))
	nat := f.AddTask(g, "ensure nats",
		f.EnsureNatGateways, shared.Timeout(defaultLongTimeout), shared.Dependencies(resourceGroup, ip))

	_ = f.AddTask(g, "ensure subnets", f.EnsureSubnets,
		shared.Timeout(defaultLongTimeout), shared.Dependencies(vnet, routeTable, securityGroup, nat))
	return g
}

// Delete deletes all resources managed by the reconciler
func (f *FlowContext) Delete(ctx context.Context) error {
	if len(f.state.Items) == 0 {
		// special case where the credentials were invalid from the beginning
		if _, ok := f.state.Data[CreatedResourcesExistKey]; !ok {
			return nil
		}
	}

	g := flow.NewGraph("Azure infrastructure deletion")

	foreignSubnets := f.AddTask(g, "delete subnets in foreign resource group",
		f.DeleteSubnetsInForeignGroup, shared.Timeout(defaultLongTimeout))
	f.AddTask(g, "delete resource group",
		f.DeleteResourceGroup, shared.Dependencies(foreignSubnets), shared.Timeout(defaultLongTimeout))

	fl := g.Compile()
	if err := fl.Run(ctx, flow.Opts{}); err != nil {
		return flow.Causes(err)
	}

	return nil
}

// persist is an implementations of the BasicFlowContext's persistFunc.
func (f *FlowContext) persist(ctx context.Context, _ shared.FlatMap) error {
	state, err := f.GetInfrastructureState()
	if err != nil {
		return err
	}
	return f.persistFunc(ctx, state)
}

func (f *FlowContext) forceGen() {
	f.whiteboard.Set("time", time.Now().String())
}
