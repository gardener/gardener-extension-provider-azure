// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
)

// DeployMachineDependencies implements genericactuator.WorkerDelegate.
// Deprecated: Do not use this func. It is deprecated in genericactuator.WorkerDelegate.
func (w *workerDelegate) DeployMachineDependencies(_ context.Context) error {
	return nil
}

// CleanupMachineDependencies implements genericactuator.WorkerDelegate.
// Deprecated: Do not use this func. It is deprecated in genericactuator.WorkerDelegate.
func (w *workerDelegate) CleanupMachineDependencies(_ context.Context) error {
	return nil
}

// PreReconcileHook implements genericactuator.WorkerDelegate.
func (w *workerDelegate) PreReconcileHook(ctx context.Context) error {
	infrastructureStatus, err := w.decodeAzureInfrastructureStatus()
	if err != nil {
		return err
	}
	workerProviderStatus, err := w.decodeWorkerProviderStatus()
	if err != nil {
		return err
	}

	if helper.IsVmoRequired(infrastructureStatus) {
		vmoDependencies, err := w.reconcileVmoDependencies(ctx, infrastructureStatus, workerProviderStatus)
		workerProviderStatus.VmoDependencies = vmoDependencies
		if err != nil {
			return w.updateWorkerProviderStatusWithError(ctx, workerProviderStatus, err)
		}
		return w.updateWorkerProviderStatus(ctx, workerProviderStatus)
	}

	return nil
}

// PostReconcileHook implements genericactuator.WorkerDelegate.
func (w *workerDelegate) PostReconcileHook(ctx context.Context) error {
	return w.cleanupMachineDependencies(ctx)
}

// PreDeleteHook implements genericactuator.WorkerDelegate.
func (w *workerDelegate) PreDeleteHook(_ context.Context) error {
	return nil
}

// PostDeleteHook implements genericactuator.WorkerDelegate.
func (w *workerDelegate) PostDeleteHook(ctx context.Context) error {
	return w.cleanupMachineDependencies(ctx)
}

// cleanupMachineDependencies cleans up machine dependencies.
//
// TODO(dkistner, kon-angelo): Currently both PostReconcileHook and PostDeleteHook funcs call cleanupMachineDependencies.
// cleanupMachineDependencies calls cleanupVmoDependencies. cleanupVmoDependencies handles the cases when the Worker is being
// deleted (logic applicable for PostDeleteHook) and is not being deleted (logic applicable for PostReconcileHook).
// Refactor this so that PostDeleteHook executes only the handling for Worker being deleted and PostReconcileHook executes only
// the handling for Worker reconciled (not being deleted).
func (w *workerDelegate) cleanupMachineDependencies(ctx context.Context) error {
	infrastructureStatus, err := w.decodeAzureInfrastructureStatus()
	if err != nil {
		return err
	}
	workerProviderStatus, err := w.decodeWorkerProviderStatus()
	if err != nil {
		return err
	}

	if helper.IsVmoRequired(infrastructureStatus) {
		vmoDependencies, err := w.cleanupVmoDependencies(ctx, infrastructureStatus, workerProviderStatus)
		workerProviderStatus.VmoDependencies = vmoDependencies
		if err != nil {
			return w.updateWorkerProviderStatusWithError(ctx, workerProviderStatus, err)
		}
		return w.updateWorkerProviderStatus(ctx, workerProviderStatus)
	}

	return nil
}
