// SPDX-FileCopyrightText: Contributors to the Gardener project
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
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-azure/pkg/features"
)

// TODO(kon-angelo): Remove this actuator wrapper and all remedy-controller migration code in v1.70.0.
const (
	// GracefulDeletionWaitInterval is the default interval for retry operations.
	GracefulDeletionWaitInterval = 1 * time.Minute
	// GracefulDeletionTimeout is the timeout that defines how long the actuator should wait for remedy controller resources to be deleted
	// gracefully by the remedy controller itself
	GracefulDeletionTimeout       = 10 * time.Minute
	publicIpFinalizerString       = "azure.remedy.gardener.cloud/publicipaddress"
	virtualMachineFinalizerString = "azure.remedy.gardener.cloud/virtualmachine"
	serviceFinalizerName          = "azure.remedy.gardener.cloud/service"
	nodeFinalizerName             = "azure.remedy.gardener.cloud/node"
	remedyAPIVersion              = "azure.remedy.gardener.cloud/v1alpha1"
	publicIPAddressKind           = "PublicIPAddress"
	virtualMachineKind            = "VirtualMachine"
	remedyShootAccessSecretName   = "shoot-access-remedy-controller-azure"
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
// It also cleans up legacy remedy-controller resources and finalizers.
func (a *actuator) Delete(
	ctx context.Context,
	log logr.Logger,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) error {
	if !features.ExtensionFeatureGate.Enabled(features.DisableRemedyController) {
		list := newRemedyResourceList(publicIPAddressKind)
		if err := a.client.List(ctx, list, client.InNamespace(cp.Namespace)); ignoreMissingRemedyAPI(err) != nil {
			return err
		}
		if len(list.Items) != 0 {
			if time.Since(cp.DeletionTimestamp.Time) <= a.gracefulDeletionTimeout {
				log.Info("Some publicipaddresses still exist. Deletion will be retried ...")
				return &reconcilerutils.RequeueAfterError{
					RequeueAfter: a.gracefulDeletionWaitInterval,
				}
			} else {
				log.Info("The timeout for waiting for publicipaddresses to be gracefully deleted has expired. They will be forcefully removed.")
			}
		}
	}

	// Call Delete on the composed Actuator
	if err := a.Actuator.Delete(ctx, log, cp, cluster); err != nil {
		return err
	}

	if features.ExtensionFeatureGate.Enabled(features.DisableRemedyController) {
		return a.forceDeleteRemedyControllerResources(ctx, log, cp.Namespace, cluster)
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
// It also cleans up legacy remedy-controller resources and finalizers.
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
		return a.forceDeleteRemedyControllerResources(ctx, log, cp.Namespace, cluster)
	}
	// Delete all remaining remedy controller resources
	return a.forceDeleteSeedRemedyControllerResources(ctx, log, cp.GetNamespace())
}

func (a *actuator) forceDeleteSeedRemedyControllerResources(ctx context.Context, log logr.Logger, namespace string) error {
	log.Info("Removing legacy remedy controller resources")
	for _, resource := range []struct {
		kind      string
		finalizer string
	}{
		{kind: publicIPAddressKind, finalizer: publicIpFinalizerString},
		{kind: virtualMachineKind, finalizer: virtualMachineFinalizerString},
	} {
		list := newRemedyResourceList(resource.kind)
		if err := a.client.List(ctx, list, client.InNamespace(namespace)); err != nil {
			if ignoreMissingRemedyAPI(err) == nil {
				continue
			}
			return fmt.Errorf("could not list legacy %s resources: %w", resource.kind, err)
		}
		for i := range list.Items {
			obj := &list.Items[i]
			if controllerutil.ContainsFinalizer(obj, resource.finalizer) {
				if err := controllerutils.RemoveFinalizers(ctx, a.client, obj, resource.finalizer); ignoreMissingRemedyAPI(err) != nil {
					return fmt.Errorf("could not remove finalizer from legacy %s: %w", resource.kind, err)
				}
			}
		}
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion(remedyAPIVersion)
		obj.SetKind(resource.kind)
		if err := a.client.DeleteAllOf(ctx, obj, client.InNamespace(namespace)); ignoreMissingRemedyAPI(err) != nil {
			return fmt.Errorf("could not delete legacy %s resources: %w", resource.kind, err)
		}
	}

	return client.IgnoreNotFound(a.client.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name: remedyShootAccessSecretName, Namespace: namespace,
	}}))
}

func newRemedyResourceList(kind string) *unstructured.UnstructuredList {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.FromAPIVersionAndKind(remedyAPIVersion, kind+"List"))
	return list
}

func ignoreMissingRemedyAPI(err error) error {
	if meta.IsNoMatchError(err) || apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (a *actuator) forceDeleteShootRemedyControllerResources(ctx context.Context, log logr.Logger, namespace string, cluster *extensionscontroller.Cluster) error {
	// Nothing to do if the cluster is hibernated. Finalizers are cleaned up after it wakes up.
	if extensionscontroller.IsHibernated(cluster) {
		return nil
	}

	_, shootClient, err := util.NewClientForShoot(ctx, a.client, namespace, client.Options{}, extensionsconfigv1alpha1.RESTOptions{})
	if err != nil {
		log.Info("Could not create shoot client to check for existing remedy controller resources", "error", err.Error())
		return err
	}

	// The seed CRDs may already have been removed by the extension chart upgrade.
	// Shoot finalizers must be cleaned up independently of the existence of seed resources.
	if err := a.forceDeleteRemedyLoadBalancerFinalizer(ctx, log, shootClient); err != nil {
		return fmt.Errorf("could not remove remedy controller finalizers from services: %w", err)
	}

	if err := a.forceDeleteRemedyNodeFinalizer(ctx, log, shootClient); err != nil {
		return fmt.Errorf("could not remove remedy controller finalizers from nodes: %w", err)
	}

	return nil
}

func (a *actuator) forceDeleteRemedyControllerResources(ctx context.Context, log logr.Logger, namespace string, cluster *extensionscontroller.Cluster) error {
	err := a.forceDeleteShootRemedyControllerResources(ctx, log, namespace, cluster)
	if err != nil {
		return err
	}
	return a.forceDeleteSeedRemedyControllerResources(ctx, log, namespace)
}

func (a *actuator) forceDeleteRemedyNodeFinalizer(ctx context.Context, log logr.Logger, shootClient client.Client) error {
	nodes := &corev1.NodeList{}
	var errs error

	for {
		if err := shootClient.List(ctx, nodes, client.Limit(100), client.Continue(nodes.GetContinue())); err != nil {
			log.Info("Could not list nodes in shoot cluster to check for existing remedy controller resources", "error", err)
			return err
		}

		for _, node := range nodes.Items {
			if controllerutil.ContainsFinalizer(&node, nodeFinalizerName) {
				if retryErr := retry.RetryOnConflict(retry.DefaultRetry,
					func() error {
						err := shootClient.Get(ctx, client.ObjectKey{Name: node.Name}, &node)
						if err != nil {
							return err
						}
						return controllerutils.RemoveFinalizers(ctx, shootClient, &node, nodeFinalizerName)
					}); retryErr != nil {
					log.Info("Could not remove remedy finalizer from node", "name", node.Name, "error", retryErr)
					errs = errors.Join(errs, retryErr)
					continue
				}
			}
		}
		if nodes.GetContinue() == "" {
			break
		}
	}
	return errs
}

func (a *actuator) forceDeleteRemedyLoadBalancerFinalizer(ctx context.Context, log logr.Logger, shootClient client.Client) error {
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
