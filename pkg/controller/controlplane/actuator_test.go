// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"time"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	azurev1alpha1 "github.com/gardener/remedy-controller/pkg/apis/azure/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mockcontrolplane "github.com/gardener/gardener/extensions/pkg/controller/controlplane/mock"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/utils/test"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

var _ = Describe("Actuator", func() {
	var (
		ctrl   *gomock.Controller
		ctx    = context.TODO()
		logger = log.Log.WithName("test")

		c        *mockclient.MockClient
		a        *mockcontrolplane.MockActuator
		actuator controlplane.Actuator

		gracefulDeletionTimeout      time.Duration
		gracefulDeletionWaitInterval time.Duration
		newControlPlane              = func(purpose *extensionsv1alpha1.Purpose) *extensionsv1alpha1.ControlPlane {
			return &extensionsv1alpha1.ControlPlane{
				ObjectMeta: metav1.ObjectMeta{Name: "control-plane", Namespace: namespace},
				Spec: extensionsv1alpha1.ControlPlaneSpec{
					Purpose: purpose,
				},
			}
		}
		cluster = &extensionscontroller.Cluster{
			Shoot: &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{},
			},
		}
		newPubip = func(annotations map[string]string) *azurev1alpha1.PublicIPAddress {
			return &azurev1alpha1.PublicIPAddress{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "foo-1.2.3.4",
					Namespace:       namespace,
					Annotations:     annotations,
					ResourceVersion: "1",
				},
				Spec: azurev1alpha1.PublicIPAddressSpec{
					IPAddress: "1.2.3.4",
				},
			}
		}

		newVirtualMachine = func() *azurev1alpha1.VirtualMachine {
			return &azurev1alpha1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "node-name",
					Namespace:       namespace,
					ResourceVersion: "1",
				},
			}
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		a = mockcontrolplane.NewMockActuator(ctrl)
		gracefulDeletionTimeout = 10 * time.Second
		gracefulDeletionWaitInterval = 1 * time.Second
		actuator = NewActuator(a, gracefulDeletionTimeout, gracefulDeletionWaitInterval)

		err := actuator.(inject.Client).InjectClient(c)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Delete", func() {
		It("should successfully delete controlplane if there are no remedy controller resources", func() {
			cp := newControlPlane(nil)
			time := metav1.Now()
			cp.DeletionTimestamp = &time
			c.EXPECT().List(gomock.Any(), &azurev1alpha1.PublicIPAddressList{}, client.InNamespace(namespace)).
				DoAndReturn(func(_ context.Context, list *azurev1alpha1.PublicIPAddressList, _ ...client.ListOption) error {
					list.Items = []azurev1alpha1.PublicIPAddress{}
					return nil
				}).Times(2)

			c.EXPECT().List(ctx, &azurev1alpha1.VirtualMachineList{}, client.InNamespace(namespace)).
				DoAndReturn(func(_ context.Context, list *azurev1alpha1.VirtualMachineList, _ ...client.ListOption) error {
					list.Items = []azurev1alpha1.VirtualMachine{}
					return nil
				})

			c.EXPECT().DeleteAllOf(ctx, &azurev1alpha1.PublicIPAddress{}, client.InNamespace(namespace)).Return(nil)
			c.EXPECT().DeleteAllOf(ctx, &azurev1alpha1.VirtualMachine{}, client.InNamespace(namespace)).Return(nil)

			a.EXPECT().Delete(ctx, logger, cp, cluster).Return(nil)
			err := actuator.Delete(ctx, logger, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return RequeueAfterError if there are publicipaddresses remaining and timeout is not yet reached", func() {
			cp := newControlPlane(nil)
			time := metav1.Now()
			cp.DeletionTimestamp = &time

			pubip := newPubip(nil)
			pubipWithFinalizers := pubip.DeepCopy()
			pubipWithFinalizers.Finalizers = append(pubipWithFinalizers.Finalizers, "azure.remedy.gardener.cloud/publicipaddress")

			c.EXPECT().List(ctx, &azurev1alpha1.PublicIPAddressList{}, client.InNamespace(namespace)).
				DoAndReturn(func(_ context.Context, list *azurev1alpha1.PublicIPAddressList, _ ...client.ListOption) error {
					list.Items = []azurev1alpha1.PublicIPAddress{*pubipWithFinalizers}
					return nil
				})

			err := actuator.Delete(ctx, logger, cp, cluster)
			Expect(err).To(MatchError(&reconcilerutils.RequeueAfterError{RequeueAfter: gracefulDeletionWaitInterval}))
		})

		It("should forcefully remove remedy controller resources after grace period timeout has been reached", func() {
			cp := newControlPlane(nil)
			time := metav1.NewTime(time.Now().Add(time.Duration(-2 * gracefulDeletionTimeout)))
			cp.DeletionTimestamp = &time

			pubip := newPubip(nil)
			pubipWithFinalizers := pubip.DeepCopy()
			pubipWithFinalizers.Finalizers = append(pubipWithFinalizers.Finalizers, "azure.remedy.gardener.cloud/publicipaddress")
			c.EXPECT().List(ctx, &azurev1alpha1.PublicIPAddressList{}, client.InNamespace(namespace)).
				DoAndReturn(func(_ context.Context, list *azurev1alpha1.PublicIPAddressList, _ ...client.ListOption) error {
					list.Items = []azurev1alpha1.PublicIPAddress{*pubipWithFinalizers}
					return nil
				}).Times(2)

			test.EXPECTPatchWithOptimisticLock(ctx, c, pubip, pubipWithFinalizers, types.MergePatchType)
			c.EXPECT().DeleteAllOf(ctx, &azurev1alpha1.PublicIPAddress{}, client.InNamespace(namespace)).Return(nil)

			vm := newVirtualMachine()
			vmWithFinalizers := vm.DeepCopy()
			vmWithFinalizers.Finalizers = append(vmWithFinalizers.Finalizers, "azure.remedy.gardener.cloud/virtualmachine")
			c.EXPECT().List(ctx, &azurev1alpha1.VirtualMachineList{}, client.InNamespace(namespace)).
				DoAndReturn(func(_ context.Context, list *azurev1alpha1.VirtualMachineList, _ ...client.ListOption) error {
					list.Items = []azurev1alpha1.VirtualMachine{*vmWithFinalizers}
					return nil
				})
			test.EXPECTPatchWithOptimisticLock(ctx, c, vm, vmWithFinalizers, types.MergePatchType)
			c.EXPECT().DeleteAllOf(ctx, &azurev1alpha1.VirtualMachine{}, client.InNamespace(namespace)).Return(nil)

			a.EXPECT().Delete(ctx, logger, cp, cluster).Return(nil)

			err := actuator.Delete(ctx, logger, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#Migrate", func() {
		It("should remove finalizers from remedy controller resources and then delete them", func() {
			cp := newControlPlane(nil)
			a.EXPECT().Migrate(ctx, logger, cp, cluster).Return(nil)

			pubip := newPubip(nil)
			pubipWithFinalizers := pubip.DeepCopy()
			pubipWithFinalizers.Finalizers = append(pubipWithFinalizers.Finalizers, "azure.remedy.gardener.cloud/publicipaddress")
			c.EXPECT().List(ctx, &azurev1alpha1.PublicIPAddressList{}, client.InNamespace(namespace)).
				DoAndReturn(func(_ context.Context, list *azurev1alpha1.PublicIPAddressList, _ ...client.ListOption) error {
					list.Items = []azurev1alpha1.PublicIPAddress{*pubipWithFinalizers}
					return nil
				})
			test.EXPECTPatchWithOptimisticLock(ctx, c, pubip, pubipWithFinalizers, types.MergePatchType)
			c.EXPECT().DeleteAllOf(ctx, &azurev1alpha1.PublicIPAddress{}, client.InNamespace(namespace)).Return(nil)

			vm := newVirtualMachine()
			vmWithFinalizers := vm.DeepCopy()
			vmWithFinalizers.Finalizers = append(vmWithFinalizers.Finalizers, "azure.remedy.gardener.cloud/virtualmachine")
			c.EXPECT().List(ctx, &azurev1alpha1.VirtualMachineList{}, client.InNamespace(namespace)).
				DoAndReturn(func(_ context.Context, list *azurev1alpha1.VirtualMachineList, _ ...client.ListOption) error {
					list.Items = []azurev1alpha1.VirtualMachine{*vmWithFinalizers}
					return nil
				})
			test.EXPECTPatchWithOptimisticLock(ctx, c, vm, vmWithFinalizers, types.MergePatchType)
			c.EXPECT().DeleteAllOf(ctx, &azurev1alpha1.VirtualMachine{}, client.InNamespace(namespace)).Return(nil)

			err := actuator.Migrate(ctx, logger, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should neither remove finalizers from remaining remedy controller resources nor delete them for controlplane with purpose exposure", func() {
			exposure := extensionsv1alpha1.Exposure
			cp := newControlPlane(&exposure)
			a.EXPECT().Migrate(ctx, logger, cp, cluster).Return(nil)

			err := actuator.Migrate(ctx, logger, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
