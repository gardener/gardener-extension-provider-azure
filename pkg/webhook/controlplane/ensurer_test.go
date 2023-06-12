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

	"github.com/Masterminds/semver"
	"github.com/coreos/go-systemd/v22/unit"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/csimigration"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/test"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/utils/version"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
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

		dummyContext   = gcontext.NewGardenContext(nil, nil)
		eContextK8s120 = gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.20.0",
						},
					},
					Status: gardencorev1beta1.ShootStatus{
						TechnicalID: namespace,
					},
				},
			},
		)
		eContextK8s121 = gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.21.0",
						},
					},
					Status: gardencorev1beta1.ShootStatus{
						TechnicalID: namespace,
					},
				},
			},
		)
		eContextK8s121WithCSIAnnotation = gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						csimigration.AnnotationKeyNeedsComplete: "true",
					},
				},
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.21.0",
						},
					},
					Status: gardencorev1beta1.ShootStatus{
						TechnicalID: namespace,
					},
				},
			},
		)
		eContextK8s127 = gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.27.1",
						},
					},
					Status: gardencorev1beta1.ShootStatus{
						TechnicalID: namespace,
					},
				},
			},
		)
		key    = client.ObjectKey{Namespace: namespace, Name: azure.CloudProviderConfigName}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: azure.CloudProviderConfigName},
			Data:       map[string][]byte{"abc": []byte("xyz"), azure.CloudProviderConfigMapKey: []byte(cloudProviderConfigContent)},
		}

		annotations = map[string]string{
			"checksum/secret-" + azure.CloudProviderConfigName: "546bca950d25ff0b53fe8b7d7e2cee183f61524d4e3207f9e4db953ee06bc48d",
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

		It("should add missing elements to kube-apiserver deployment (k8s < 1.21)", func() {
			c.EXPECT().Get(ctx, key, &corev1.Secret{}).DoAndReturn(clientGet(secret))

			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s120, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, annotations, "1.20.4", false)
		})

		It("should add missing elements to kube-apiserver deployment (k8s >= 1.21)", func() {
			c.EXPECT().Get(ctx, key, &corev1.Secret{}).DoAndReturn(clientGet(secret))

			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s121, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, annotations, "1.21.0", false)
		})

		It("should add missing elements to kube-apiserver deployment (k8s >= 1.21 w/ CSI annotation)", func() {
			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s121WithCSIAnnotation, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, nil, "1.21.0", true)
		})

		It("should add missing elements to kube-apiserver deployment (k8s >= 1.27)", func() {
			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s127, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, annotations, "1.27.1", false)
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

			Expect(ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s120, dep, nil)).To(Not(HaveOccurred()))
			checkKubeAPIServerDeployment(dep, annotations, "1.20.0", false)
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

		It("should add missing elements to kube-controller-manager deployment (k8s < 1.21)", func() {
			c.EXPECT().Get(ctx, key, &corev1.Secret{}).DoAndReturn(clientGet(secret))

			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s120, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, annotations, kubeControllerManagerLabels, "1.20.8", false)
		})

		It("should add missing elements to kube-controller-manager deployment (k8s >= 1.21 w/o CSI annotation)", func() {
			c.EXPECT().Get(ctx, key, &corev1.Secret{}).DoAndReturn(clientGet(secret))

			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s121, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, annotations, kubeControllerManagerLabels, "1.21.0", false)
		})

		It("should add missing elements to kube-controller-manager deployment (k8s >= 1.21 w/ CSI annotation)", func() {
			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s121WithCSIAnnotation, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, nil, nil, "1.21.0", true)
		})

		It("should add missing elements to kube-controller-manager deployment (k8s >= 1.27)", func() {
			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s127, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, annotations, kubeControllerManagerLabels, "1.27.0", false)
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

			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s120, dep, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkKubeControllerManagerDeployment(dep, annotations, kubeControllerManagerLabels, "1.20.0", false)
		})
	})

	Describe("#EnsureKubeSchedulerDeployment", func() {
		var dep *appsv1.Deployment

		BeforeEach(func() {
			dep = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.DeploymentNameKubeScheduler},
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

		It("should not add anything to kube-scheduler deployment (k8s < 1.21)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s120, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.20.0", false)
		})

		It("should add missing elements to kube-scheduler deployment (k8s >= 1.21 w/o CSI annotation)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s121, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.21.0", false)
		})

		It("should add missing elements to kube-scheduler deployment (k8s >= 1.21 w/ CSI annotation)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s121WithCSIAnnotation, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.21.0", true)
		})

		It("should add missing elements to kube-scheduler deployment (k8s >= 1.27)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s127, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.27.0", false)
		})
	})

	Describe("#EnsureClusterAutoscalerDeployment", func() {
		var (
			dep     *appsv1.Deployment
			ensurer genericmutator.Ensurer
		)

		BeforeEach(func() {
			dep = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.DeploymentNameClusterAutoscaler},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "cluster-autoscaler",
								},
							},
						},
					},
				},
			}

			ensurer = NewEnsurer(logger)
		})

		It("should not add anything to cluster-autoscaler deployment (k8s < 1.21)", func() {
			err := ensurer.EnsureClusterAutoscalerDeployment(ctx, eContextK8s121, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkClusterAutoscalerDeployment(dep, "1.21.0", false)
		})

		It("should add missing elements to cluster-autoscaler deployment (k8s >= 1.21 w/o CSI annotation)", func() {
			err := ensurer.EnsureClusterAutoscalerDeployment(ctx, eContextK8s121, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkClusterAutoscalerDeployment(dep, "1.21.0", false)
		})

		It("should add missing elements to cluster-autoscaler deployment (k8s >= 1.21 w/ CSI annotation)", func() {
			err := ensurer.EnsureClusterAutoscalerDeployment(ctx, eContextK8s121WithCSIAnnotation, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkClusterAutoscalerDeployment(dep, "1.21.0", true)
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

		DescribeTable("should modify existing elements of kubelet.service unit options",
			func(gctx gcontext.GardenContext, kubeletVersion *semver.Version, cloudProvider string, withACRConfig bool, withControllerAttachDetachFlag bool) {
				newUnitOptions := []*unit.UnitOption{
					{
						Section: "Service",
						Name:    "ExecStart",
						Value: `/opt/bin/hyperkube kubelet \
    --config=/var/lib/kubelet/config/kubelet`,
					},
				}

				if cloudProvider == "azure" {
					newUnitOptions[0].Value += ` \
    --cloud-provider=azure \
    --cloud-config=/var/lib/kubelet/cloudprovider.conf`
				} else if cloudProvider == "external" {
					newUnitOptions[0].Value += ` \
    --cloud-provider=external`
				}

				if withControllerAttachDetachFlag {
					newUnitOptions[0].Value += ` \
    --enable-controller-attach-detach=true`
				}

				if withACRConfig {
					acrCM := &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: azure.CloudProviderAcrConfigName},
						Data:       map[string]string{},
					}
					c.EXPECT().Get(ctx, acrCmKey, &corev1.ConfigMap{}).DoAndReturn(clientGet(acrCM))

					newUnitOptions[0].Value += ` \
    --azure-container-registry-config=/var/lib/kubelet/acr.conf`
				} else {
					c.EXPECT().Get(ctx, acrCmKey, &corev1.ConfigMap{}).Return(apierrors.NewNotFound(schema.GroupResource{}, azure.CloudProviderAcrConfigName))
				}

				opts, err := ensurer.EnsureKubeletServiceUnitOptions(ctx, gctx, kubeletVersion, oldUnitOptions, nil)
				Expect(err).To(Not(HaveOccurred()))
				Expect(opts).To(Equal(newUnitOptions))
			},

			Entry("kubelet < 1.21, w/o acr", eContextK8s120, semver.MustParse("1.20.0"), "azure", false, false),
			Entry("kubelet < 1.21, w/ acr", eContextK8s120, semver.MustParse("1.20.0"), "azure", true, false),
			Entry("1.21 <= kubelet < 1.23, w/o acr", eContextK8s121, semver.MustParse("1.21.0"), "external", false, true),
			Entry("1.21 <= kubelet < 1.23, w/ acr", eContextK8s121, semver.MustParse("1.21.0"), "external", true, true),
			Entry("kubelet >= 1.23, w/o acr", eContextK8s121, semver.MustParse("1.23.0"), "external", false, false),
			Entry("kubelet >= 1.23, w/ acr", eContextK8s121, semver.MustParse("1.23.0"), "external", true, false),
		)
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

		DescribeTable("should modify existing elements of kubelet configuration",
			func(gctx gcontext.GardenContext, kubeletVersion *semver.Version, withCSIFeatureGates bool, enableControllerAttachDetach *bool) {
				newKubeletConfig := &kubeletconfigv1beta1.KubeletConfiguration{
					FeatureGates: map[string]bool{
						"Foo": true,
					},
					EnableControllerAttachDetach: enableControllerAttachDetach,
				}
				kubeletConfig := *oldKubeletConfig

				if withCSIFeatureGates {
					newKubeletConfig.FeatureGates["CSIMigration"] = true
					newKubeletConfig.FeatureGates["CSIMigrationAzureDisk"] = true
					newKubeletConfig.FeatureGates["CSIMigrationAzureFile"] = true
					newKubeletConfig.FeatureGates["InTreePluginAzureDiskUnregister"] = true
					newKubeletConfig.FeatureGates["InTreePluginAzureFileUnregister"] = true
				}

				err := ensurer.EnsureKubeletConfiguration(ctx, gctx, kubeletVersion, &kubeletConfig, nil)
				Expect(err).To(Not(HaveOccurred()))
				Expect(&kubeletConfig).To(Equal(newKubeletConfig))
			},

			Entry("kubelet < 1.21", eContextK8s120, semver.MustParse("1.20.0"), false, nil),
			Entry("1.21 <= kubelet < 1.23", eContextK8s121, semver.MustParse("1.21.0"), true, nil),
			Entry("kubelet >= 1.23", eContextK8s121, semver.MustParse("1.23.0"), true, pointer.Bool(true)),
		)
	})

	Describe("#ShouldProvisionKubeletCloudProviderConfig", func() {
		It("should return true (k8s < 1.21)", func() {
			Expect(ensurer.ShouldProvisionKubeletCloudProviderConfig(ctx, eContextK8s120, semver.MustParse("1.20.0"))).To(BeTrue())
		})

		It("should return false (k8s >= 1.21)", func() {
			Expect(ensurer.ShouldProvisionKubeletCloudProviderConfig(ctx, eContextK8s121, semver.MustParse("1.21.0"))).To(BeFalse())
		})
	})

	Describe("#EnsureKubeletCloudProviderConfig", func() {
		var (
			objKey = client.ObjectKey{Namespace: namespace, Name: azure.CloudProviderDiskConfigName}
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: azure.CloudProviderDiskConfigName},
				Data:       map[string][]byte{"abc": []byte("xyz"), azure.CloudProviderConfigMapKey: []byte(cloudProviderConfigContent)},
			}

			existingData = pointer.String("[LoadBalancer]\nlb-version=v2\nlb-provider:\n")
			emptydata    = pointer.String("")
		)

		It("cloud provider secret does not exist", func() {
			c.EXPECT().Get(ctx, objKey, &corev1.Secret{}).Return(apierrors.NewNotFound(schema.GroupResource{}, secret.Name))

			err := ensurer.EnsureKubeletCloudProviderConfig(ctx, dummyContext, nil, emptydata, namespace)
			Expect(err).To(Not(HaveOccurred()))
			Expect(*emptydata).To(Equal(""))
		})

		It("should create element containing cloud provider config content with secret", func() {
			c.EXPECT().Get(ctx, objKey, &corev1.Secret{}).DoAndReturn(clientGet(secret))

			err := ensurer.EnsureKubeletCloudProviderConfig(ctx, dummyContext, nil, emptydata, namespace)
			Expect(err).To(Not(HaveOccurred()))
			Expect(*emptydata).To(Equal(cloudProviderConfigContent))
		})

		It("should modify existing element containing cloud provider config content", func() {
			c.EXPECT().Get(ctx, objKey, &corev1.Secret{}).DoAndReturn(clientGet(secret))

			err := ensurer.EnsureKubeletCloudProviderConfig(ctx, dummyContext, nil, existingData, namespace)
			Expect(err).To(Not(HaveOccurred()))
			Expect(*existingData).To(Equal(cloudProviderConfigContent))
		})
	})
})

func checkKubeAPIServerDeployment(dep *appsv1.Deployment, annotations map[string]string, k8sVersion string, needsCSIMigrationCompletedFeatureGates bool) {
	k8sVersionAtLeast121, _ := version.CompareVersions(k8sVersion, ">=", "1.21")
	k8sVersionAtLeast127, _ := version.CompareVersions(k8sVersion, ">=", "1.27")

	// Check that the kube-apiserver container still exists and contains all needed command line args,
	// env vars, and volume mounts
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-apiserver")
	Expect(c).To(Not(BeNil()))

	if k8sVersionAtLeast127 {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigrationAzureFile=true,InTreePluginAzureDiskUnregister=true,InTreePluginAzureFileUnregister=true"))
		Expect(c.Command).NotTo(ContainElement("--cloud-provider=azure"))
		Expect(c.Command).NotTo(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
		Expect(c.Command).NotTo(test.ContainElementWithPrefixContaining("--enable-admission-plugins=", "PersistentVolumeLabel", ","))
		Expect(c.Command).To(test.ContainElementWithPrefixContaining("--disable-admission-plugins=", "PersistentVolumeLabel", ","))
		Expect(c.VolumeMounts).NotTo(ContainElement(cloudProviderConfigVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(cloudProviderConfigVolume))
		Expect(dep.Spec.Template.Annotations).To(BeNil())
		Expect(dep.Spec.Template.Labels).To(HaveKeyWithValue("networking.resources.gardener.cloud/to-csi-snapshot-validation-tcp-443", "allowed"))
	} else if !needsCSIMigrationCompletedFeatureGates {
		Expect(c.Command).To(ContainElement("--cloud-provider=azure"))
		Expect(c.Command).To(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
		Expect(c.Command).To(test.ContainElementWithPrefixContaining("--enable-admission-plugins=", "PersistentVolumeLabel", ","))
		Expect(c.Command).To(Not(test.ContainElementWithPrefixContaining("--disable-admission-plugins=", "PersistentVolumeLabel", ",")))
		Expect(dep.Spec.Template.Annotations).To(Equal(annotations))
		Expect(c.VolumeMounts).To(ContainElement(cloudProviderConfigVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(cloudProviderConfigVolume))
		Expect(dep.Spec.Template.Labels).To(HaveKeyWithValue("networking.resources.gardener.cloud/to-csi-snapshot-validation-tcp-443", "allowed"))
		if k8sVersionAtLeast121 {
			Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationAzureDisk=true,CSIMigrationAzureFile=true"))
		}
	} else {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationAzureDisk=true,CSIMigrationAzureFile=true,InTreePluginAzureDiskUnregister=true,InTreePluginAzureFileUnregister=true"))
		Expect(c.Command).NotTo(ContainElement("--cloud-provider=azure"))
		Expect(c.Command).NotTo(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
		Expect(c.Command).NotTo(test.ContainElementWithPrefixContaining("--enable-admission-plugins=", "PersistentVolumeLabel", ","))
		Expect(c.Command).To(test.ContainElementWithPrefixContaining("--disable-admission-plugins=", "PersistentVolumeLabel", ","))
		Expect(c.VolumeMounts).NotTo(ContainElement(cloudProviderConfigVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(cloudProviderConfigVolume))
		Expect(dep.Spec.Template.Annotations).To(BeNil())
		Expect(dep.Spec.Template.Labels).To(HaveKeyWithValue("networking.resources.gardener.cloud/to-csi-snapshot-validation-tcp-443", "allowed"))
	}
}

func checkKubeControllerManagerDeployment(dep *appsv1.Deployment, annotations, labels map[string]string, k8sVersion string, needsCSIMigrationCompletedFeatureGates bool) {
	k8sVersionAtLeast121, _ := version.CompareVersions(k8sVersion, ">=", "1.21")
	k8sVersionAtLeast127, _ := version.CompareVersions(k8sVersion, ">=", "1.27")

	// Check that the kube-controller-manager container still exists and contains all needed command line args,
	// env vars, and volume mounts
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-controller-manager")
	Expect(c).To(Not(BeNil()))

	Expect(c.Command).To(ContainElement("--cloud-provider=external"))

	if k8sVersionAtLeast127 {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigrationAzureFile=true,InTreePluginAzureDiskUnregister=true,InTreePluginAzureFileUnregister=true"))
		Expect(c.Command).NotTo(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
		Expect(c.Command).NotTo(ContainElement("--external-cloud-volume-plugin=aws"))
		Expect(dep.Spec.Template.Labels).To(BeNil())
		Expect(dep.Spec.Template.Spec.Volumes).To(BeNil())
		Expect(c.VolumeMounts).NotTo(ContainElement(cloudProviderConfigVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(cloudProviderConfigVolume))
		Expect(c.VolumeMounts).NotTo(ContainElement(etcSSLVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(etcSSLVolume))
		Expect(c.VolumeMounts).NotTo(ContainElement(usrShareCaCertsVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(usrShareCaCertsVolume))
	} else if !needsCSIMigrationCompletedFeatureGates {
		Expect(c.Command).To(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
		Expect(c.Command).To(ContainElement("--external-cloud-volume-plugin=azure"))
		Expect(c.VolumeMounts).To(ContainElement(cloudProviderConfigVolumeMount))
		Expect(dep.Spec.Template.Annotations).To(Equal(annotations))
		Expect(dep.Spec.Template.Labels).To(Equal(labels))
		Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(cloudProviderConfigVolume))
		Expect(c.VolumeMounts).To(ContainElement(etcSSLVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(etcSSLVolume))
		Expect(c.VolumeMounts).To(ContainElement(usrShareCaCertsVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(usrShareCaCertsVolume))
		if k8sVersionAtLeast121 {
			Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationAzureDisk=true,CSIMigrationAzureFile=true"))
		}
	} else {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationAzureDisk=true,CSIMigrationAzureFile=true,InTreePluginAzureDiskUnregister=true,InTreePluginAzureFileUnregister=true"))
		Expect(c.Command).NotTo(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
		Expect(c.Command).NotTo(ContainElement("--external-cloud-volume-plugin=aws"))
		Expect(dep.Spec.Template.Labels).To(BeNil())
		Expect(dep.Spec.Template.Spec.Volumes).To(BeNil())
		Expect(c.VolumeMounts).NotTo(ContainElement(cloudProviderConfigVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(cloudProviderConfigVolume))
		Expect(c.VolumeMounts).NotTo(ContainElement(etcSSLVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(etcSSLVolume))
		Expect(c.VolumeMounts).NotTo(ContainElement(usrShareCaCertsVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(usrShareCaCertsVolume))
	}
}

func checkKubeSchedulerDeployment(dep *appsv1.Deployment, k8sVersion string, needsCSIMigrationCompletedFeatureGates bool) {
	if k8sVersionAtLeast121, _ := version.CompareVersions(k8sVersion, ">=", "1.21"); !k8sVersionAtLeast121 {
		return
	}
	k8sVersionAtLeast127, _ := version.CompareVersions(k8sVersion, ">=", "1.27")

	// Check that the kube-scheduler container still exists and contains all needed command line args.
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-scheduler")
	Expect(c).To(Not(BeNil()))

	if k8sVersionAtLeast127 {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigrationAzureFile=true,InTreePluginAzureDiskUnregister=true,InTreePluginAzureFileUnregister=true"))
	} else if !needsCSIMigrationCompletedFeatureGates {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationAzureDisk=true,CSIMigrationAzureFile=true"))
	} else {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationAzureDisk=true,CSIMigrationAzureFile=true,InTreePluginAzureDiskUnregister=true,InTreePluginAzureFileUnregister=true"))
	}
}

func checkClusterAutoscalerDeployment(dep *appsv1.Deployment, k8sVersion string, needsCSIMigrationCompletedFeatureGates bool) {
	if k8sVersionAtLeast121, _ := version.CompareVersions(k8sVersion, ">=", "1.21"); !k8sVersionAtLeast121 {
		return
	}
	k8sVersionAtLeast127, _ := version.CompareVersions(k8sVersion, ">=", "1.27")

	// Check that the cluster-autoscaler container still exists and contains all needed command line args.
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "cluster-autoscaler")
	Expect(c).To(Not(BeNil()))

	if k8sVersionAtLeast127 {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigrationAzureFile=true,InTreePluginAzureDiskUnregister=true,InTreePluginAzureFileUnregister=true"))
	} else if !needsCSIMigrationCompletedFeatureGates {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationAzureDisk=true,CSIMigrationAzureFile=true"))
	} else {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationAzureDisk=true,CSIMigrationAzureFile=true,InTreePluginAzureDiskUnregister=true,InTreePluginAzureFileUnregister=true"))
	}
}

func clientGet(result runtime.Object) interface{} {
	return func(ctx context.Context, key client.ObjectKey, obj runtime.Object, _ ...client.GetOption) error {
		switch obj.(type) {
		case *corev1.Secret:
			*obj.(*corev1.Secret) = *result.(*corev1.Secret)
		case *corev1.ConfigMap:
			*obj.(*corev1.ConfigMap) = *result.(*corev1.ConfigMap)
		}
		return nil
	}
}
