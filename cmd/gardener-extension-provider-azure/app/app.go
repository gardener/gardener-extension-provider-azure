// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"os"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	"github.com/gardener/gardener/extensions/pkg/controller"
	controllercmd "github.com/gardener/gardener/extensions/pkg/controller/cmd"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	"github.com/gardener/gardener/extensions/pkg/controller/heartbeat"
	heartbeatcmd "github.com/gardener/gardener/extensions/pkg/controller/heartbeat/cmd"
	"github.com/gardener/gardener/extensions/pkg/util"
	webhookcmd "github.com/gardener/gardener/extensions/pkg/webhook/cmd"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	azurev1alpha1 "github.com/gardener/remedy-controller/pkg/apis/azure/v1alpha1"
	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/spf13/cobra"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/component-base/version/verflag"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	azureinstall "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/install"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	azurecmd "github.com/gardener/gardener-extension-provider-azure/pkg/cmd"
	azurebackupbucket "github.com/gardener/gardener-extension-provider-azure/pkg/controller/backupbucket"
	azurebackupentry "github.com/gardener/gardener-extension-provider-azure/pkg/controller/backupentry"
	azurebastion "github.com/gardener/gardener-extension-provider-azure/pkg/controller/bastion"
	azurecontrolplane "github.com/gardener/gardener-extension-provider-azure/pkg/controller/controlplane"
	azurednsrecord "github.com/gardener/gardener-extension-provider-azure/pkg/controller/dnsrecord"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/healthcheck"
	azureinfrastructure "github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure"
	azureworker "github.com/gardener/gardener-extension-provider-azure/pkg/controller/worker"
	"github.com/gardener/gardener-extension-provider-azure/pkg/features"
	haNamespace "github.com/gardener/gardener-extension-provider-azure/pkg/webhook/highavailability/namespace"
	azureseedprovider "github.com/gardener/gardener-extension-provider-azure/pkg/webhook/seedprovider"
	"github.com/gardener/gardener-extension-provider-azure/pkg/webhook/topology"
)

// NewControllerManagerCommand creates a new command for running a Azure provider controller.
func NewControllerManagerCommand(ctx context.Context) *cobra.Command {
	var (
		generalOpts = &controllercmd.GeneralOptions{}
		restOpts    = &controllercmd.RESTOptions{}
		mgrOpts     = &controllercmd.ManagerOptions{
			LeaderElection:          true,
			LeaderElectionID:        controllercmd.LeaderElectionNameID(azure.Name),
			LeaderElectionNamespace: os.Getenv("LEADER_ELECTION_NAMESPACE"),
			WebhookServerPort:       443,
			WebhookCertDir:          "/tmp/gardener-extensions-cert",
			MetricsBindAddress:      ":8080",
			HealthBindAddress:       ":8081",
		}
		configFileOpts = &azurecmd.ConfigOptions{}

		// options for the backupbucket controller
		backupBucketCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the backupentry controller
		backupEntryCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the bastion controller
		bastionCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the health care controller
		healthCheckCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the heartbeat controller
		heartbeatCtrlOpts = &heartbeatcmd.Options{
			ExtensionName:        azure.Name,
			RenewIntervalSeconds: 30,
			Namespace:            os.Getenv("LEADER_ELECTION_NAMESPACE"),
		}

		// options for the controlplane controller
		controlPlaneCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the csimigration controller
		csiMigrationCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the dnsrecord controller
		dnsRecordCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the infrastructure controller
		infraCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}
		reconcileOpts = &controllercmd.ReconcilerOptions{}

		// options for the worker controller
		workerCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the webhook server
		webhookServerOptions = &webhookcmd.ServerOptions{
			Namespace: os.Getenv("WEBHOOK_CONFIG_NAMESPACE"),
		}

		controllerSwitches = azurecmd.ControllerSwitchOptions()
		webhookSwitches    = azurecmd.WebhookSwitchOptions()
		webhookOptions     = webhookcmd.NewAddToManagerOptions(
			azure.Name,
			genericactuator.ShootWebhooksResourceName,
			genericactuator.ShootWebhookNamespaceSelector(azure.Type),
			webhookServerOptions,
			webhookSwitches,
		)

		seedOptions = &azurecmd.SeedConfigOptions{}

		aggOption = controllercmd.NewOptionAggregator(
			generalOpts,
			restOpts,
			mgrOpts,
			controllercmd.PrefixOption("backupbucket-", backupBucketCtrlOpts),
			controllercmd.PrefixOption("backupentry-", backupEntryCtrlOpts),
			controllercmd.PrefixOption("bastion-", bastionCtrlOpts),
			controllercmd.PrefixOption("controlplane-", controlPlaneCtrlOpts),
			controllercmd.PrefixOption("csimigration-", csiMigrationCtrlOpts),
			controllercmd.PrefixOption("dnsrecord-", dnsRecordCtrlOpts),
			controllercmd.PrefixOption("infrastructure-", infraCtrlOpts),
			controllercmd.PrefixOption("worker-", workerCtrlOpts),
			controllercmd.PrefixOption("healthcheck-", healthCheckCtrlOpts),
			controllercmd.PrefixOption("heartbeat-", heartbeatCtrlOpts),
			configFileOpts,
			controllerSwitches,
			reconcileOpts,
			webhookOptions,
			seedOptions,
		)
	)

	cmd := &cobra.Command{
		Use: fmt.Sprintf("%s-controller-manager", azure.Name),

		RunE: func(_ *cobra.Command, _ []string) error {
			verflag.PrintAndExitIfRequested()

			if err := aggOption.Complete(); err != nil {
				return fmt.Errorf("error completing options: %w", err)
			}

			if err := heartbeatCtrlOpts.Validate(); err != nil {
				return err
			}

			if err := features.ExtensionFeatureGate.SetFromMap(configFileOpts.Completed().Config.FeatureGates); err != nil {
				return err
			}

			util.ApplyClientConnectionConfigurationToRESTConfig(configFileOpts.Completed().Config.ClientConnection, restOpts.Completed().Config)

			mgr, err := manager.New(restOpts.Completed().Config, mgrOpts.Completed().Options())
			if err != nil {
				return fmt.Errorf("could not instantiate manager: %w", err)
			}

			scheme := mgr.GetScheme()
			if err := controller.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := azureinstall.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := druidcorev1alpha1.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := vpaautoscalingv1.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := machinev1alpha1.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := azurev1alpha1.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := monitoringv1.AddToScheme(mgr.GetScheme()); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}

			log := mgr.GetLogger()
			gardenCluster, err := getGardenCluster(log)
			if err != nil {
				return err
			}
			log.Info("Adding garden cluster to manager")
			if err := mgr.Add(gardenCluster); err != nil {
				return fmt.Errorf("failed adding garden cluster to manager: %w", err)
			}

			log.Info("Adding controllers to manager")
			configFileOpts.Completed().ApplyETCDStorage(&azureseedprovider.DefaultAddOptions.ETCDStorage)
			configFileOpts.Completed().ApplyHealthCheckConfig(&healthcheck.DefaultAddOptions.HealthCheckConfig)
			healthCheckCtrlOpts.Completed().Apply(&healthcheck.DefaultAddOptions.Controller)
			heartbeatCtrlOpts.Completed().Apply(&heartbeat.DefaultAddOptions)
			backupBucketCtrlOpts.Completed().Apply(&azurebackupbucket.DefaultAddOptions.Controller)
			backupEntryCtrlOpts.Completed().Apply(&azurebackupentry.DefaultAddOptions.Controller)
			bastionCtrlOpts.Completed().Apply(&azurebastion.DefaultAddOptions.Controller)
			controlPlaneCtrlOpts.Completed().Apply(&azurecontrolplane.DefaultAddOptions.Controller)
			dnsRecordCtrlOpts.Completed().Apply(&azurednsrecord.DefaultAddOptions.Controller)
			infraCtrlOpts.Completed().Apply(&azureinfrastructure.DefaultAddOptions.Controller)
			reconcileOpts.Completed().Apply(&azureinfrastructure.DefaultAddOptions.IgnoreOperationAnnotation, &azureinfrastructure.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&azurecontrolplane.DefaultAddOptions.IgnoreOperationAnnotation, &azurecontrolplane.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&azureworker.DefaultAddOptions.IgnoreOperationAnnotation, &azureworker.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&azurebastion.DefaultAddOptions.IgnoreOperationAnnotation, &azurebastion.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&azurebackupbucket.DefaultAddOptions.IgnoreOperationAnnotation, &azurebackupbucket.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&azurebackupentry.DefaultAddOptions.IgnoreOperationAnnotation, &azurebackupentry.DefaultAddOptions.ExtensionClass)
			reconcileOpts.Completed().Apply(&azurednsrecord.DefaultAddOptions.IgnoreOperationAnnotation, &azurednsrecord.DefaultAddOptions.ExtensionClass)
			workerCtrlOpts.Completed().Apply(&azureworker.DefaultAddOptions.Controller)
			azureworker.DefaultAddOptions.GardenCluster = gardenCluster
			azureworker.DefaultAddOptions.AutonomousShootCluster = generalOpts.Completed().AutonomousShootCluster

			topology.SeedRegion = seedOptions.Completed().Region
			topology.SeedProvider = seedOptions.Completed().Provider
			haNamespace.SeedRegion = seedOptions.Completed().Region
			haNamespace.SeedProvider = seedOptions.Completed().Provider

			shootWebhookConfig, err := webhookOptions.Completed().AddToManager(ctx, mgr, nil, generalOpts.Completed().AutonomousShootCluster)
			if err != nil {
				return fmt.Errorf("could not add webhooks to manager: %w", err)
			}
			azurecontrolplane.DefaultAddOptions.ShootWebhookConfig = shootWebhookConfig
			azurecontrolplane.DefaultAddOptions.WebhookServerNamespace = webhookOptions.Server.Namespace

			if err := controllerSwitches.Completed().AddToManager(ctx, mgr); err != nil {
				return fmt.Errorf("could not add controllers to manager: %w", err)
			}

			if err := mgr.AddReadyzCheck("informer-sync", gardenerhealthz.NewCacheSyncHealthz(mgr.GetCache())); err != nil {
				return fmt.Errorf("could not add readycheck for informers: %w", err)
			}

			if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
				return fmt.Errorf("could not add health check to manager: %w", err)
			}

			if err := mgr.AddReadyzCheck("webhook-server", mgr.GetWebhookServer().StartedChecker()); err != nil {
				return fmt.Errorf("could not add ready check for webhook server to manager: %w", err)
			}

			// TODO (georgibaltiev): Remove after the release of version 1.55.0
			if reconcileOpts.ExtensionClass != "garden" {
				log.Info("Adding migration runnables")
				if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
					return purgeMachineControllerManagerRBACResources(ctx, mgr.GetClient(), log)
				})); err != nil {
					return fmt.Errorf("error adding mcm migrations: %w", err)
				}
			}

			// TODO (kon-angelo): Remove after the release of version 1.46.0
			if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
				return purgeTerraformerRBACResources(ctx, mgr.GetClient(), log)
			})); err != nil {
				return fmt.Errorf("error adding terraformer migrations: %w", err)
			}

			if err := mgr.Start(ctx); err != nil {
				return fmt.Errorf("error running manager: %w", err)
			}

			return nil
		},
	}

	verflag.AddFlags(cmd.Flags())
	aggOption.AddFlags(cmd.Flags())

	return cmd
}

func getGardenCluster(log logr.Logger) (cluster.Cluster, error) {
	log.Info("Getting rest config for garden")
	gardenRESTConfig, err := kubernetes.RESTConfigFromKubeconfigFile(os.Getenv("GARDEN_KUBECONFIG"), kubernetes.AuthTokenFile)
	if err != nil {
		return nil, err
	}

	log.Info("Setting up cluster object for garden")
	gardenCluster, err := cluster.New(gardenRESTConfig, func(opts *cluster.Options) {
		opts.Scheme = kubernetes.GardenScheme
		opts.Logger = log
	})
	if err != nil {
		return nil, fmt.Errorf("failed creating garden cluster object: %w", err)
	}
	return gardenCluster, nil
}
