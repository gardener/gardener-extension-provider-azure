// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	"github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	"github.com/gardener/gardener/extensions/pkg/util"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardener "github.com/gardener/gardener/pkg/client/kubernetes"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

type delegateFactory struct {
	seedClient   client.Client
	restConfig   *rest.Config
	scheme       *runtime.Scheme
	gardenReader client.Reader
}

// NewActuator creates a new Actuator that updates the status of the handled WorkerPoolConfigs.
func NewActuator(mgr manager.Manager, gardenCluster cluster.Cluster) worker.Actuator {
	var (
		workerDelegate = &delegateFactory{
			seedClient:   mgr.GetClient(),
			restConfig:   mgr.GetConfig(),
			scheme:       mgr.GetScheme(),
			gardenReader: gardenCluster.GetAPIReader(),
		}
	)

	return genericactuator.NewActuator(
		mgr,
		gardenCluster,
		workerDelegate,
		func(err error) []gardencorev1beta1.ErrorCode {
			return util.DetermineErrorCodes(err, helper.KnownCodes)
		},
	)
}

func (d *delegateFactory) WorkerDelegate(ctx context.Context, worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster) (genericactuator.WorkerDelegate, error) {
	clientset, err := kubernetes.NewForConfig(d.restConfig)
	if err != nil {
		return nil, err
	}

	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	seedChartApplier, err := gardener.NewChartApplierForConfig(d.restConfig)
	if err != nil {
		return nil, err
	}
	factory, err := azureclient.NewAzureClientFactory(ctx, d.seedClient, worker.Spec.SecretRef)
	if err != nil {
		return nil, err
	}

	return NewWorkerDelegate(d.seedClient, d.scheme, seedChartApplier, serverVersion.GitVersion, worker, cluster, factory)
}

type workerDelegate struct {
	client         client.Client
	scheme         *runtime.Scheme
	decoder        runtime.Decoder
	lenientDecoder runtime.Decoder

	seedChartApplier gardener.ChartApplier
	serverVersion    string

	cloudProfileConfig *api.CloudProfileConfig
	cluster            *extensionscontroller.Cluster
	worker             *extensionsv1alpha1.Worker

	machineClasses     []map[string]interface{}
	machineDeployments worker.MachineDeployments
	machineImages      []api.MachineImage

	clientFactory azureclient.Factory
}

// NewWorkerDelegate creates a new context for a worker reconciliation.
func NewWorkerDelegate(
	client client.Client,
	scheme *runtime.Scheme,
	seedChartApplier gardener.ChartApplier,
	serverVersion string,
	worker *extensionsv1alpha1.Worker,
	cluster *extensionscontroller.Cluster,
	factory azureclient.Factory,
) (genericactuator.WorkerDelegate, error) {
	config, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return nil, err
	}

	return &workerDelegate{
		client:         client,
		scheme:         scheme,
		decoder:        serializer.NewCodecFactory(scheme, serializer.EnableStrict).UniversalDecoder(),
		lenientDecoder: serializer.NewCodecFactory(scheme).UniversalDecoder(),

		seedChartApplier: seedChartApplier,
		serverVersion:    serverVersion,

		cloudProfileConfig: config,
		cluster:            cluster,
		worker:             worker,

		clientFactory: factory,
	}, nil
}
