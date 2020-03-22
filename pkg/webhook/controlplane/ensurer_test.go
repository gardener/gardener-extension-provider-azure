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
	"testing"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	extensionscontroller "github.com/gardener/gardener-extensions/pkg/controller"
	mockclient "github.com/gardener/gardener-extensions/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener-extensions/pkg/util"
	extensionswebhook "github.com/gardener/gardener-extensions/pkg/webhook"
	"github.com/gardener/gardener-extensions/pkg/webhook/controlplane/genericmutator"
	"github.com/gardener/gardener-extensions/pkg/webhook/controlplane/test"

	"github.com/coreos/go-systemd/unit"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

const (
	namespace                  = "test"
	cloudProviderConfigContent = "[Global]\nsome: content\n"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controlplane Webhook Suite")
}

var _ = Describe("Ensurer", func() {
	var (
		ctrl           *gomock.Controller
		dummyContext   = genericmutator.NewEnsurerContext(nil, nil)
		eContextK8s116 = genericmutator.NewInternalEnsurerContext(
			&extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.16.0",
						},
					},
				},
			},
		)
		eContextK8s117 = genericmutator.NewInternalEnsurerContext(
			&extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.17.0",
						},
					},
				},
			},
		)

		cmKey = client.ObjectKey{Namespace: namespace, Name: azure.CloudProviderConfigName}
		cm    = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: azure.CloudProviderConfigName},
			Data:       map[string]string{"abc": "xyz", azure.CloudProviderConfigMapKey: cloudProviderConfigContent},
		}

		annotations = map[string]string{
			"checksum/configmap-" + azure.CloudProviderConfigName: "31d2e116fbf854a590e84ab9176f299af6ff86aeea61bcee6bd705de78da9bf3",
		}

		kubeControllerManagerLabels = map[string]string{
			v1beta1constants.LabelNetworkPolicyToPublicNetworks:  v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToPrivateNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToBlockedCIDRs:    v1beta1constants.LabelNetworkPolicyAllowed,
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#EnsureKubeAPIServerDeployment", func() {
		It("should add missing elements to kube-apiserver deployment (k8s < 1.17)", func() {
			var (
				dep = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.DeploymentNameKubeAPIServer},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "kube-apiserver",
									},
								},
							},
						},
					},
				}
			)

			// Create mock client
			client := mockclient.NewMockClient(ctrl)
			client.EXPECT().Get(context.TODO(), cmKey, &corev1.ConfigMap{}).DoAndReturn(clientGet(cm))

			// Create ensurer
			ensurer := NewEnsurer(logger)
			err := ensurer.(inject.Client).InjectClient(client)
			Expect(err).To(Not(HaveOccurred()))

			// Call EnsureKubeAPIServerDeployment method and check the result
			err = ensurer.EnsureKubeAPIServerDeployment(context.TODO(), eContextK8s116, dep, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkKubeAPIServerDeployment(dep, annotations, true)
		})

		It("should add missing elements to kube-apiserver deployment (k8s >= 1.17)", func() {
			var (
				dep = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.DeploymentNameKubeAPIServer},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "kube-apiserver",
									},
								},
							},
						},
					},
				}
			)

			// Create mock client
			client := mockclient.NewMockClient(ctrl)
			client.EXPECT().Get(context.TODO(), cmKey, &corev1.ConfigMap{}).DoAndReturn(clientGet(cm))

			// Create ensurer
			ensurer := NewEnsurer(logger)
			err := ensurer.(inject.Client).InjectClient(client)
			Expect(err).To(Not(HaveOccurred()))

			// Call EnsureKubeAPIServerDeployment method and check the result
			err = ensurer.EnsureKubeAPIServerDeployment(context.TODO(), eContextK8s117, dep, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkKubeAPIServerDeployment(dep, annotations, false)
		})

		It("should modify existing elements of kube-apiserver deployment", func() {
			var (
				dep = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.DeploymentNameKubeAPIServer},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "kube-apiserver",
										Command: []string{
											"--cloud-provider=?",
											"--cloud-config=?",
											"--enable-admission-plugins=Priority,NamespaceLifecycle",
											"--disable-admission-plugins=PersistentVolumeLabel",
										},
										VolumeMounts: []corev1.VolumeMount{
											{Name: azure.CloudProviderConfigName, MountPath: "?"},
										},
									},
								},
								Volumes: []corev1.Volume{
									{Name: azure.CloudProviderConfigName},
								},
							},
						},
					},
				}
			)

			// Create mock client
			client := mockclient.NewMockClient(ctrl)
			client.EXPECT().Get(context.TODO(), cmKey, &corev1.ConfigMap{}).DoAndReturn(clientGet(cm))

			// Create ensurer
			ensurer := NewEnsurer(logger)
			err := ensurer.(inject.Client).InjectClient(client)
			Expect(err).To(Not(HaveOccurred()))

			// Call EnsureKubeAPIServerDeployment method and check the result
			err = ensurer.EnsureKubeAPIServerDeployment(context.TODO(), eContextK8s116, dep, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkKubeAPIServerDeployment(dep, annotations, true)
		})
	})

	Describe("#EnsureKubeControllerManagerDeployment", func() {
		It("should add missing elements to kube-controller-manager deployment (k8s < 1.17)", func() {
			var (
				dep = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.DeploymentNameKubeControllerManager},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "kube-controller-manager",
									},
								},
							},
						},
					},
				}
			)

			// Create mock client
			client := mockclient.NewMockClient(ctrl)
			client.EXPECT().Get(context.TODO(), cmKey, &corev1.ConfigMap{}).DoAndReturn(clientGet(cm))

			// Create ensurer
			ensurer := NewEnsurer(logger)
			err := ensurer.(inject.Client).InjectClient(client)
			Expect(err).To(Not(HaveOccurred()))

			// Call EnsureKubeControllerManagerDeployment method and check the result
			err = ensurer.EnsureKubeControllerManagerDeployment(context.TODO(), eContextK8s116, dep, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkKubeControllerManagerDeployment(dep, annotations, kubeControllerManagerLabels, true)
		})

		It("should add missing elements to kube-controller-manager deployment (k8s >= 1.17)", func() {
			var (
				dep = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.DeploymentNameKubeControllerManager},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "kube-controller-manager",
									},
								},
							},
						},
					},
				}
			)

			// Create mock client
			client := mockclient.NewMockClient(ctrl)
			client.EXPECT().Get(context.TODO(), cmKey, &corev1.ConfigMap{}).DoAndReturn(clientGet(cm))

			// Create ensurer
			ensurer := NewEnsurer(logger)
			err := ensurer.(inject.Client).InjectClient(client)
			Expect(err).To(Not(HaveOccurred()))

			// Call EnsureKubeControllerManagerDeployment method and check the result
			err = ensurer.EnsureKubeControllerManagerDeployment(context.TODO(), eContextK8s117, dep, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkKubeControllerManagerDeployment(dep, annotations, kubeControllerManagerLabels, false)
		})

		It("should modify existing elements of kube-controller-manager deployment", func() {
			var (
				dep = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.DeploymentNameKubeControllerManager},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "kube-controller-manager",
										Command: []string{
											"--cloud-provider=?",
											"--cloud-config=?",
											"--external-cloud-volume-plugin=?",
										},
										VolumeMounts: []corev1.VolumeMount{
											{Name: azure.CloudProviderConfigName, MountPath: "?"},
										},
									},
								},
								Volumes: []corev1.Volume{
									{Name: azure.CloudProviderConfigName},
								},
							},
						},
					},
				}
			)

			// Create mock client
			client := mockclient.NewMockClient(ctrl)
			client.EXPECT().Get(context.TODO(), cmKey, &corev1.ConfigMap{}).DoAndReturn(clientGet(cm))

			// Create ensurer
			ensurer := NewEnsurer(logger)
			err := ensurer.(inject.Client).InjectClient(client)
			Expect(err).To(Not(HaveOccurred()))

			// Call EnsureKubeControllerManagerDeployment method and check the result
			err = ensurer.EnsureKubeControllerManagerDeployment(context.TODO(), eContextK8s116, dep, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkKubeControllerManagerDeployment(dep, annotations, kubeControllerManagerLabels, true)
		})
	})

	Describe("#EnsureKubeletServiceUnitOptions", func() {
		var (
			acrCmKey   = client.ObjectKey{Namespace: namespace, Name: azure.CloudProviderAcrConfigName}
			acrContext = genericmutator.NewInternalEnsurerContext(
				&extensionscontroller.Cluster{
					Shoot: &gardencorev1beta1.Shoot{
						Status: gardencorev1beta1.ShootStatus{
							TechnicalID: namespace,
						},
					},
				},
			)
		)

		It("should modify existing elements of kubelet.service unit options", func() {
			var (
				oldUnitOptions = []*unit.UnitOption{
					{
						Section: "Service",
						Name:    "ExecStart",
						Value: `/opt/bin/hyperkube kubelet \
    --config=/var/lib/kubelet/config/kubelet`,
					},
				}
				newUnitOptions = []*unit.UnitOption{
					{
						Section: "Service",
						Name:    "ExecStart",
						Value: `/opt/bin/hyperkube kubelet \
    --config=/var/lib/kubelet/config/kubelet \
    --cloud-provider=azure \
    --cloud-config=/var/lib/kubelet/cloudprovider.conf`,
					},
				}
			)

			// Create mock client
			client := mockclient.NewMockClient(ctrl)
			client.EXPECT().Get(context.TODO(), acrCmKey, &corev1.ConfigMap{}).Return(errors.NewNotFound(schema.GroupResource{}, azure.CloudProviderAcrConfigName))

			// Create ensurer
			ensurer := NewEnsurer(logger)
			err := ensurer.(inject.Client).InjectClient(client)
			Expect(err).NotTo(HaveOccurred())

			// Call EnsureKubeletServiceUnitOptions method and check the result
			opts, err := ensurer.EnsureKubeletServiceUnitOptions(context.TODO(), acrContext, oldUnitOptions, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(opts).To(Equal(newUnitOptions))
		})

		It("should modify existing elements of kubelet.service unit options and add acr config", func() {
			var (
				acrCM = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: azure.CloudProviderAcrConfigName},
					Data:       map[string]string{},
				}
				oldUnitOptions = []*unit.UnitOption{
					{
						Section: "Service",
						Name:    "ExecStart",
						Value: `/opt/bin/hyperkube kubelet \
		--config=/var/lib/kubelet/config/kubelet`,
					},
				}
				newUnitOptions = []*unit.UnitOption{
					{
						Section: "Service",
						Name:    "ExecStart",
						Value: `/opt/bin/hyperkube kubelet \
    --config=/var/lib/kubelet/config/kubelet \
    --cloud-provider=azure \
    --cloud-config=/var/lib/kubelet/cloudprovider.conf \
    --azure-container-registry-config=/var/lib/kubelet/acr.conf`,
					},
				}
			)

			// Create mock client
			client := mockclient.NewMockClient(ctrl)
			client.EXPECT().Get(context.TODO(), acrCmKey, &corev1.ConfigMap{}).DoAndReturn(clientGet(acrCM))

			// Create ensurer
			ensurer := NewEnsurer(logger)
			err := ensurer.(inject.Client).InjectClient(client)
			Expect(err).NotTo(HaveOccurred())

			// Call EnsureKubeletServiceUnitOptions method and check the result
			opts, err := ensurer.EnsureKubeletServiceUnitOptions(context.TODO(), acrContext, oldUnitOptions, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(opts).To(Equal(newUnitOptions))
		})
	})

	Describe("#EnsureKubeletConfiguration", func() {
		It("should modify existing elements of kubelet configuration", func() {
			var (
				oldKubeletConfig = &kubeletconfigv1beta1.KubeletConfiguration{
					FeatureGates: map[string]bool{
						"Foo":                      true,
						"VolumeSnapshotDataSource": true,
						"CSINodeInfo":              true,
					},
				}
				newKubeletConfig = &kubeletconfigv1beta1.KubeletConfiguration{
					FeatureGates: map[string]bool{
						"Foo": true,
					},
				}
			)

			// Create ensurer
			ensurer := NewEnsurer(logger)

			// Call EnsureKubeletConfiguration method and check the result
			kubeletConfig := *oldKubeletConfig
			err := ensurer.EnsureKubeletConfiguration(context.TODO(), dummyContext, &kubeletConfig, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(&kubeletConfig).To(Equal(newKubeletConfig))
		})
	})

	Describe("#EnsureKubeletCloudProviderConfig", func() {
		var (
			cmKey = client.ObjectKey{Namespace: namespace, Name: azure.CloudProviderKubeletConfigName}
			cm    = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: azure.CloudProviderKubeletConfigName},
				Data:       map[string]string{"abc": "xyz", azure.CloudProviderConfigMapKey: cloudProviderConfigContent},
			}

			existingData = util.StringPtr("[LoadBalancer]\nlb-version=v2\nlb-provider:\n")
			emptydata    = util.StringPtr("")
		)
		It("cloud provider configmap do not exist", func() {
			// Create mock client
			client := mockclient.NewMockClient(ctrl)
			client.EXPECT().Get(context.TODO(), cmKey, &corev1.ConfigMap{}).Return(errors.NewNotFound(schema.GroupResource{}, cm.Name))

			// Create ensurer
			ensurer := NewEnsurer(logger)
			err := ensurer.(inject.Client).InjectClient(client)
			Expect(err).NotTo(HaveOccurred())

			// Call EnsureKubeletConfiguration method and check the result
			err = ensurer.EnsureKubeletCloudProviderConfig(context.TODO(), dummyContext, emptydata, namespace)
			Expect(err).To(Not(HaveOccurred()))
			Expect(*emptydata).To(Equal(""))
		})
		It("should create element containing cloud provider config content", func() {
			// Create mock client
			client := mockclient.NewMockClient(ctrl)
			client.EXPECT().Get(context.TODO(), cmKey, &corev1.ConfigMap{}).DoAndReturn(clientGet(cm))

			// Create ensurer
			ensurer := NewEnsurer(logger)
			err := ensurer.(inject.Client).InjectClient(client)
			Expect(err).NotTo(HaveOccurred())

			// Call EnsureKubeletConfiguration method and check the result
			err = ensurer.EnsureKubeletCloudProviderConfig(context.TODO(), dummyContext, emptydata, namespace)
			Expect(err).To(Not(HaveOccurred()))
			Expect(*emptydata).To(Equal(cloudProviderConfigContent))
		})
		It("should modify existing element containing cloud provider config content", func() {
			// Create mock client
			client := mockclient.NewMockClient(ctrl)
			client.EXPECT().Get(context.TODO(), cmKey, &corev1.ConfigMap{}).DoAndReturn(clientGet(cm))

			// Create ensurer
			ensurer := NewEnsurer(logger)
			err := ensurer.(inject.Client).InjectClient(client)
			Expect(err).NotTo(HaveOccurred())

			// Call EnsureKubeletConfiguration method and check the result
			err = ensurer.EnsureKubeletCloudProviderConfig(context.TODO(), dummyContext, existingData, namespace)
			Expect(err).To(Not(HaveOccurred()))
			Expect(*existingData).To(Equal(cloudProviderConfigContent))
		})
	})
})

func checkKubeAPIServerDeployment(dep *appsv1.Deployment, annotations map[string]string, k8sVersionLessThan117 bool) {
	// Check that the kube-apiserver container still exists and contains all needed command line args,
	// env vars, and volume mounts
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-apiserver")
	Expect(c).To(Not(BeNil()))
	Expect(c.Command).To(ContainElement("--cloud-provider=azure"))
	Expect(c.Command).To(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
	Expect(c.Command).To(test.ContainElementWithPrefixContaining("--enable-admission-plugins=", "PersistentVolumeLabel", ","))
	Expect(c.Command).To(Not(test.ContainElementWithPrefixContaining("--disable-admission-plugins=", "PersistentVolumeLabel", ",")))
	Expect(c.VolumeMounts).To(ContainElement(cloudProviderConfigVolumeMount))
	Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(cloudProviderConfigVolume))

	if !k8sVersionLessThan117 {
		Expect(c.VolumeMounts).To(ContainElement(etcSSLVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(etcSSLVolume))
	}

	// Check that the Pod template contains all needed checksum annotations
	Expect(dep.Spec.Template.Annotations).To(Equal(annotations))
}

func checkKubeControllerManagerDeployment(dep *appsv1.Deployment, annotations, labels map[string]string, k8sVersionLessThan117 bool) {
	// Check that the kube-controller-manager container still exists and contains all needed command line args,
	// env vars, and volume mounts
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-controller-manager")
	Expect(c).To(Not(BeNil()))
	Expect(c.Command).To(ContainElement("--cloud-provider=external"))
	Expect(c.Command).To(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
	Expect(c.Command).To(ContainElement("--external-cloud-volume-plugin=azure"))
	Expect(c.VolumeMounts).To(ContainElement(cloudProviderConfigVolumeMount))
	Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(cloudProviderConfigVolume))

	if !k8sVersionLessThan117 {
		Expect(c.VolumeMounts).To(ContainElement(etcSSLVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(etcSSLVolume))
	}

	// Check that the Pod template contains all needed checksum annotations
	Expect(dep.Spec.Template.Annotations).To(Equal(annotations))

	// Check that the labels for network policies are added
	Expect(dep.Spec.Template.Labels).To(Equal(labels))
}

func clientGet(result runtime.Object) interface{} {
	return func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
		switch obj.(type) {
		case *corev1.Secret:
			*obj.(*corev1.Secret) = *result.(*corev1.Secret)
		case *corev1.ConfigMap:
			*obj.(*corev1.ConfigMap) = *result.(*corev1.ConfigMap)
		}
		return nil
	}
}
