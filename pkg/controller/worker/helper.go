// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	azureapi "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
)

func (w *workerDelegate) decodeAzureInfrastructureStatus() (*azureapi.InfrastructureStatus, error) {
	infrastructureStatus := &azureapi.InfrastructureStatus{}
	if _, _, err := w.lenientDecoder.Decode(w.worker.Spec.InfrastructureProviderStatus.Raw, nil, infrastructureStatus); err != nil {
		return nil, err
	}
	return infrastructureStatus, nil
}

func (w *workerDelegate) decodeWorkerProviderStatus() (*azureapi.WorkerStatus, error) {
	workerStatus := &azureapi.WorkerStatus{}
	if w.worker.Status.ProviderStatus == nil {
		return workerStatus, nil
	}

	if _, _, err := w.decoder.Decode(w.worker.Status.ProviderStatus.Raw, nil, workerStatus); err != nil {
		return nil, fmt.Errorf("could not decode the worker provider status of worker '%s': %w", kutil.ObjectName(w.worker), err)
	}
	return workerStatus, nil
}

func (w *workerDelegate) updateWorkerProviderStatus(ctx context.Context, workerStatus *azureapi.WorkerStatus) error {
	workerStatusV1alpha1 := &v1alpha1.WorkerStatus{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "WorkerStatus",
		},
	}

	if err := w.scheme.Convert(workerStatus, workerStatusV1alpha1, nil); err != nil {
		return err
	}

	patch := client.MergeFrom(w.worker.DeepCopy())
	w.worker.Status.ProviderStatus = &runtime.RawExtension{Object: workerStatusV1alpha1}
	return w.client.Status().Patch(ctx, w.worker, patch)
}

func (w *workerDelegate) updateWorkerProviderStatusWithError(ctx context.Context, workerStatus *azureapi.WorkerStatus, err error) error {
	if statusUpdateErr := w.updateWorkerProviderStatus(ctx, workerStatus); statusUpdateErr != nil {
		return fmt.Errorf("%s: %w", err.Error(), statusUpdateErr)
	}
	return err
}
