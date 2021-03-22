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

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane"
	azurev1alpha1 "github.com/gardener/remedy-controller/pkg/apis/azure/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mockcontrolplane "github.com/gardener/gardener/extensions/pkg/controller/controlplane/mock"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

		newControlPlane = func(purpose *extensionsv1alpha1.Purpose) *extensionsv1alpha1.ControlPlane {
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
					Name:        "foo-1.2.3.4",
					Namespace:   namespace,
					Annotations: annotations,
				},
				Spec: azurev1alpha1.PublicIPAddressSpec{
					IPAddress: "1.2.3.4",
				},
			}
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		a = mockcontrolplane.NewMockActuator(ctrl)

		actuator = NewActuator(a, logger)

		err := actuator.(inject.Client).InjectClient(c)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Delete", func() {
		It("should delete remaining remedy controller resources", func() {
			pubip := newPubip(nil)
			cp := newControlPlane(nil)
			c.EXPECT().List(ctx, &azurev1alpha1.PublicIPAddressList{}, client.InNamespace(namespace)).
				DoAndReturn(func(_ context.Context, list *azurev1alpha1.PublicIPAddressList, _ ...client.ListOption) error {
					list.Items = []azurev1alpha1.PublicIPAddress{*pubip}
					return nil
				})
			annotatedPubip := newPubip(map[string]string{"azure.remedy.gardener.cloud/do-not-clean": "true"})
			c.EXPECT().Update(ctx, annotatedPubip).Return(nil)
			c.EXPECT().DeleteAllOf(ctx, &azurev1alpha1.PublicIPAddress{}, client.InNamespace(namespace)).Return(nil)
			c.EXPECT().DeleteAllOf(ctx, &azurev1alpha1.VirtualMachine{}, client.InNamespace(namespace)).Return(nil)
			c.EXPECT().List(gomock.Any(), &azurev1alpha1.PublicIPAddressList{}, client.InNamespace(namespace)).
				DoAndReturn(func(_ context.Context, list *azurev1alpha1.PublicIPAddressList, _ ...client.ListOption) error {
					list.Items = []azurev1alpha1.PublicIPAddress{}
					return nil
				})
			c.EXPECT().List(gomock.Any(), &azurev1alpha1.VirtualMachineList{}, client.InNamespace(namespace)).
				DoAndReturn(func(_ context.Context, list *azurev1alpha1.VirtualMachineList, _ ...client.ListOption) error {
					list.Items = []azurev1alpha1.VirtualMachine{}
					return nil
				})
			a.EXPECT().Delete(ctx, cp, cluster).Return(nil)

			err := actuator.Delete(ctx, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not delete remaining remedy controller resources for controlplane with purpose exposure", func() {
			exposure := extensionsv1alpha1.Exposure
			cp := newControlPlane(&exposure)
			a.EXPECT().Delete(ctx, cp, cluster).Return(nil)

			err := actuator.Delete(ctx, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
