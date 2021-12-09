// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controlplane

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	azurev1alpha1 "github.com/gardener/remedy-controller/pkg/apis/azure/v1alpha1"
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
	a controlplane.Actuator,
	logger logr.Logger,
	gracefulDeletionTimeout time.Duration,
	gracefulDeletionWaitInterval time.Duration,
) controlplane.Actuator {
	return &actuator{
		Actuator:                     a,
		logger:                       logger.WithName("azure-controlplane-actuator"),
		gracefulDeletionTimeout:      gracefulDeletionTimeout,
		gracefulDeletionWaitInterval: gracefulDeletionWaitInterval,
	}
}

// actuator is an Actuator that acts upon and updates the status of ControlPlane resources.
type actuator struct {
	controlplane.Actuator
	client                       client.Client
	logger                       logr.Logger
	gracefulDeletionTimeout      time.Duration
	gracefulDeletionWaitInterval time.Duration
}

// InjectFunc enables injecting Kubernetes dependencies into actuator's dependencies.
func (a *actuator) InjectFunc(f inject.Func) error {
	return f(a.Actuator)
}

// InjectClient injects the given client into the valuesProvider.
func (a *actuator) InjectClient(client client.Client) error {
	a.client = client
	return nil
}

// Delete reconciles the given controlplane and cluster, deleting the additional
// control plane components as needed.
// Before delegating to the composed Actuator, it ensures that all remedy controller resources have been deleted gracefully.
func (a *actuator) Delete(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) error {
	if cp.Spec.Purpose == nil || *cp.Spec.Purpose == extensionsv1alpha1.Normal {
		list := &azurev1alpha1.PublicIPAddressList{}
		if err := a.client.List(ctx, list, client.InNamespace(cp.Namespace)); err != nil {
			return err
		}
		if meta.LenList(list) != 0 {
			if time.Since(cp.DeletionTimestamp.Local()) <= a.gracefulDeletionTimeout {
				a.logger.Info("Some publicipaddresses still exist. Deletion will be retried ...")
				return &reconcilerutils.RequeueAfterError{
					RequeueAfter: a.gracefulDeletionWaitInterval,
				}
			} else {
				a.logger.Info("The timeout for waiting for publicipaddresses to be gracefully deleted has expired. They will be forcefully removed.")
			}
		}
	}

	// Call Delete on the composed Actuator
	if err := a.Actuator.Delete(ctx, cp, cluster); err != nil {
		return err
	}

	if cp.Spec.Purpose == nil || *cp.Spec.Purpose == extensionsv1alpha1.Normal {
		// Delete all remaining remedy controller resources
		return a.forceDeleteRemedyControllerResources(ctx, cp)
	}
	return nil
}

// Migrate reconciles the given controlplane and cluster, migrating the additional
// control plane components as needed.
// Before delegating to the composed Actuator, it ensures that all remedy controller resources have been deleted.
func (a *actuator) Migrate(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) error {
	// Call Migrate on the composed Actuator so that the controlplane chart is deleted and therefore
	// the remedy controller is also removed.
	if err := a.Actuator.Migrate(ctx, cp, cluster); err != nil {
		return err
	}
	if cp.Spec.Purpose == nil || *cp.Spec.Purpose == extensionsv1alpha1.Normal {
		// Delete all remaining remedy controller resources
		return a.forceDeleteRemedyControllerResources(ctx, cp)
	}
	return nil
}

func (a *actuator) forceDeleteRemedyControllerResources(ctx context.Context, cp *extensionsv1alpha1.ControlPlane) error {
	a.logger.Info("Removing finalizers from remedy controller resources")
	pubipList := &azurev1alpha1.PublicIPAddressList{}
	if err := a.client.List(ctx, pubipList, client.InNamespace(cp.Namespace)); err != nil {
		return fmt.Errorf("could not list publicipaddresses: %w", err)
	}
	for _, pubip := range pubipList.Items {
		if err := controllerutils.PatchRemoveFinalizers(ctx, a.client, &pubip, "azure.remedy.gardener.cloud/publicipaddress"); err != nil {
			return fmt.Errorf("could not remove finalizers from publicipaddress: %w", err)
		}
	}

	virtualMachineList := &azurev1alpha1.VirtualMachineList{}
	if err := a.client.List(ctx, virtualMachineList, client.InNamespace(cp.Namespace)); err != nil {
		return fmt.Errorf("could not list virtualmachines: %w", err)
	}
	for _, virtualMachine := range virtualMachineList.Items {
		if err := controllerutils.PatchRemoveFinalizers(ctx, a.client, &virtualMachine, "azure.remedy.gardener.cloud/virtualmachine"); err != nil {
			return fmt.Errorf("could not remove finalizers from virtualmachine: %w", err)
		}
	}

	a.logger.Info("Deleting all remaining remedy controller resources")
	if err := a.client.DeleteAllOf(ctx, &azurev1alpha1.PublicIPAddress{}, client.InNamespace(cp.Namespace)); err != nil {
		return fmt.Errorf("could not delete publicipaddress resources: %w", err)
	}
	if err := a.client.DeleteAllOf(ctx, &azurev1alpha1.VirtualMachine{}, client.InNamespace(cp.Namespace)); err != nil {
		return fmt.Errorf("could not delete virtualmachine resources: %w", err)
	}

	return nil
}
