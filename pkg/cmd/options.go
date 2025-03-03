// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	extensionsbackupbucketcontroller "github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	extensionsbackupentrycontroller "github.com/gardener/gardener/extensions/pkg/controller/backupentry"
	extensionsbastioncontroller "github.com/gardener/gardener/extensions/pkg/controller/bastion"
	controllercmd "github.com/gardener/gardener/extensions/pkg/controller/cmd"
	extensionscontrolplanecontroller "github.com/gardener/gardener/extensions/pkg/controller/controlplane"
	extensionsdnsrecordcontroller "github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"
	extensionshealthcheckcontroller "github.com/gardener/gardener/extensions/pkg/controller/healthcheck"
	extensionsheartbeatcontroller "github.com/gardener/gardener/extensions/pkg/controller/heartbeat"
	extensionsinfrastructurecontroller "github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	extensionsworkercontroller "github.com/gardener/gardener/extensions/pkg/controller/worker"
	extensionscloudproviderwebhook "github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	webhookcmd "github.com/gardener/gardener/extensions/pkg/webhook/cmd"
	extensionscontrolplanewebhook "github.com/gardener/gardener/extensions/pkg/webhook/controlplane"
	extensionsnetworkwebhook "github.com/gardener/gardener/extensions/pkg/webhook/network"

	backupbucketcontroller "github.com/gardener/gardener-extension-provider-azure/pkg/controller/backupbucket"
	backupentrycontroller "github.com/gardener/gardener-extension-provider-azure/pkg/controller/backupentry"
	bastioncontroller "github.com/gardener/gardener-extension-provider-azure/pkg/controller/bastion"
	controlplanecontroller "github.com/gardener/gardener-extension-provider-azure/pkg/controller/controlplane"
	dnsrecordcontroller "github.com/gardener/gardener-extension-provider-azure/pkg/controller/dnsrecord"
	healthcheckcontroller "github.com/gardener/gardener-extension-provider-azure/pkg/controller/healthcheck"
	infrastructurecontroller "github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure"
	workercontroller "github.com/gardener/gardener-extension-provider-azure/pkg/controller/worker"
	acceleratednetworkwebhook "github.com/gardener/gardener-extension-provider-azure/pkg/webhook/acceleratednetwork"
	cloudproviderwebhook "github.com/gardener/gardener-extension-provider-azure/pkg/webhook/cloudprovider"
	controlplanewebhook "github.com/gardener/gardener-extension-provider-azure/pkg/webhook/controlplane"
	haNamespace "github.com/gardener/gardener-extension-provider-azure/pkg/webhook/highavailability/namespace"
	infrastructurewebhook "github.com/gardener/gardener-extension-provider-azure/pkg/webhook/infrastructure"
	networkwebhook "github.com/gardener/gardener-extension-provider-azure/pkg/webhook/network"
	seedproviderwebhook "github.com/gardener/gardener-extension-provider-azure/pkg/webhook/seedprovider"
	terraformerwebhook "github.com/gardener/gardener-extension-provider-azure/pkg/webhook/terraformer"
	"github.com/gardener/gardener-extension-provider-azure/pkg/webhook/topology"
)

// ControllerSwitchOptions are the controllercmd.SwitchOptions for the provider controllers.
func ControllerSwitchOptions() *controllercmd.SwitchOptions {
	return controllercmd.NewSwitchOptions(
		controllercmd.Switch(extensionsbackupbucketcontroller.ControllerName, backupbucketcontroller.AddToManager),
		controllercmd.Switch(extensionsbackupentrycontroller.ControllerName, backupentrycontroller.AddToManager),
		controllercmd.Switch(extensionsbastioncontroller.ControllerName, bastioncontroller.AddToManager),
		controllercmd.Switch(extensionscontrolplanecontroller.ControllerName, controlplanecontroller.AddToManager),
		controllercmd.Switch(extensionsdnsrecordcontroller.ControllerName, dnsrecordcontroller.AddToManager),
		controllercmd.Switch(extensionsinfrastructurecontroller.ControllerName, infrastructurecontroller.AddToManager),
		controllercmd.Switch(extensionsworkercontroller.ControllerName, workercontroller.AddToManager),
		controllercmd.Switch(extensionshealthcheckcontroller.ControllerName, healthcheckcontroller.AddToManager),
		controllercmd.Switch(extensionsheartbeatcontroller.ControllerName, extensionsheartbeatcontroller.AddToManager),
	)
}

// WebhookSwitchOptions are the webhookcmd.SwitchOptions for the provider webhooks.
func WebhookSwitchOptions() *webhookcmd.SwitchOptions {
	return webhookcmd.NewSwitchOptions(
		webhookcmd.Switch(extensionsnetworkwebhook.WebhookName, networkwebhook.AddToManager),
		webhookcmd.Switch(acceleratednetworkwebhook.WebhookName, acceleratednetworkwebhook.AddToManager),
		webhookcmd.Switch(infrastructurewebhook.WebhookName, infrastructurewebhook.AddToManager),
		webhookcmd.Switch(extensionscontrolplanewebhook.WebhookName, controlplanewebhook.AddToManager),
		webhookcmd.Switch(extensionscontrolplanewebhook.SeedProviderWebhookName, seedproviderwebhook.AddToManager),
		webhookcmd.Switch(extensionscloudproviderwebhook.WebhookName, cloudproviderwebhook.AddToManager),
		webhookcmd.Switch(topology.WebhookName, topology.AddToManager),
		webhookcmd.Switch(haNamespace.WebhookName, haNamespace.AddToManager),
		webhookcmd.Switch(terraformerwebhook.WebhookName, terraformerwebhook.AddToManager),
	)
}
