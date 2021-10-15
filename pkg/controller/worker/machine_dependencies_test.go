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
	"fmt"
	"net/http"
	"time"

	azureapi "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	factorymock "github.com/gardener/gardener-extension-provider-azure/pkg/mock/factory"
	vmssmock "github.com/gardener/gardener-extension-provider-azure/pkg/mock/vmss"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-03-01/compute"
	"github.com/Azure/go-autorest/autorest"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
)

var _ = Describe("MachinesDependencies", func() {
	var (
		ctrl         *gomock.Controller
		c            *mockclient.MockClient
		statusWriter *mockclient.MockStatusWriter
		factory      *factorymock.MockFactory

		ctx       context.Context
		secretRef corev1.SecretReference

		namespace, resourceGroupName, region string
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		statusWriter = mockclient.NewMockStatusWriter(ctrl)
		factory = factorymock.NewMockFactory(ctrl)

		// Let the client always the mocked status writer when Status() is called.
		c.EXPECT().Status().AnyTimes().Return(statusWriter)

		ctx = context.TODO()
		namespace = "shoot--foobar--azure"
		resourceGroupName = namespace
		region = "westeurope"
		secretRef = corev1.SecretReference{
			Name:      "secret",
			Namespace: namespace,
		}
	})

	Describe("VMO Dependencies", func() {
		var (
			vmoClient *vmssmock.MockVmss

			vmoName, vmoID   string
			faultDomainCount int32

			cluster              *extensionscontroller.Cluster
			infrastructureStatus *azureapi.InfrastructureStatus
			pool                 extensionsv1alpha1.WorkerPool
			vmoDependency        v1alpha1.VmoDependency
		)

		BeforeEach(func() {
			// Create a vmo client mock and let the factory always return the mocked vmo client.
			vmoClient = vmssmock.NewMockVmss(ctrl)
			factory.EXPECT().Vmss(ctx, secretRef).AnyTimes().Return(vmoClient, nil)

			faultDomainCount = 3
			cluster = makeCluster("", "westeurope", nil, nil, faultDomainCount)
			cluster.Shoot.Annotations = map[string]string{
				azure.ShootVmoUsageAnnotation: "true",
			}
			infrastructureStatus = makeInfrastructureStatus(resourceGroupName, "vnet-name", "subnet-name", false, nil, nil, nil)
			pool = extensionsv1alpha1.WorkerPool{
				Name: "my-pool",
			}

			vmoName = fmt.Sprintf("vmo-%s-12345678", pool.Name)
			vmoID = fmt.Sprintf("/subscriptions/sample-subscription/resourceGroups/sample-rg/providers/Microsoft.Compute/virtualNetworks/virtualMachineScaleSets/%s", vmoName)
			vmoDependency = v1alpha1.VmoDependency{
				ID:       vmoID,
				Name:     vmoName,
				PoolName: pool.Name,
			}
		})

		Context("#DeployMachineDependencies", func() {
			It("should deploy no vmo dependency as it is not required", func() {
				w := makeWorker(namespace, region, nil, nil)
				workerDelegate := wrapNewWorkerDelegate(c, nil, w, cluster, factory)

				err := workerDelegate.DeployMachineDependencies(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should deploy new vmo dependency as none exists for the worker pool", func() {
				w := makeWorker(namespace, region, nil, infrastructureStatus, pool)
				workerDelegate := wrapNewWorkerDelegate(c, nil, w, cluster, factory)

				expectVmoCreateToSucceed(ctx, vmoClient, resourceGroupName, vmoName, vmoID)
				expectWorkerProviderStatusUpdateToSucceed(ctx, c, statusWriter)
				err := workerDelegate.DeployMachineDependencies(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := decodeWorkerProviderStatus(w)
				Expect(workerStatus.VmoDependencies).To(ContainElements(MatchFields(IgnoreExtras, Fields{
					"ID":       Equal(vmoID),
					"Name":     Equal(vmoName),
					"PoolName": Equal(pool.Name),
				})))
			})

			It("should not deploy a vmo dependency as already one exists for the worker pool", func() {
				w := makeWorker(namespace, region, nil, infrastructureStatus, pool)
				w.Status.ProviderStatus = generateWorkerStatusWithVmo(vmoDependency)
				workerDelegate := wrapNewWorkerDelegate(c, nil, w, cluster, factory)

				expectVmoGetToSucceed(ctx, vmoClient, resourceGroupName, vmoName, vmoID, faultDomainCount)
				expectWorkerProviderStatusUpdateToSucceed(ctx, c, statusWriter)
				err := workerDelegate.DeployMachineDependencies(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := decodeWorkerProviderStatus(w)
				Expect(workerStatus.VmoDependencies).To(ContainElements(MatchFields(IgnoreExtras, Fields{
					"ID":       Equal(vmoDependency.ID),
					"Name":     Equal(vmoDependency.Name),
					"PoolName": Equal(vmoDependency.PoolName),
				})))
			})

			It("should deploy a new vmo dependency as the fault domain count changes", func() {
				w := makeWorker(namespace, region, nil, infrastructureStatus, pool)
				w.Status.ProviderStatus = generateWorkerStatusWithVmo(vmoDependency)
				workerDelegate := wrapNewWorkerDelegate(c, nil, w, cluster, factory)

				var oldFaultDomainCoaunt int32 = 2
				expectVmoGetToSucceed(ctx, vmoClient, resourceGroupName, vmoName, vmoID, oldFaultDomainCoaunt)
				expectVmoCreateToSucceed(ctx, vmoClient, resourceGroupName, vmoName, vmoID)
				expectWorkerProviderStatusUpdateToSucceed(ctx, c, statusWriter)
				err := workerDelegate.DeployMachineDependencies(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := decodeWorkerProviderStatus(w)
				Expect(workerStatus.VmoDependencies).To(ContainElements(MatchFields(IgnoreExtras, Fields{
					"ID":       Equal(vmoDependency.ID),
					"Name":     Equal(vmoDependency.Name),
					"PoolName": Equal(vmoDependency.PoolName),
				})))
			})
		})

		Context("#CleanupMachineDependencies", func() {
			It("should cleanup nothing as no vmo was required", func() {
				w := makeWorker(namespace, region, nil, nil)
				workerDelegate := wrapNewWorkerDelegate(c, nil, w, nil, factory)

				err := workerDelegate.CleanupMachineDependencies(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not cleanup a vmo dependency as resource group is gone", func() {
				w := makeWorker(namespace, region, nil, infrastructureStatus, pool)
				w.Status.ProviderStatus = generateWorkerStatusWithVmo(vmoDependency)
				workerDelegate := wrapNewWorkerDelegate(c, nil, w, cluster, factory)

				vmoClient.EXPECT().List(ctx, resourceGroupName).Return(nil, autorest.DetailedError{
					Original:   fmt.Errorf("ResourceGroupNotFound"),
					StatusCode: http.StatusNotFound,
					Response: &http.Response{
						StatusCode: http.StatusNotFound,
					},
				})
				expectWorkerProviderStatusUpdateToSucceed(ctx, c, statusWriter)
				err := workerDelegate.CleanupMachineDependencies(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not cleanup a vmo dependency as worker pool still exists", func() {
				w := makeWorker(namespace, region, nil, infrastructureStatus, pool)
				w.Status.ProviderStatus = generateWorkerStatusWithVmo(vmoDependency)
				workerDelegate := wrapNewWorkerDelegate(c, nil, w, cluster, factory)

				expectVmoListToSucceed(ctx, vmoClient, resourceGroupName, generateExpectedVmo(vmoName, vmoID))
				expectWorkerProviderStatusUpdateToSucceed(ctx, c, statusWriter)
				err := workerDelegate.CleanupMachineDependencies(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := decodeWorkerProviderStatus(w)
				Expect(workerStatus.VmoDependencies).To(ContainElements(MatchFields(IgnoreExtras, Fields{
					"ID":       Equal(vmoDependency.ID),
					"Name":     Equal(vmoDependency.Name),
					"PoolName": Equal(vmoDependency.PoolName),
				})))
			})

			It("should not cleanup vmo dependencies, but remove orphan managed vmos", func() {
				w := makeWorker(namespace, region, nil, infrastructureStatus, pool)
				w.Status.ProviderStatus = generateWorkerStatusWithVmo(vmoDependency)
				workerDelegate := wrapNewWorkerDelegate(c, nil, w, cluster, factory)

				expectVmoListToSucceed(ctx, vmoClient, resourceGroupName, generateExpectedVmo(vmoName, vmoID), generateExpectedVmo("orphan-managed-vmss", "/some/orphan/vmss/id"))
				expectVmoDeleteToSucceed(ctx, vmoClient, resourceGroupName)
				expectWorkerProviderStatusUpdateToSucceed(ctx, c, statusWriter)
				err := workerDelegate.CleanupMachineDependencies(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := decodeWorkerProviderStatus(w)
				Expect(workerStatus.VmoDependencies).To(ContainElements(MatchFields(IgnoreExtras, Fields{
					"ID":       Equal(vmoDependency.ID),
					"Name":     Equal(vmoDependency.Name),
					"PoolName": Equal(vmoDependency.PoolName),
				})))
			})

			It("should cleanup a vmo dependency as corresponding worker pool does not exist anymore", func() {
				var (
					deletedPoolVmoName       = "deleted-pool-vmo-name"
					deletedPoolVmoID         = "deleted-pool-vmo-id"
					deletedPoolVmoDependency = v1alpha1.VmoDependency{
						ID:       deletedPoolVmoName,
						Name:     deletedPoolVmoID,
						PoolName: "deleted-pool-name",
					}
					w = makeWorker(namespace, region, nil, infrastructureStatus)
				)
				w.Status.ProviderStatus = generateWorkerStatusWithVmo(deletedPoolVmoDependency)
				workerDelegate := wrapNewWorkerDelegate(c, nil, w, cluster, factory)

				expectVmoListToSucceed(ctx, vmoClient, resourceGroupName, generateExpectedVmo(deletedPoolVmoName, deletedPoolVmoID))
				expectVmoDeleteToSucceed(ctx, vmoClient, resourceGroupName)
				expectWorkerProviderStatusUpdateToSucceed(ctx, c, statusWriter)
				err := workerDelegate.CleanupMachineDependencies(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := decodeWorkerProviderStatus(w)
				Expect(workerStatus.VmoDependencies).To(HaveLen(0))
			})

			It("should cleanup all vmo dependencies as Worker is intended to be deleted", func() {
				w := makeWorker(namespace, region, nil, infrastructureStatus, pool)
				w.Status.ProviderStatus = generateWorkerStatusWithVmo(vmoDependency)
				w.GetObjectMeta().SetDeletionTimestamp(&metav1.Time{Time: time.Now()})

				workerDelegate := wrapNewWorkerDelegate(c, nil, w, cluster, factory)

				expectVmoListToSucceed(ctx, vmoClient, resourceGroupName, generateExpectedVmo(vmoName, vmoID))
				expectVmoDeleteToSucceed(ctx, vmoClient, resourceGroupName)
				expectWorkerProviderStatusUpdateToSucceed(ctx, c, statusWriter)
				err := workerDelegate.CleanupMachineDependencies(ctx)
				Expect(err).NotTo(HaveOccurred())

				workerStatus := decodeWorkerProviderStatus(w)
				Expect(workerStatus.VmoDependencies).To(HaveLen(0))
			})
		})
	})
})

func expectVmoGetToSucceed(ctx context.Context, c *vmssmock.MockVmss, resourceGroupName, name, id string, faultDomainCount int32) {
	// As the vmo name (parameter 3) contains a random suffix, we use simply anything of type string for the mock.
	c.EXPECT().Get(ctx, resourceGroupName, gomock.AssignableToTypeOf(""), compute.ExpandTypesForGetVMScaleSetsUserData).Return(&compute.VirtualMachineScaleSet{
		ID:   pointer.StringPtr(id),
		Name: pointer.StringPtr(name),
		VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
			PlatformFaultDomainCount: &faultDomainCount,
		},
	}, nil)
}

func expectVmoListToSucceed(ctx context.Context, c *vmssmock.MockVmss, resourceGroupName string, vmos ...compute.VirtualMachineScaleSet) {
	c.EXPECT().List(ctx, resourceGroupName).Return(vmos, nil)
}

func expectVmoCreateToSucceed(ctx context.Context, c *vmssmock.MockVmss, resourceGroupName, name, id string) {
	// As the vmo name (parameter 3) contains a random suffix, we use simply anything of type string for the mock.
	c.EXPECT().Create(ctx, resourceGroupName, gomock.AssignableToTypeOf(""), gomock.AssignableToTypeOf(&compute.VirtualMachineScaleSet{})).Return(&compute.VirtualMachineScaleSet{
		ID:   pointer.StringPtr(id),
		Name: pointer.StringPtr(name),
	}, nil)
}

func expectVmoDeleteToSucceed(ctx context.Context, c *vmssmock.MockVmss, resourceGroupName string) {
	// As the vmo name (parameter 3) contains a random suffix, we use simply anything of type string for the mock.
	c.EXPECT().Delete(ctx, resourceGroupName, gomock.AssignableToTypeOf(""), pointer.Bool(false)).AnyTimes().Return(nil)
}

func generateWorkerStatusWithVmo(vmos ...v1alpha1.VmoDependency) *runtime.RawExtension {
	workerStatus := &v1alpha1.WorkerStatus{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "WorkerStatus",
		},
		VmoDependencies: vmos,
	}
	workerStatusMarshaled, err := json.Marshal(workerStatus)
	Expect(err).NotTo(HaveOccurred())
	return &runtime.RawExtension{
		Raw: workerStatusMarshaled,
	}
}

func generateExpectedVmo(name, id string) compute.VirtualMachineScaleSet {
	return compute.VirtualMachineScaleSet{
		ID:   pointer.StringPtr(id),
		Name: pointer.StringPtr(name),
		Tags: map[string]*string{
			azure.MachineSetTagKey: pointer.StringPtr("1"),
		},
	}
}
