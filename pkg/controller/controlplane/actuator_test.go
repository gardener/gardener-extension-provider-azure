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
	testutils "github.com/gardener/gardener/pkg/utils/test"
	azurev1alpha1 "github.com/gardener/remedy-controller/pkg/apis/azure/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("Actuator", func() {
	var (
		ctrl   *gomock.Controller
		ctx    = context.TODO()
		logger = log.Log.WithName("test")

		c        client.Client
		scheme   *runtime.Scheme
		mgr      testutils.FakeManager
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
					Name:        "foo-1.2.3.4",
					Namespace:   namespace,
					Annotations: annotations,
				},
				Spec: azurev1alpha1.PublicIPAddressSpec{
					IPAddress: "1.2.3.4",
				},
			}
		}

		newVirtualMachine = func() *azurev1alpha1.VirtualMachine {
			return &azurev1alpha1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "node-name",
					Namespace: namespace,
				},
			}
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		scheme = runtime.NewScheme()
		Expect(azurev1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(extensionsv1alpha1.AddToScheme(scheme)).To(Succeed())

		c = fakeclient.NewClientBuilder().WithScheme(scheme).Build()
		mgr = testutils.FakeManager{Client: c}

		a = mockcontrolplane.NewMockActuator(ctrl)
		gracefulDeletionTimeout = 10 * time.Second
		gracefulDeletionWaitInterval = 1 * time.Second
		actuator = NewActuator(mgr, a, gracefulDeletionTimeout, gracefulDeletionWaitInterval)
	})

	Describe("#Delete", func() {
		It("should successfully delete controlplane if there are no remedy controller resources", func() {
			cp := newControlPlane()
			t := metav1.Now()
			cp.DeletionTimestamp = &t

			a.EXPECT().Delete(ctx, logger, cp, cluster).Return(nil)
			err := actuator.Delete(ctx, logger, cp, cluster)
			Expect(err).NotTo(HaveOccurred())

			// Both DeleteAllOf calls must have run (store was already empty, remains empty).
			pubipList := &azurev1alpha1.PublicIPAddressList{}
			Expect(c.List(ctx, pubipList, client.InNamespace(namespace))).To(Succeed())
			Expect(pubipList.Items).To(BeEmpty())

			vmList := &azurev1alpha1.VirtualMachineList{}
			Expect(c.List(ctx, vmList, client.InNamespace(namespace))).To(Succeed())
			Expect(vmList.Items).To(BeEmpty())
		})

		It("should return RequeueAfterError if there are publicipaddresses remaining and timeout is not yet reached", func() {
			cp := newControlPlane()
			t := metav1.Now()
			cp.DeletionTimestamp = &t

			pubipWithFinalizers := newPubip(nil)
			pubipWithFinalizers.Finalizers = append(pubipWithFinalizers.Finalizers, "azure.remedy.gardener.cloud/publicipaddress")
			Expect(c.Create(ctx, pubipWithFinalizers)).To(Succeed())

			err := actuator.Delete(ctx, logger, cp, cluster)
			Expect(err).To(MatchError(&reconcilerutils.RequeueAfterError{RequeueAfter: gracefulDeletionWaitInterval}))
		})

		It("should forcefully remove remedy controller resources after grace period timeout has been reached", func() {
			cp := newControlPlane()
			t := metav1.NewTime(time.Now().Add(-2 * gracefulDeletionTimeout))
			cp.DeletionTimestamp = &t

			pubipWithFinalizers := newPubip(nil)
			pubipWithFinalizers.Finalizers = append(pubipWithFinalizers.Finalizers, "azure.remedy.gardener.cloud/publicipaddress")
			Expect(c.Create(ctx, pubipWithFinalizers)).To(Succeed())

			vmWithFinalizers := newVirtualMachine()
			vmWithFinalizers.Finalizers = append(vmWithFinalizers.Finalizers, "azure.remedy.gardener.cloud/virtualmachine")
			Expect(c.Create(ctx, vmWithFinalizers)).To(Succeed())

			a.EXPECT().Delete(ctx, logger, cp, cluster).Return(nil)

			err := actuator.Delete(ctx, logger, cp, cluster)
			Expect(err).NotTo(HaveOccurred())

			// Finalizers removed and DeleteAllOf called — both objects must be gone.
			pubipList := &azurev1alpha1.PublicIPAddressList{}
			Expect(c.List(ctx, pubipList, client.InNamespace(namespace))).To(Succeed())
			Expect(pubipList.Items).To(BeEmpty())

			vmList := &azurev1alpha1.VirtualMachineList{}
			Expect(c.List(ctx, vmList, client.InNamespace(namespace))).To(Succeed())
			Expect(vmList.Items).To(BeEmpty())
		})
	})

	Describe("#Migrate", func() {
		It("should remove finalizers from remedy controller resources and then delete them", func() {
			cp := newControlPlane()
			a.EXPECT().Migrate(ctx, logger, cp, cluster).Return(nil)

			pubipWithFinalizers := newPubip(nil)
			pubipWithFinalizers.Finalizers = append(pubipWithFinalizers.Finalizers, "azure.remedy.gardener.cloud/publicipaddress")
			Expect(c.Create(ctx, pubipWithFinalizers)).To(Succeed())

			vmWithFinalizers := newVirtualMachine()
			vmWithFinalizers.Finalizers = append(vmWithFinalizers.Finalizers, "azure.remedy.gardener.cloud/virtualmachine")
			Expect(c.Create(ctx, vmWithFinalizers)).To(Succeed())

			err := actuator.Migrate(ctx, logger, cp, cluster)
			Expect(err).NotTo(HaveOccurred())

			// Finalizers removed and DeleteAllOf called — both objects must be gone.
			pubipList := &azurev1alpha1.PublicIPAddressList{}
			Expect(c.List(ctx, pubipList, client.InNamespace(namespace))).To(Succeed())
			Expect(pubipList.Items).To(BeEmpty())

			vmList := &azurev1alpha1.VirtualMachineList{}
			Expect(c.List(ctx, vmList, client.InNamespace(namespace))).To(Succeed())
			Expect(vmList.Items).To(BeEmpty())
		})
	})
})
