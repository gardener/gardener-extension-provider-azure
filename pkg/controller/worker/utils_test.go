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

package worker_test

import (
	"context"
	"encoding/json"

	apiazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/controller/worker"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/common"
	"github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func wrapNewWorkerDelegate(client *mockclient.MockClient, seedChartApplier *mockkubernetes.MockChartApplier, worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster, factory azureclient.Factory) genericactuator.WorkerDelegate {
	expectGetSecretCallToWork(client, worker)

	scheme := runtime.NewScheme()
	_ = apiazure.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)

	workerDelegate, err := NewWorkerDelegate(common.NewClientContext(client, scheme, serializer.NewCodecFactory(scheme, serializer.EnableStrict).UniversalDecoder()), seedChartApplier, "", worker, cluster, factory)
	Expect(err).NotTo(HaveOccurred())
	return workerDelegate
}

func decodeWorkerProviderStatus(worker *extensionsv1alpha1.Worker) *v1alpha1.WorkerStatus {
	workerProviderStatus, ok := worker.Status.ProviderStatus.Object.(*v1alpha1.WorkerStatus)
	Expect(ok).To(BeTrue())
	return workerProviderStatus
}

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}

func expectWorkerProviderStatusUpdateToSucceed(ctx context.Context, c *mockclient.MockClient, statusWriter *mockclient.MockStatusWriter) {
	c.EXPECT().Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(&extensionsv1alpha1.Worker{})).
		DoAndReturn(func(_ context.Context, _ client.ObjectKey, worker *extensionsv1alpha1.Worker) error {
			return nil
		})
	statusWriter.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.Worker{})).Return(nil)
}

func expectGetSecretCallToWork(c *mockclient.MockClient, w *extensionsv1alpha1.Worker) {
	c.EXPECT().Get(context.TODO(), kutil.Key(w.Spec.SecretRef.Namespace, w.Spec.SecretRef.Name), &corev1.Secret{}).DoAndReturn(
		func(_ context.Context, __ client.ObjectKey, secret *corev1.Secret) error {
			secret.Data = map[string][]byte{
				azure.ClientIDKey:       []byte("client-id"),
				azure.ClientSecretKey:   []byte("client-secret"),
				azure.SubscriptionIDKey: []byte("1234"),
				azure.TenantIDKey:       []byte("1234"),
			}
			return nil
		}).AnyTimes()
}

func makeWorker(namespace string, region string, sshKey *string, infrastructureStatus *apiazure.InfrastructureStatus, pools ...extensionsv1alpha1.WorkerPool) *extensionsv1alpha1.Worker {
	var (
		infraStatus = infrastructureStatus
		sshKeyByte  = []byte{}
	)

	if infrastructureStatus == nil {
		infraStatus = &apiazure.InfrastructureStatus{
			Zoned: true,
		}
	}

	if sshKey != nil {
		sshKeyByte = []byte(*sshKey)
	}

	return &extensionsv1alpha1.Worker{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
		},
		Spec: extensionsv1alpha1.WorkerSpec{
			SecretRef: corev1.SecretReference{
				Name:      "secret",
				Namespace: namespace,
			},
			Region:       region,
			SSHPublicKey: sshKeyByte,
			InfrastructureProviderStatus: &runtime.RawExtension{
				Raw: encode(infraStatus),
			},
			Pools: pools,
		},
	}
}

func makeCluster(shootVersion, region string, machineTypes []v1alpha1.MachineType, machineImages []v1alpha1.MachineImages, faultDomainCount int32) *extensionscontroller.Cluster {
	cloudProfileConfig := &v1alpha1.CloudProfileConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "CloudProfileConfig",
		},
		MachineImages: machineImages,
		MachineTypes:  machineTypes,
		CountFaultDomains: []v1alpha1.DomainCount{{
			Region: region,
			Count:  faultDomainCount,
		}},
	}
	cloudProfileConfigJSON, _ := json.Marshal(cloudProfileConfig)

	return &extensionscontroller.Cluster{
		CloudProfile: &gardencorev1beta1.CloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name: "azure",
			},
			Spec: gardencorev1beta1.CloudProfileSpec{
				ProviderConfig: &runtime.RawExtension{
					Raw: cloudProfileConfigJSON,
				},
			},
		},
		Shoot: &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: shootVersion,
				},
			},
		},
	}
}

func makeInfrastructureStatus(resourceGroupName, vnetName, subnetName string, zoned bool, vnetrg, availabilitySetID, identityID *string) *apiazure.InfrastructureStatus {
	topology := apiazure.TopologyRegional
	if zoned {
		topology = apiazure.TopologyZonalSingleSubnet
	}

	var infrastructureStatus = apiazure.InfrastructureStatus{
		ResourceGroup: apiazure.ResourceGroup{
			Name: resourceGroupName,
		},
		Networks: apiazure.NetworkStatus{
			VNet: apiazure.VNetStatus{
				Name: vnetName,
			},
			Subnets: []apiazure.Subnet{
				{
					Purpose: apiazure.PurposeNodes,
					Name:    subnetName,
				},
			},
			Topology: topology,
		},
		Zoned: zoned,
	}
	if vnetrg != nil {
		infrastructureStatus.Networks.VNet.ResourceGroup = vnetrg
	}
	if availabilitySetID != nil {
		infrastructureStatus.AvailabilitySets = []apiazure.AvailabilitySet{{
			Purpose: apiazure.PurposeNodes,
			ID:      *availabilitySetID,
		}}
	}
	if identityID != nil {
		infrastructureStatus.Identity = &apiazure.IdentityStatus{
			ID: *identityID,
		}
	}
	return &infrastructureStatus
}
