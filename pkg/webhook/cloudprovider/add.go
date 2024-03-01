// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprovider

import (
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

var (
	logger = log.Log.WithName("azure-cloudprovider-webhook")
)

func init() {
	var err error
	utilruntime.Must(err)
}

// AddToManager creates the cloudprovider webhook and adds it to the manager.
func AddToManager(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	logger.Info("adding webhook to manager")

	return cloudprovider.New(mgr, cloudprovider.Args{
		Provider:             azure.Type,
		Mutator:              cloudprovider.NewMutator(mgr, logger, NewEnsurer(mgr, logger)),
		EnableObjectSelector: true,
	})
}
