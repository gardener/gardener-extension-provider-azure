// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"time"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane"
	mockcontrolplane "github.com/gardener/gardener/extensions/pkg/controller/controlplane/mock"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/gardener/gardener/pkg/utils/test"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	azurev1alpha1 "github.com/gardener/remedy-controller/pkg/apis/azure/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("Actuator", func() {
	var (
		ctrl   *gomock.Controller
		ctx    = context.TODO()
		logger = log.Log.WithName("test")

		c        *mockclient.MockClient
		mgr      *mockmanager.MockManager
		a        *mockcontrolplane.MockActuator
		actuator controlplane.Actuator

		gracefulDeletionTimeout      time.Duration
		gracefulDeletionWaitInterval time.Duration
		newControlPlane              = func() *extensionsv1alpha1.ControlPlane {
			return &extensionsv1alpha1.ControlPlane{
				ObjectMeta: metav1.ObjectMeta{Name: "control-plane", Namespace: namespace},
				Spec:       extensionsv1alpha1.ControlPlaneSpec{},
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
		mgr = mockmanager.NewMockManager(ctrl)
		mgr.EXPECT().GetClient().Return(c)

		a = mockcontrolplane.NewMockActuator(ctrl)
		gracefulDeletionTimeout = 10 * time.Second
		gracefulDeletionWaitInterval = 1 * time.Second
		actuator = NewActuator(mgr, a, gracefulDeletionTimeout, gracefulDeletionWaitInterval)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Delete", func() {
		It("should successfully delete controlplane if there are no remedy controller resources", func() {
			cp := newControlPlane()
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
			cp := newControlPlane()
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
			cp := newControlPlane()
			time := metav1.NewTime(time.Now().Add(-2 * gracefulDeletionTimeout))
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
			cp := newControlPlane()
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
	})
})
