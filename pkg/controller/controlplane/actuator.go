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
	remedyctrl "github.com/gardener/remedy-controller/pkg/controller/azure/service"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	GracefulDeletionTimeout = 10 * time.Minute
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

	return ok, a.forceDeleteShootRemedyControllerResources(ctx, log, cluster)
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
	// Delete all remaining remedy controller resources
	return a.forceDeleteSeedRemedyControllerResources(ctx, log, cp)
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
	return a.forceDeleteSeedRemedyControllerResources(ctx, log, cp)
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
	// Delete all remaining remedy controller resources
	return a.forceDeleteSeedRemedyControllerResources(ctx, log, cp)
}

func (a *actuator) forceDeleteSeedRemedyControllerResources(ctx context.Context, log logr.Logger, cp *extensionsv1alpha1.ControlPlane) error {
	log.Info("Removing finalizers from remedy controller resources")
	pubipList := &azurev1alpha1.PublicIPAddressList{}
	if err := a.client.List(ctx, pubipList, client.InNamespace(cp.Namespace)); err != nil {
		return fmt.Errorf("could not list publicipaddresses: %w", err)
	}
	for _, pubip := range pubipList.Items {
		finalizerString := "azure.remedy.gardener.cloud/publicipaddress"
		if controllerutil.ContainsFinalizer(&pubip, finalizerString) {
			if err := controllerutils.RemoveFinalizers(ctx, a.client, &pubip, finalizerString); err != nil {
				return fmt.Errorf("could not remove finalizers from publicipaddress: %w", err)
			}
		}
	}

	virtualMachineList := &azurev1alpha1.VirtualMachineList{}
	if err := a.client.List(ctx, virtualMachineList, client.InNamespace(cp.Namespace)); err != nil {
		return fmt.Errorf("could not list virtualmachines: %w", err)
	}
	for _, virtualMachine := range virtualMachineList.Items {
		finalizerString := "azure.remedy.gardener.cloud/virtualmachine"
		if controllerutil.ContainsFinalizer(&virtualMachine, finalizerString) {
			if err := controllerutils.RemoveFinalizers(ctx, a.client, &virtualMachine, finalizerString); err != nil {
				return fmt.Errorf("could not remove finalizers from virtualmachine: %w", err)
			}
		}
	}

	log.Info("Deleting all remaining remedy controller resources")
	if err := a.client.DeleteAllOf(ctx, &azurev1alpha1.PublicIPAddress{}, client.InNamespace(cp.Namespace)); err != nil {
		return fmt.Errorf("could not delete publicipaddress resources: %w", err)
	}
	if err := a.client.DeleteAllOf(ctx, &azurev1alpha1.VirtualMachine{}, client.InNamespace(cp.Namespace)); err != nil {
		return fmt.Errorf("could not delete virtualmachine resources: %w", err)
	}

	return nil
}

func (a *actuator) forceDeleteShootRemedyControllerResources(ctx context.Context, log logr.Logger, cluster *extensionscontroller.Cluster) error {
	// Nothing to do if the feature is disabled
	if !features.ExtensionFeatureGate.Enabled(features.DisableRemedyController) {
		return nil
	}

	// Nothing to do  if the cluster is hibernated
	if extensionscontroller.IsHibernated(cluster) {
		return nil
	}
	_, shootClient, err := util.NewClientForShoot(ctx, a.client, cluster.ObjectMeta.Name, client.Options{}, extensionsconfigv1alpha1.RESTOptions{})
	if err != nil {
		// No need to report the error as this is anyway only best effort. Some scenarios, e.g. autonomous shoot clusters,
		// might not have the gardener secret and hence cannot construct the shoot client here.
		log.Info("Could not create shoot client to check for existing remedy controller resources", "error", err.Error())
		return err
	}

	lbs := &corev1.ServiceList{}
	var errs error

	for {
		if err := shootClient.List(ctx, lbs, client.Limit(10), client.Continue(lbs.GetContinue())); err != nil {
			log.Info("Could not list services in shoot cluster to check for existing remedy controller resources", "error", err)
			return err
		}

		for _, lb := range lbs.Items {
			if lb.Spec.Type != corev1.ServiceTypeLoadBalancer {
				continue
			}
			if controllerutil.ContainsFinalizer(&lb, remedyctrl.FinalizerName) {
				if retryErr := retry.RetryOnConflict(retry.DefaultRetry,
					func() error {
						err := controllerutils.RemoveFinalizers(ctx, shootClient, &lb, remedyctrl.FinalizerName)
						if apierrors.IsConflict(err) {
							return shootClient.Get(ctx, client.ObjectKey{Namespace: lb.Namespace, Name: lb.Name}, &lb)
						} else if err != nil {
							return err
						}
						return nil
					}); retryErr != nil {
					log.Info("Could not remove remedy finalizer from service", "namespace", lb.Namespace, "name", lb.Name, "error", retryErr)
					errs = errors.Join(errs, err)
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
