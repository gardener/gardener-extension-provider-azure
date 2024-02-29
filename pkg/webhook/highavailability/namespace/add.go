// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package namespace

import (
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

const (
	// WebhookName the name of the topology webhook.
	WebhookName = "high-availability-namespace"
	webhookPath = "high-availability/namespace"
)

var (
	logger = log.Log.WithName("high-availability-namespace-webhook")
	// SeedRegion is the region where the seed is located.
	SeedRegion = ""
	// SeedProvider is the provider type of the seed.
	SeedProvider = ""
)

// AddOptions contains the configuration options for the topology webhook.
type AddOptions struct {
	// SeedRegion is the region where the seed is located.
	SeedRegion string
	// SeedProvider is the provider type of the seed.
	SeedProvider string
}

// AddToManager adds the webhook to the manager.
func AddToManager(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	return AddToManagerWithOpts(mgr, AddOptions{
		SeedRegion:   SeedRegion,
		SeedProvider: SeedProvider,
	})
}

// AddToManagerWithOpts creates the webhook with the given opts and adds that to the manager.
func AddToManagerWithOpts(mgr manager.Manager, options AddOptions) (*extensionswebhook.Webhook, error) {
	logger.Info("Adding webhook to manager")
	types := []extensionswebhook.Type{
		{Obj: &corev1.Namespace{}},
	}

	logger.Info("Creating webhook")
	return &extensionswebhook.Webhook{
		Name:     WebhookName,
		Provider: azure.Type,
		Path:     webhookPath,
		Target:   extensionswebhook.TargetSeed,
		Types:    types,
		Webhook:  &admission.Webhook{Handler: New(admission.NewDecoder(mgr.GetScheme()), logger, options), RecoverPanic: true},
		Selector: &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      v1beta1constants.GardenRole,
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{v1beta1constants.GardenRoleShoot},
				},
				{
					Key:      v1alpha1.HighAvailabilityConfigConsider,
					Operator: metav1.LabelSelectorOpExists,
				},
			},
		},
	}, nil
}
