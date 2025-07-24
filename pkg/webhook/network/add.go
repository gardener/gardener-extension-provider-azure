// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network

import (
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/network"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

var logger = log.Log.WithName("networking-calico-webhook")

// AddToManager creates a new topology webhook.
func AddToManager(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	logger.Info("Adding webhook to manager")
	return network.New(mgr, network.Args{
		CloudProvider:   azure.Type,
		NetworkProvider: "calico",
		Types: []extensionswebhook.Type{
			{Obj: &extensionsv1alpha1.Network{}},
		},
		Mutator: network.NewMutator(mgr, logger, mutateNetworkConfig),
	})
}
