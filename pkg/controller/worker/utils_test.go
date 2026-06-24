// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker_test

import (
	"context"
	"encoding/json"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/controller/worker"
)

func wrapNewWorkerDelegate(c client.Client, seedChartApplier *mockkubernetes.MockChartApplier, worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster, factory azureclient.Factory) genericactuator.WorkerDelegate {
	// Pre-populate the secret referenced by the worker in the client.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: worker.Spec.SecretRef.Namespace,
			Name:      worker.Spec.SecretRef.Name,
		},
		Data: map[string][]byte{
			azure.ClientIDKey:       []byte("seedClient-id"),
			azure.ClientSecretKey:   []byte("seedClient-secret"),
			azure.SubscriptionIDKey: []byte("1234"),
			azure.TenantIDKey:       []byte("1234"),
		},
	}
	// Use Create or Update to handle multiple wrapNewWorkerDelegate calls in the same test.
	existing := &corev1.Secret{}
	if err := c.Get(context.TODO(), client.ObjectKeyFromObject(secret), existing); err != nil {
		Expect(c.Create(context.TODO(), secret)).To(Succeed())
	}

	// Create a minimal worker in the fake client so status patches can succeed.
	// Assign a synthetic name if the worker doesn't have one.
	if worker.Name == "" {
		worker.Name = "worker"
	}
	minimalWorker := &extensionsv1alpha1.Worker{
		ObjectMeta: metav1.ObjectMeta{
			Name:      worker.Name,
			Namespace: worker.Namespace,
		},
	}
	existingWorker := &extensionsv1alpha1.Worker{}
	if err := c.Get(context.TODO(), client.ObjectKeyFromObject(minimalWorker), existingWorker); err != nil {
		Expect(c.Create(context.TODO(), minimalWorker)).To(Succeed())
	}
	// Also mirror the existing status into the store so merge-patches produce a non-empty diff.
	if worker.Status.ProviderStatus != nil {
		statusPatch := client.MergeFrom(minimalWorker.DeepCopy())
		minimalWorker.Status = worker.Status
		Expect(c.Status().Patch(context.TODO(), minimalWorker, statusPatch)).To(Succeed())
	}

	scheme := runtime.NewScheme()
	_ = apiazure.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)

	workerDelegate, err := NewWorkerDelegate(c, scheme, seedChartApplier, "", worker, cluster, factory)
	Expect(err).NotTo(HaveOccurred())
	return workerDelegate
}

func decodeWorkerProviderStatus(worker *extensionsv1alpha1.Worker) *v1alpha1.WorkerStatus {
	ps := worker.Status.ProviderStatus
	Expect(ps).NotTo(BeNil())

	// If the object was already decoded (e.g. set directly by actuator before status patch).
	if ps.Object != nil {
		workerProviderStatus, ok := ps.Object.(*v1alpha1.WorkerStatus)
		Expect(ok).To(BeTrue())
		return workerProviderStatus
	}

	// Decode from Raw bytes (happens after fake client JSON round-trip on the stored copy).
	workerStatus := &v1alpha1.WorkerStatus{}
	Expect(json.Unmarshal(ps.Raw, workerStatus)).To(Succeed())
	return workerStatus
}

// readBackWorkerStatus reads the Worker back from the fake client and returns its provider status.
// Use this when the in-memory worker might not reflect the status updated via Status().Patch().
func readBackWorkerStatus(ctx context.Context, c client.Client, worker *extensionsv1alpha1.Worker) *v1alpha1.WorkerStatus {
	updated := &extensionsv1alpha1.Worker{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(worker), updated); err != nil {
		// Fallback to in-memory object if not in the store (e.g. worker has no name in some tests).
		return decodeWorkerProviderStatus(worker)
	}
	if updated.Status.ProviderStatus == nil {
		// Status not stored (e.g. actuator set it on in-memory obj but fake client didn't persist it).
		return decodeWorkerProviderStatus(worker)
	}
	return decodeWorkerProviderStatus(updated)
}

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
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

func makeCluster(technicalID, shootVersion, region string, machineTypes []v1alpha1.MachineType, machineImages []v1alpha1.MachineImages, faultDomainCount int32) *extensionscontroller.Cluster {
	coreMachineTypes := []gardencorev1beta1.MachineType{}

	for _, mt := range machineTypes {
		coreMachineTypes = append(coreMachineTypes, gardencorev1beta1.MachineType{Name: mt.Name})
	}

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
				MachineTypes: coreMachineTypes,
			},
		},
		Shoot: &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: shootVersion,
				},
			},
			Status: gardencorev1beta1.ShootStatus{
				TechnicalID: technicalID,
			},
		},
	}
}

func makeInfrastructureStatus(resourceGroupName, vnetName, subnetName string, zoned bool, vnetrg, identityID *string) *apiazure.InfrastructureStatus {
	infrastructureStatus := apiazure.InfrastructureStatus{
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
			Layout: apiazure.NetworkLayoutSingleSubnet,
		},
		Zoned: zoned,
	}

	if vnetrg != nil {
		infrastructureStatus.Networks.VNet.ResourceGroup = vnetrg
	}

	if identityID != nil {
		infrastructureStatus.Identity = &apiazure.IdentityStatus{
			ID: *identityID,
		}
	}
	return &infrastructureStatus
}
