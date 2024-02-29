// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network

import (
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/shoot"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	// WebhookName is the webhook name.
	WebhookName = "acceleratednetwork"
)

var logger = log.Log.WithName("networking-calico-accelerated-webhook")

// AddToManager creates a new topology webhook.
func AddToManager(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	logger.Info("Adding webhook to manager")
	return shoot.New(mgr, shoot.Args{
		Types: []extensionswebhook.Type{
			{Obj: &appsv1.DaemonSet{}},
		},
		Mutator: NewMutator(mgr, logger),
	})
}
