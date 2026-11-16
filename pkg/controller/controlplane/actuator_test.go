// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	mockcontrolplane "github.com/gardener/gardener/extensions/pkg/controller/controlplane/mock"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	testutils "github.com/gardener/gardener/pkg/utils/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gardener/gardener-extension-provider-azure/pkg/features"
)

var _ = Describe("Actuator", func() {
	var (
		ctrl   *gomock.Controller
		ctx    = context.TODO()
		logger = log.Log.WithName("test")

		c      client.Client
		scheme *runtime.Scheme
		mgr    testutils.FakeManager
		a      *mockcontrolplane.MockActuator
		act    *actuator

		gracefulDeletionTimeout      time.Duration
		gracefulDeletionWaitInterval time.Duration
		newControlPlane              = func() *extensionsv1alpha1.ControlPlane {
			return &extensionsv1alpha1.ControlPlane{
				ObjectMeta: metav1.ObjectMeta{Name: "control-plane", Namespace: namespace},
				Spec:       extensionsv1alpha1.ControlPlaneSpec{},
			}
		}
		cluster  *extensionscontroller.Cluster
		newPubip = func() *unstructured.Unstructured {
			return &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": remedyAPIVersion,
				"kind":       publicIPAddressKind,
				"metadata":   map[string]interface{}{"name": "foo-1.2.3.4", "namespace": namespace},
				"spec":       map[string]interface{}{"ipAddress": "1.2.3.4"},
			}}
		}

		newVirtualMachine = func() *unstructured.Unstructured {
			return &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": remedyAPIVersion,
				"kind":       virtualMachineKind,
				"metadata":   map[string]interface{}{"name": "node-name", "namespace": namespace},
			}}
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cluster = &extensionscontroller.Cluster{
			Shoot: &gardencorev1beta1.Shoot{
				Spec:   gardencorev1beta1.ShootSpec{Hibernation: &gardencorev1beta1.Hibernation{Enabled: ptr.To(true)}},
				Status: gardencorev1beta1.ShootStatus{IsHibernated: true},
			},
		}

		scheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(extensionsv1alpha1.AddToScheme(scheme)).To(Succeed())

		c = fakeclient.NewClientBuilder().WithScheme(scheme).Build()
		mgr = testutils.FakeManager{Client: c}

		a = mockcontrolplane.NewMockActuator(ctrl)
		gracefulDeletionTimeout = 10 * time.Second
		gracefulDeletionWaitInterval = 1 * time.Second
		act = NewActuator(mgr, a, gracefulDeletionTimeout, gracefulDeletionWaitInterval).(*actuator)
	})

	Describe("#Delete", func() {
		It("should successfully delete controlplane if there are no remedy controller resources", func() {
			cp := newControlPlane()
			t := metav1.Now()
			cp.DeletionTimestamp = &t

			a.EXPECT().Delete(ctx, logger, cp, cluster).Return(nil)
			err := act.Delete(ctx, logger, cp, cluster)
			Expect(err).NotTo(HaveOccurred())

			// Both DeleteAllOf calls must have run (store was already empty, remains empty).
			pubipList := newRemedyResourceList(publicIPAddressKind)
			Expect(c.List(ctx, pubipList, client.InNamespace(namespace))).To(Succeed())
			Expect(pubipList.Items).To(BeEmpty())

			vmList := newRemedyResourceList(virtualMachineKind)
			Expect(c.List(ctx, vmList, client.InNamespace(namespace))).To(Succeed())
			Expect(vmList.Items).To(BeEmpty())
		})

		It("should return RequeueAfterError if there are publicipaddresses remaining and timeout is not yet reached", func() {
			featuregatetesting.SetFeatureGateDuringTest(GinkgoT(), features.ExtensionFeatureGate, features.DisableRemedyController, false)
			cp := newControlPlane()
			t := metav1.Now()
			cp.DeletionTimestamp = &t

			pubipWithFinalizers := newPubip()
			pubipWithFinalizers.SetFinalizers([]string{publicIpFinalizerString})
			Expect(c.Create(ctx, pubipWithFinalizers)).To(Succeed())

			err := act.Delete(ctx, logger, cp, cluster)
			Expect(err).To(MatchError(&reconcilerutils.RequeueAfterError{RequeueAfter: gracefulDeletionWaitInterval}))
		})

		It("should forcefully remove remedy controller resources after grace period timeout has been reached", func() {
			featuregatetesting.SetFeatureGateDuringTest(GinkgoT(), features.ExtensionFeatureGate, features.DisableRemedyController, false)
			cp := newControlPlane()
			t := metav1.NewTime(time.Now().Add(-2 * gracefulDeletionTimeout))
			cp.DeletionTimestamp = &t

			pubipWithFinalizers := newPubip()
			pubipWithFinalizers.SetFinalizers([]string{publicIpFinalizerString})
			Expect(c.Create(ctx, pubipWithFinalizers)).To(Succeed())

			vmWithFinalizers := newVirtualMachine()
			vmWithFinalizers.SetFinalizers([]string{virtualMachineFinalizerString})
			Expect(c.Create(ctx, vmWithFinalizers)).To(Succeed())

			a.EXPECT().Delete(ctx, logger, cp, cluster).Return(nil)

			err := act.Delete(ctx, logger, cp, cluster)
			Expect(err).NotTo(HaveOccurred())

			// Finalizers removed and DeleteAllOf called — both objects must be gone.
			pubipList := newRemedyResourceList(publicIPAddressKind)
			Expect(c.List(ctx, pubipList, client.InNamespace(namespace))).To(Succeed())
			Expect(pubipList.Items).To(BeEmpty())

			vmList := newRemedyResourceList(virtualMachineKind)
			Expect(c.List(ctx, vmList, client.InNamespace(namespace))).To(Succeed())
			Expect(vmList.Items).To(BeEmpty())
		})
	})

	Describe("legacy resource cleanup", func() {
		It("should clean up seed resources by default even while the shoot is hibernated", func() {
			pubip := newPubip()
			pubip.SetFinalizers([]string{publicIpFinalizerString})
			vm := newVirtualMachine()
			vm.SetFinalizers([]string{virtualMachineFinalizerString})
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: remedyShootAccessSecretName, Namespace: namespace}}
			for _, obj := range []client.Object{pubip, vm, secret} {
				Expect(c.Create(ctx, obj)).To(Succeed())
			}
			cp := newControlPlane()
			a.EXPECT().Reconcile(ctx, logger, cp, cluster).Return(true, nil)

			requeue, err := act.Reconcile(ctx, logger, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(requeue).To(BeTrue())
			for _, obj := range []client.Object{pubip, vm, secret} {
				Expect(apierrors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(obj), obj))).To(BeTrue())
			}
		})

		DescribeTable("should tolerate removed CRDs", func(apiErr error) {
			act.client = fakeclient.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
				List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
					return apiErr
				},
			}).Build()
			Expect(act.forceDeleteSeedRemedyControllerResources(ctx, logger, namespace)).To(Succeed())
		},
			Entry("no REST mapping", &meta.NoKindMatchError{GroupKind: schema.GroupKind{Group: "azure.remedy.gardener.cloud", Kind: publicIPAddressKind}}),
			Entry("API endpoint removed", apierrors.NewNotFound(schema.GroupResource{Group: "azure.remedy.gardener.cloud", Resource: "publicipaddresses"}, "")),
		)

		It("should tolerate CRDs disappearing between listing and deletion", func() {
			act.client = fakeclient.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
				DeleteAllOf: func(context.Context, client.WithWatch, client.Object, ...client.DeleteAllOfOption) error {
					return apierrors.NewNotFound(schema.GroupResource{Group: "azure.remedy.gardener.cloud", Resource: "publicipaddresses"}, "")
				},
			}).Build()
			Expect(act.forceDeleteSeedRemedyControllerResources(ctx, logger, namespace)).To(Succeed())
		})

		It("should preserve unrelated finalizers and resources in other namespaces", func() {
			pubip := newPubip()
			pubip.SetFinalizers([]string{publicIpFinalizerString, "example.com/other"})
			other := newVirtualMachine()
			other.SetNamespace("other")
			other.SetFinalizers([]string{virtualMachineFinalizerString})
			Expect(c.Create(ctx, pubip)).To(Succeed())
			Expect(c.Create(ctx, other)).To(Succeed())

			Expect(act.forceDeleteSeedRemedyControllerResources(ctx, logger, namespace)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(pubip), pubip)).To(Succeed())
			Expect(pubip.GetFinalizers()).To(ConsistOf("example.com/other"))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(other), other)).To(Succeed())
			Expect(other.GetFinalizers()).To(ConsistOf(virtualMachineFinalizerString))
		})

		It("should propagate API errors other than missing CRDs", func() {
			apiErr := apierrors.NewForbidden(schema.GroupResource{Group: "azure.remedy.gardener.cloud", Resource: "publicipaddresses"}, "", fmt.Errorf("denied"))
			act.client = fakeclient.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
				List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
					return apiErr
				},
			}).Build()
			Expect(act.forceDeleteSeedRemedyControllerResources(ctx, logger, namespace)).To(MatchError(ContainSubstring("denied")))
		})

		DescribeTable("should clean up shoot finalizers without seed CRDs", func(operation string) {
			cluster.Shoot.Spec.Hibernation = nil
			cluster.Shoot.Status.IsHibernated = false
			patches := make(chan string, 2)
			service := &corev1.Service{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
				ObjectMeta: metav1.ObjectMeta{Name: "lb", Namespace: "default", ResourceVersion: "1", Finalizers: []string{serviceFinalizerName, "example.com/other"}},
				Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
			}
			node := &corev1.Node{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Node"},
				ObjectMeta: metav1.ObjectMeta{Name: "node", ResourceVersion: "1", Finalizers: []string{nodeFinalizerName, "example.com/other"}},
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer GinkgoRecover()
				w.Header().Set("Content-Type", "application/json")
				var response interface{}
				switch r.URL.Path {
				case "/api":
					response = &metav1.APIVersions{Versions: []string{"v1"}}
				case "/apis":
					response = &metav1.APIGroupList{}
				case "/api/v1":
					response = &metav1.APIResourceList{GroupVersion: "v1", APIResources: []metav1.APIResource{
						{Name: "services", Kind: "Service", Namespaced: true},
						{Name: "nodes", Kind: "Node"},
					}}
				case "/api/v1/services":
					response = &corev1.ServiceList{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ServiceList"}, Items: []corev1.Service{*service}}
				case "/api/v1/nodes":
					response = &corev1.NodeList{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "NodeList"}, Items: []corev1.Node{*node}}
				case "/api/v1/namespaces/default/services/lb":
					response = service
				case "/api/v1/nodes/node":
					response = node
				default:
					http.NotFound(w, r)
					return
				}
				if r.Method == http.MethodPatch {
					var patch map[string]interface{}
					Expect(json.NewDecoder(r.Body).Decode(&patch)).To(Succeed())
					Expect(patch["metadata"]).To(HaveKeyWithValue("finalizers", []interface{}{"example.com/other"}))
					patches <- r.URL.Path
				}
				Expect(json.NewEncoder(w).Encode(response)).To(Succeed())
			}))
			DeferCleanup(server.Close)
			kubeconfig := fmt.Sprintf("apiVersion: v1\nkind: Config\nclusters:\n- name: shoot\n  cluster:\n    server: %s\ncontexts:\n- name: shoot\n  context:\n    cluster: shoot\ncurrent-context: shoot\n", server.URL)
			Expect(c.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "gardener", Namespace: namespace},
				Data:       map[string][]byte{"kubeconfig": []byte(kubeconfig)},
			})).To(Succeed())
			act.client = interceptor.NewClient(c.(client.WithWatch), interceptor.Funcs{
				List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
					return &meta.NoKindMatchError{GroupKind: schema.GroupKind{Group: "azure.remedy.gardener.cloud", Kind: publicIPAddressKind}}
				},
			})
			cp := newControlPlane()
			switch operation {
			case "Reconcile":
				a.EXPECT().Reconcile(ctx, logger, cp, cluster).Return(false, nil)
				_, err := act.Reconcile(ctx, logger, cp, cluster)
				Expect(err).NotTo(HaveOccurred())
			case "Delete":
				a.EXPECT().Delete(ctx, logger, cp, cluster).Return(nil)
				Expect(act.Delete(ctx, logger, cp, cluster)).To(Succeed())
			case "Migrate":
				a.EXPECT().Migrate(ctx, logger, cp, cluster).Return(nil)
				Expect(act.Migrate(ctx, logger, cp, cluster)).To(Succeed())
			}
			Expect(patches).To(HaveLen(2))
			Expect(patches).To(Receive(Equal("/api/v1/namespaces/default/services/lb")))
			Expect(patches).To(Receive(Equal("/api/v1/nodes/node")))
		},
			Entry("during reconciliation", "Reconcile"),
			Entry("during deletion", "Delete"),
			Entry("during migration", "Migrate"),
		)
	})

	Describe("#Migrate", func() {
		It("should remove finalizers from remedy controller resources and then delete them", func() {
			cp := newControlPlane()
			a.EXPECT().Migrate(ctx, logger, cp, cluster).Return(nil)

			pubipWithFinalizers := newPubip()
			pubipWithFinalizers.SetFinalizers([]string{publicIpFinalizerString})
			Expect(c.Create(ctx, pubipWithFinalizers)).To(Succeed())

			vmWithFinalizers := newVirtualMachine()
			vmWithFinalizers.SetFinalizers([]string{virtualMachineFinalizerString})
			Expect(c.Create(ctx, vmWithFinalizers)).To(Succeed())

			err := act.Migrate(ctx, logger, cp, cluster)
			Expect(err).NotTo(HaveOccurred())

			// Finalizers removed and DeleteAllOf called — both objects must be gone.
			pubipList := newRemedyResourceList(publicIPAddressKind)
			Expect(c.List(ctx, pubipList, client.InNamespace(namespace))).To(Succeed())
			Expect(pubipList.Items).To(BeEmpty())

			vmList := newRemedyResourceList(virtualMachineKind)
			Expect(c.List(ctx, vmList, client.InNamespace(namespace))).To(Succeed())
			Expect(vmList.Items).To(BeEmpty())
		})
	})
})
