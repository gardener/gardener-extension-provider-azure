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

	"github.com/coreos/go-systemd/unit"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/csimigration"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/test"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/utils/version"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	RunSpecs(t, "ControlPlane Webhook Suite")
}

var _ = Describe("Ensurer", func() {
	var (
		ctx  = context.TODO()
		ctrl *gomock.Controller

		ensurer genericmutator.Ensurer
		c       *mockclient.MockClient

		dummyContext   = genericmutator.NewEnsurerContext(nil, nil)
		eContextK8s116 = genericmutator.NewInternalEnsurerContext(
			&extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.16.0",
						},
					},
					Status: gardencorev1beta1.ShootStatus{
						TechnicalID: namespace,
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
					Status: gardencorev1beta1.ShootStatus{
						TechnicalID: namespace,
					},
				},
			},
		)
		eContextK8s119 = genericmutator.NewInternalEnsurerContext(
			&extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.19.0",
						},
					},
					Status: gardencorev1beta1.ShootStatus{
						TechnicalID: namespace,
					},
				},
			},
		)
		eContextK8s119WithCSIAnnotation = genericmutator.NewInternalEnsurerContext(
			&extensionscontroller.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						csimigration.AnnotationKeyNeedsComplete: "true",
					},
				},
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.19.0",
						},
					},
					Status: gardencorev1beta1.ShootStatus{
						TechnicalID: namespace,
					},
				},
			},
		)

		key = client.ObjectKey{Namespace: namespace, Name: azure.CloudProviderConfigName}
		cm  = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: azure.CloudProviderConfigName},
			Data:       map[string]string{"abc": "xyz", azure.CloudProviderConfigMapKey: cloudProviderConfigContent},
		}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: azure.CloudProviderConfigName},
			Data:       map[string][]byte{"abc": []byte("xyz"), azure.CloudProviderConfigMapKey: []byte(cloudProviderConfigContent)},
		}

		annotations = map[string]string{
			"checksum/secret-" + azure.CloudProviderConfigName: "546bca950d25ff0b53fe8b7d7e2cee183f61524d4e3207f9e4db953ee06bc48d",
		}

		annotationsConfigMap = map[string]string{
			"checksum/configmap-" + azure.CloudProviderConfigName: "31d2e116fbf854a590e84ab9176f299af6ff86aeea61bcee6bd705de78da9bf3",
		}

		kubeControllerManagerLabels = map[string]string{
			v1beta1constants.LabelNetworkPolicyToPublicNetworks:  v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToPrivateNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)

		ensurer = NewEnsurer(logger)
		err := ensurer.(inject.Client).InjectClient(c)
		Expect(err).To(Not(HaveOccurred()))
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#EnsureKubeAPIServerDeployment", func() {
		var dep *appsv1.Deployment

		BeforeEach(func() {
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
		})

		It("should add missing elements to kube-apiserver deployment (k8s < 1.17)", func() {
			c.EXPECT().Get(ctx, key, &corev1.Secret{}).DoAndReturn(clientGet(secret))

			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s116, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, annotations, "1.16.0", false)
		})

		It("should add missing elements to kube-apiserver deployment (k8s >= 1.17, < 1.19)", func() {
			c.EXPECT().Get(ctx, key, &corev1.Secret{}).DoAndReturn(clientGet(secret))

			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s117, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, annotations, "1.17.4", false)
		})

		It("should add missing elements to kube-apiserver deployment (k8s >= 1.19)", func() {
			c.EXPECT().Get(ctx, key, &corev1.Secret{}).DoAndReturn(clientGet(secret))

			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s119, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, annotations, "1.19.0", false)
		})

		It("should add missing elements to kube-apiserver deployment (k8s >= 1.19 w/ CSI annotation)", func() {
			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s119WithCSIAnnotation, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, nil, "1.19.0", true)
		})

		It("should modify existing elements of kube-apiserver deployment", func() {
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

			c.EXPECT().Get(ctx, key, &corev1.Secret{}).DoAndReturn(clientGet(secret))

			Expect(ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s116, dep, nil)).To(Not(HaveOccurred()))
			checkKubeAPIServerDeployment(dep, annotations, "1.16.0", false)
		})
	})

	Describe("#EnsureKubeControllerManagerDeployment", func() {
		var dep *appsv1.Deployment

		BeforeEach(func() {
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
		})

		It("should add missing elements to kube-controller-manager deployment (k8s < 1.17)", func() {
			c.EXPECT().Get(ctx, key, &corev1.Secret{}).Return(apierrors.NewNotFound(schema.GroupResource{}, "Secret"))
			c.EXPECT().Get(ctx, key, &corev1.ConfigMap{}).DoAndReturn(clientGet(cm))

			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s116, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, annotationsConfigMap, kubeControllerManagerLabels, "1.16.4", false)
		})

		It("should add missing elements to kube-controller-manager deployment (k8s >= 1.17, k8s < 1.19)", func() {
			c.EXPECT().Get(ctx, key, &corev1.Secret{}).DoAndReturn(clientGet(secret))

			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s117, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, annotations, kubeControllerManagerLabels, "1.17.8", false)
		})

		It("should add missing elements to kube-controller-manager deployment (k8s >= 1.19 w/o CSI annotation)", func() {
			c.EXPECT().Get(ctx, key, &corev1.Secret{}).DoAndReturn(clientGet(secret))

			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s119, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, annotations, kubeControllerManagerLabels, "1.19.0", false)
		})

		It("should add missing elements to kube-controller-manager deployment (k8s >= 1.19 w/ CSI annotation)", func() {
			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s119WithCSIAnnotation, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, nil, nil, "1.18.0", true)
		})

		It("should modify existing elements of kube-controller-manager deployment", func() {
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

			c.EXPECT().Get(ctx, key, &corev1.Secret{}).DoAndReturn(clientGet(secret))

			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s116, dep, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkKubeControllerManagerDeployment(dep, annotations, kubeControllerManagerLabels, "1.16.0", false)
		})
	})

	Describe("#EnsureKubeSchedulerDeployment", func() {
		var dep *appsv1.Deployment

		BeforeEach(func() {
			dep = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.DeploymentNameKubeControllerManager},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "kube-scheduler",
								},
							},
						},
					},
				},
			}

			ensurer = NewEnsurer(logger)
		})

		It("should add missing elements to kube-scheduler deployment (k8s < 1.19)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s117, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.17.0", false)
		})

		It("should add missing elements to kube-scheduler deployment (k8s >= 1.19 w/o CSI annotation)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s119, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.19.0", false)
		})

		It("should add missing elements to kube-scheduler deployment (k8s >= 1.19 w/ CSI annotation)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s119WithCSIAnnotation, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.19.0", true)
		})
	})

	Describe("#EnsureKubeletServiceUnitOptions", func() {
		var (
			acrCmKey = client.ObjectKey{Namespace: namespace, Name: azure.CloudProviderAcrConfigName}

			oldUnitOptions []*unit.UnitOption
		)

		BeforeEach(func() {
			oldUnitOptions = []*unit.UnitOption{
				{
					Section: "Service",
					Name:    "ExecStart",
					Value: `/opt/bin/hyperkube kubelet \
    --config=/var/lib/kubelet/config/kubelet`,
				},
			}
		})

		It("should modify existing elements of kubelet.service unit options (k8s < 1.19)", func() {
			newUnitOptions := []*unit.UnitOption{
				{
					Section: "Service",
					Name:    "ExecStart",
					Value: `/opt/bin/hyperkube kubelet \
    --config=/var/lib/kubelet/config/kubelet \
    --cloud-provider=azure \
    --cloud-config=/var/lib/kubelet/cloudprovider.conf`,
				},
			}

			c.EXPECT().Get(ctx, acrCmKey, &corev1.ConfigMap{}).Return(apierrors.NewNotFound(schema.GroupResource{}, azure.CloudProviderAcrConfigName))

			opts, err := ensurer.EnsureKubeletServiceUnitOptions(ctx, eContextK8s117, oldUnitOptions, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(opts).To(Equal(newUnitOptions))
		})

		It("should modify existing elements of kubelet.service unit options and add acr config (k8s < 1.19)", func() {
			var (
				acrCM = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: azure.CloudProviderAcrConfigName},
					Data:       map[string]string{},
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

			c.EXPECT().Get(ctx, acrCmKey, &corev1.ConfigMap{}).DoAndReturn(clientGet(acrCM))

			opts, err := ensurer.EnsureKubeletServiceUnitOptions(ctx, eContextK8s117, oldUnitOptions, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(opts).To(Equal(newUnitOptions))
		})

		It("should modify existing elements of kubelet.service unit options (k8s >= 1.19)", func() {
			newUnitOptions := []*unit.UnitOption{
				{
					Section: "Service",
					Name:    "ExecStart",
					Value: `/opt/bin/hyperkube kubelet \
    --config=/var/lib/kubelet/config/kubelet \
    --cloud-provider=external \
    --enable-controller-attach-detach=true`,
				},
			}

			c.EXPECT().Get(ctx, acrCmKey, &corev1.ConfigMap{}).Return(apierrors.NewNotFound(schema.GroupResource{}, azure.CloudProviderAcrConfigName))

			opts, err := ensurer.EnsureKubeletServiceUnitOptions(ctx, eContextK8s119, oldUnitOptions, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(opts).To(Equal(newUnitOptions))
		})

		It("should modify existing elements of kubelet.service unit options and add acr config (k8s >= 1.19)", func() {
			var (
				acrCM = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: azure.CloudProviderAcrConfigName},
					Data:       map[string]string{},
				}
				newUnitOptions = []*unit.UnitOption{
					{
						Section: "Service",
						Name:    "ExecStart",
						Value: `/opt/bin/hyperkube kubelet \
    --config=/var/lib/kubelet/config/kubelet \
    --cloud-provider=external \
    --enable-controller-attach-detach=true \
    --azure-container-registry-config=/var/lib/kubelet/acr.conf`,
					},
				}
			)

			c.EXPECT().Get(ctx, acrCmKey, &corev1.ConfigMap{}).DoAndReturn(clientGet(acrCM))

			opts, err := ensurer.EnsureKubeletServiceUnitOptions(ctx, eContextK8s119, oldUnitOptions, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(opts).To(Equal(newUnitOptions))
		})
	})

	Describe("#EnsureKubeletConfiguration", func() {
		var oldKubeletConfig *kubeletconfigv1beta1.KubeletConfiguration

		BeforeEach(func() {
			oldKubeletConfig = &kubeletconfigv1beta1.KubeletConfiguration{
				FeatureGates: map[string]bool{
					"Foo": true,
				},
			}
		})

		It("should modify existing elements of kubelet configuration (k8s < 1.19)", func() {
			newKubeletConfig := &kubeletconfigv1beta1.KubeletConfiguration{
				FeatureGates: map[string]bool{
					"Foo": true,
				},
			}
			kubeletConfig := *oldKubeletConfig

			err := ensurer.EnsureKubeletConfiguration(ctx, eContextK8s117, &kubeletConfig, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(&kubeletConfig).To(Equal(newKubeletConfig))
		})

		It("should modify existing elements of kubelet configuration (k8s >= 1.19)", func() {
			newKubeletConfig := &kubeletconfigv1beta1.KubeletConfiguration{
				FeatureGates: map[string]bool{
					"Foo":                           true,
					"CSIMigration":                  true,
					"CSIMigrationAzureDisk":         true,
					"CSIMigrationAzureFile":         true,
					"CSIMigrationAzureDiskComplete": true,
					"CSIMigrationAzureFileComplete": true,
				},
			}
			kubeletConfig := *oldKubeletConfig

			err := ensurer.EnsureKubeletConfiguration(ctx, eContextK8s119, &kubeletConfig, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(&kubeletConfig).To(Equal(newKubeletConfig))
		})
	})

	Describe("#ShouldProvisionKubeletCloudProviderConfig", func() {
		It("should return true (k8s < 1.19)", func() {
			Expect(ensurer.ShouldProvisionKubeletCloudProviderConfig(ctx, eContextK8s117)).To(BeTrue())
		})

		It("should return false (k8s >= 1.19)", func() {
			Expect(ensurer.ShouldProvisionKubeletCloudProviderConfig(ctx, eContextK8s119)).To(BeFalse())
		})
	})

	Describe("#EnsureKubeletCloudProviderConfig", func() {
		var (
			objKey = client.ObjectKey{Namespace: namespace, Name: azure.CloudProviderDiskConfigName}
			cm     = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: azure.CloudProviderDiskConfigName},
				Data:       map[string]string{"abc": "xyz", azure.CloudProviderConfigMapKey: cloudProviderConfigContent},
			}
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: azure.CloudProviderDiskConfigName},
				Data:       map[string][]byte{"abc": []byte("xyz"), azure.CloudProviderConfigMapKey: []byte(cloudProviderConfigContent)},
			}

			existingData = util.StringPtr("[LoadBalancer]\nlb-version=v2\nlb-provider:\n")
			emptydata    = util.StringPtr("")
		)

		It("cloud provider secret or configmap do not exist", func() {
			c.EXPECT().Get(ctx, objKey, &corev1.Secret{}).Return(apierrors.NewNotFound(schema.GroupResource{}, cm.Name))
			c.EXPECT().Get(ctx, objKey, &corev1.ConfigMap{}).Return(apierrors.NewNotFound(schema.GroupResource{}, cm.Name))

			err := ensurer.EnsureKubeletCloudProviderConfig(ctx, dummyContext, emptydata, namespace)
			Expect(err).To(Not(HaveOccurred()))
			Expect(*emptydata).To(Equal(""))
		})

		It("should create element containing cloud provider config content with configmap", func() {
			c.EXPECT().Get(ctx, objKey, &corev1.Secret{}).Return(apierrors.NewNotFound(schema.GroupResource{}, cm.Name))
			c.EXPECT().Get(ctx, objKey, &corev1.ConfigMap{}).DoAndReturn(clientGet(cm))

			err := ensurer.EnsureKubeletCloudProviderConfig(ctx, dummyContext, emptydata, namespace)
			Expect(err).To(Not(HaveOccurred()))
			Expect(*emptydata).To(Equal(cloudProviderConfigContent))
		})

		It("should create element containing cloud provider config content with secret", func() {
			c.EXPECT().Get(ctx, objKey, &corev1.Secret{}).DoAndReturn(clientGet(secret))

			err := ensurer.EnsureKubeletCloudProviderConfig(ctx, dummyContext, emptydata, namespace)
			Expect(err).To(Not(HaveOccurred()))
			Expect(*emptydata).To(Equal(cloudProviderConfigContent))
		})

		It("should modify existing element containing cloud provider config content", func() {
			c.EXPECT().Get(ctx, objKey, &corev1.Secret{}).DoAndReturn(clientGet(secret))

			err := ensurer.EnsureKubeletCloudProviderConfig(ctx, dummyContext, existingData, namespace)
			Expect(err).To(Not(HaveOccurred()))
			Expect(*existingData).To(Equal(cloudProviderConfigContent))
		})
	})
})

func checkKubeAPIServerDeployment(dep *appsv1.Deployment, annotations map[string]string, k8sVersion string, needsCSIMigrationCompletedFeatureGates bool) {
	k8sVersionLessThan117, _ := version.CompareVersions(k8sVersion, "<", "1.17")
	k8sVersionAtLeast119, _ := version.CompareVersions(k8sVersion, ">=", "1.19")

	// Check that the kube-apiserver container still exists and contains all needed command line args,
	// env vars, and volume mounts
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-apiserver")
	Expect(c).To(Not(BeNil()))

	if !needsCSIMigrationCompletedFeatureGates {
		Expect(c.Command).To(ContainElement("--cloud-provider=azure"))
		Expect(c.Command).To(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
		Expect(c.Command).To(test.ContainElementWithPrefixContaining("--enable-admission-plugins=", "PersistentVolumeLabel", ","))
		Expect(c.Command).To(Not(test.ContainElementWithPrefixContaining("--disable-admission-plugins=", "PersistentVolumeLabel", ",")))
		Expect(dep.Spec.Template.Annotations).To(Equal(annotations))
		Expect(c.VolumeMounts).To(ContainElement(cloudProviderConfigVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(cloudProviderConfigVolume))
		if !k8sVersionLessThan117 {
			Expect(c.VolumeMounts).To(ContainElement(etcSSLVolumeMount))
			Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(etcSSLVolume))
		}
		if k8sVersionAtLeast119 {
			Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationAzureDisk=true,CSIMigrationAzureFile=true"))
		}
	} else {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationAzureDisk=true,CSIMigrationAzureFile=true,CSIMigrationAzureDiskComplete=true,CSIMigrationAzureFileComplete=true"))
		Expect(c.Command).NotTo(ContainElement("--cloud-provider=azure"))
		Expect(c.Command).NotTo(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
		Expect(c.Command).NotTo(test.ContainElementWithPrefixContaining("--enable-admission-plugins=", "PersistentVolumeLabel", ","))
		Expect(c.Command).To(test.ContainElementWithPrefixContaining("--disable-admission-plugins=", "PersistentVolumeLabel", ","))
		Expect(c.VolumeMounts).NotTo(ContainElement(cloudProviderConfigVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(cloudProviderConfigVolume))
		Expect(c.VolumeMounts).NotTo(ContainElement(etcSSLVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(etcSSLVolume))
		Expect(dep.Spec.Template.Annotations).To(BeNil())
	}
}

func checkKubeControllerManagerDeployment(dep *appsv1.Deployment, annotations, labels map[string]string, k8sVersion string, needsCSIMigrationCompletedFeatureGates bool) {
	k8sVersionLessThan117, _ := version.CompareVersions(k8sVersion, "<", "1.17")
	k8sVersionAtLeast119, _ := version.CompareVersions(k8sVersion, ">=", "1.19")

	// Check that the kube-controller-manager container still exists and contains all needed command line args,
	// env vars, and volume mounts
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-controller-manager")
	Expect(c).To(Not(BeNil()))

	Expect(c.Command).To(ContainElement("--cloud-provider=external"))

	if !needsCSIMigrationCompletedFeatureGates {
		Expect(c.Command).To(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
		Expect(c.Command).To(ContainElement("--external-cloud-volume-plugin=azure"))
		Expect(c.VolumeMounts).To(ContainElement(cloudProviderConfigVolumeMount))
		Expect(dep.Spec.Template.Annotations).To(Equal(annotations))
		Expect(dep.Spec.Template.Labels).To(Equal(labels))
		Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(cloudProviderConfigVolume))
		if !k8sVersionLessThan117 {
			Expect(c.VolumeMounts).To(ContainElement(etcSSLVolumeMount))
			Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(etcSSLVolume))
		}
		if k8sVersionAtLeast119 {
			Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationAzureDisk=true,CSIMigrationAzureFile=true"))
		}
	} else {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationAzureDisk=true,CSIMigrationAzureFile=true,CSIMigrationAzureDiskComplete=true,CSIMigrationAzureFileComplete=true"))
		Expect(c.Command).NotTo(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
		Expect(c.Command).NotTo(ContainElement("--external-cloud-volume-plugin=aws"))
		Expect(dep.Spec.Template.Labels).To(BeNil())
		Expect(dep.Spec.Template.Spec.Volumes).To(BeNil())
		Expect(c.VolumeMounts).NotTo(ContainElement(cloudProviderConfigVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(cloudProviderConfigVolume))
		Expect(c.VolumeMounts).NotTo(ContainElement(etcSSLVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(etcSSLVolume))
	}
}

func checkKubeSchedulerDeployment(dep *appsv1.Deployment, k8sVersion string, needsCSIMigrationCompletedFeatureGates bool) {
	if k8sVersionAtLeast119, _ := version.CompareVersions(k8sVersion, ">=", "1.19"); !k8sVersionAtLeast119 {
		return
	}

	// Check that the kube-scheduler container still exists and contains all needed command line args.
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-scheduler")
	Expect(c).To(Not(BeNil()))

	if !needsCSIMigrationCompletedFeatureGates {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationAzureDisk=true,CSIMigrationAzureFile=true"))
	} else {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationAzureDisk=true,CSIMigrationAzureFile=true,CSIMigrationAzureDiskComplete=true,CSIMigrationAzureFileComplete=true"))
	}
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
