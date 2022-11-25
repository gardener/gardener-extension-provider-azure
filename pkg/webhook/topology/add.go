//  Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
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

package topology

import (
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

const (
	// WebhookName the name of the topology webhook.
	WebhookName = "topology"
	webhookPath = "topology"
)

var logger = log.Log.WithName("azure-topology-webhook")

// AddToManager creates a webhook adds the webhook to the manager.
func AddToManager(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	logger.Info("Adding webhook to manager")

	types := []extensionswebhook.Type{
		{Obj: &v1.Pod{}},
	}

	handler, err := extensionswebhook.NewBuilder(mgr, logger).WithMutator(New(logger), types...).Build()
	if err != nil {
		return nil, err
	}

	logger.Info("Creating webhook")
	return &extensionswebhook.Webhook{
		Name:     WebhookName,
		Provider: azure.Type,
		Path:     webhookPath,
		Target:   extensionswebhook.TargetSeed,
		Types:    types,
		Webhook:  &admission.Webhook{Handler: handler, RecoverPanic: true},
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				v1beta1constants.LabelSeedProvider: azure.Type,
			},
		},
		ObjectSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot,
			},
		},
	}, nil
}
