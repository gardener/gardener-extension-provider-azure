// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"errors"
	"fmt"
	"time"

	extensionsconfigv1alpha1 "github.com/gardener/gardener/extensions/pkg/apis/config/v1alpha1"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	azurev1alpha1 "github.com/gardener/remedy-controller/pkg/apis/azure/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-azure/pkg/features"
)

const (
	// GracefulDeletionWaitInterval is the default interval for retry operations.
	GracefulDeletionWaitInterval = 1 * time.Minute
	// GracefulDeletionTimeout is the timeout that defines how long the actuator should wait for remedy controller resources to be deleted
	// gracefully by the remedy controller itself
	GracefulDeletionTimeout       = 10 * time.Minute
	publicIpFinalizerString       = "azure.remedy.gardener.cloud/publicipaddress"
	virtualMachineFinalizerString = "azure.remedy.gardener.cloud/virtualmachine"
	serviceFinalizerName          = "azure.remedy.gardener.cloud/service"
)

// NewActuator creates a new Actuator that acts upon and updates the status of ControlPlane resources.
func NewActuator(
	mgr manager.Manager,
	a controlplane.Actuator,
	gracefulDeletionTimeout time.Duration,
	gracefulDeletionWaitInterval time.Duration,
) controlplane.Actuator {
	return &actuator{
		Actuator:                     a,
		client:                       mgr.GetClient(),
		gracefulDeletionTimeout:      gracefulDeletionTimeout,
		gracefulDeletionWaitInterval: gracefulDeletionWaitInterval,
	}
}

// actuator is an Actuator that acts upon and updates the status of ControlPlane resources.
type actuator struct {
	controlplane.Actuator
	client                       client.Client
	gracefulDeletionTimeout      time.Duration
	gracefulDeletionWaitInterval time.Duration
}

func (a *actuator) Reconcile(
	ctx context.Context,
	log logr.Logger,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (bool, error) {
	// Call Reconcile on the composed Actuator
	ok, err := a.Actuator.Reconcile(ctx, log, cp, cluster)
	if err != nil {
		return ok, err
	}

	// Nothing to do if the feature is disabled
	if features.ExtensionFeatureGate.Enabled(features.DisableRemedyController) {
		return ok, a.forceDeleteRemedyControllerResources(ctx, log, cp.GetNamespace(), cluster)
	}
	return ok, nil
}

// Delete reconciles the given controlplane and cluster, deleting the additional
// control plane components as needed.
// Before delegating to the composed Actuator, it ensures that all remedy controller resources have been deleted gracefully.
func (a *actuator) Delete(
	ctx context.Context,
	log logr.Logger,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) error {
	list := &azurev1alpha1.PublicIPAddressList{}
	if err := a.client.List(ctx, list, client.InNamespace(cp.Namespace)); err != nil {
		return err
	}
	if meta.LenList(list) != 0 {
		if time.Since(cp.DeletionTimestamp.Time) <= a.gracefulDeletionTimeout {
			log.Info("Some publicipaddresses still exist. Deletion will be retried ...")
			return &reconcilerutils.RequeueAfterError{
				RequeueAfter: a.gracefulDeletionWaitInterval,
			}
		} else {
			log.Info("The timeout for waiting for publicipaddresses to be gracefully deleted has expired. They will be forcefully removed.")
		}
	}

	// Call Delete on the composed Actuator
	if err := a.Actuator.Delete(ctx, log, cp, cluster); err != nil {
		return err
	}

	if features.ExtensionFeatureGate.Enabled(features.DisableRemedyController) {
		return a.forceDeleteRemedyControllerResources(ctx, log, cp.GetNamespace(), cluster)
	}
	// Delete all remaining remedy controller resources
	return a.forceDeleteSeedRemedyControllerResources(ctx, log, cp.GetNamespace())
}

// ForceDelete forcefully deletes the controlplane.
func (a *actuator) ForceDelete(
	ctx context.Context,
	log logr.Logger,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) error {
	// Call Delete on the composed Actuator
	if err := a.Actuator.Delete(ctx, log, cp, cluster); err != nil {
		return err
	}
	// Delete all remaining remedy controller resources
	return a.forceDeleteSeedRemedyControllerResources(ctx, log, cp.GetNamespace())
}

// Migrate reconciles the given controlplane and cluster, migrating the additional
// control plane components as needed.
// Before delegating to the composed Actuator, it ensures that all remedy controller resources have been deleted.
func (a *actuator) Migrate(
	ctx context.Context,
	log logr.Logger,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) error {
	// Call Migrate on the composed Actuator so that the controlplane chart is deleted and therefore
	// the remedy controller is also removed.
	if err := a.Actuator.Migrate(ctx, log, cp, cluster); err != nil {
		return err
	}
	if features.ExtensionFeatureGate.Enabled(features.DisableRemedyController) {
		return a.forceDeleteRemedyControllerResources(ctx, log, cp.GetNamespace(), cluster)
	}
	// Delete all remaining remedy controller resources
	return a.forceDeleteSeedRemedyControllerResources(ctx, log, cp.GetNamespace())
}

func (a *actuator) forceDeleteSeedRemedyControllerResources(ctx context.Context, log logr.Logger, namespace string) error {
	log.Info("Removing finalizers from remedy controller resources")
	pubipList := &azurev1alpha1.PublicIPAddressList{}
	if err := a.client.List(ctx, pubipList, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("could not list publicipaddresses: %w", err)
	}
	for _, pubip := range pubipList.Items {
		if controllerutil.ContainsFinalizer(&pubip, publicIpFinalizerString) {
			if err := controllerutils.RemoveFinalizers(ctx, a.client, &pubip, publicIpFinalizerString); err != nil {
				return fmt.Errorf("could not remove finalizers from publicipaddress: %w", err)
			}
		}
	}

	virtualMachineList := &azurev1alpha1.VirtualMachineList{}
	if err := a.client.List(ctx, virtualMachineList, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("could not list virtualmachines: %w", err)
	}
	for _, virtualMachine := range virtualMachineList.Items {
		if controllerutil.ContainsFinalizer(&virtualMachine, virtualMachineFinalizerString) {
			if err := controllerutils.RemoveFinalizers(ctx, a.client, &virtualMachine, virtualMachineFinalizerString); err != nil {
				return fmt.Errorf("could not remove finalizers from virtualmachine: %w", err)
			}
		}
	}

	log.Info("Deleting all remaining remedy controller resources")
	if err := a.client.DeleteAllOf(ctx, &azurev1alpha1.PublicIPAddress{}, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("could not delete publicipaddress resources: %w", err)
	}
	if err := a.client.DeleteAllOf(ctx, &azurev1alpha1.VirtualMachine{}, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("could not delete virtualmachine resources: %w", err)
	}

	return nil
}

func (a *actuator) forceDeleteShootRemedyControllerResources(ctx context.Context, log logr.Logger, namespace string, cluster *extensionscontroller.Cluster) error {
	// Nothing to do  if the cluster is hibernated
	if extensionscontroller.IsHibernated(cluster) {
		return nil
	}

	pubipList := &azurev1alpha1.PublicIPAddressList{}
	if err := a.client.List(ctx, pubipList, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("could not list publicipaddresses: %w", err)
	}

	log.Info("Removing finalizers from publicipaddresses")
	if len(pubipList.Items) == 0 {
		return nil
	}

	_, shootClient, err := util.NewClientForShoot(ctx, a.client, cluster.Shoot.GetName(), client.Options{}, extensionsconfigv1alpha1.RESTOptions{})
	if err != nil {
		// No need to report the error as this is anyway only best effort. Some scenarios, e.g. self hosted shoot clusters,
		// might not have the gardener secret and hence cannot construct the shoot client here.
		log.Info("Could not create shoot client to check for existing remedy controller resources", "error", err.Error())
		return err
	}

	lbs := &corev1.ServiceList{}
	var errs error

	for {
		if err := shootClient.List(ctx, lbs, client.Limit(100), client.Continue(lbs.GetContinue())); err != nil {
			log.Info("Could not list services in shoot cluster to check for existing remedy controller resources", "error", err)
			return err
		}

		for _, lb := range lbs.Items {
			if lb.Spec.Type != corev1.ServiceTypeLoadBalancer {
				continue
			}
			if controllerutil.ContainsFinalizer(&lb, serviceFinalizerName) {
				if retryErr := retry.RetryOnConflict(retry.DefaultRetry,
					func() error {
						err := shootClient.Get(ctx, client.ObjectKey{Namespace: lb.Namespace, Name: lb.Name}, &lb)
						if err != nil {
							return err
						}
						return controllerutils.RemoveFinalizers(ctx, shootClient, &lb, serviceFinalizerName)
					}); retryErr != nil {
					log.Info("Could not remove remedy finalizer from service", "namespace", lb.Namespace, "name", lb.Name, "error", retryErr)
					errs = errors.Join(errs, retryErr)
					continue
				}
			}
		}
		if lbs.GetContinue() == "" {
			break
		}
	}
	return errs
}

func (a *actuator) forceDeleteRemedyControllerResources(ctx context.Context, log logr.Logger, namespace string, cluster *extensionscontroller.Cluster) error {
	err := a.forceDeleteShootRemedyControllerResources(ctx, log, namespace, cluster)
	if err != nil {
		return err
	}
	return a.forceDeleteSeedRemedyControllerResources(ctx, log, namespace)
}
