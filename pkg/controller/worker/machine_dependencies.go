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
