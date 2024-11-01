// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/coreos/go-systemd/v22/unit"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/test"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/nodemanagement/machinecontrollermanager"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	testutils "github.com/gardener/gardener/pkg/utils/test"
	"github.com/gardener/gardener/pkg/utils/version"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
		mgr     *mockmanager.MockManager

		dummyContext   = gcontext.NewGardenContext(nil, nil)
		eContextK8s126 = gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.26.0",
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
		eContextK8s130 = gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.30.1",
						},
					},
					Status: gardencorev1beta1.ShootStatus{
						TechnicalID: namespace,
					},
				},
			},
		)
		eContextK8s131 = gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.31.1",
						},
					},
					Status: gardencorev1beta1.ShootStatus{
						TechnicalID: namespace,
					},
				},
			},
		)
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)

		mgr = mockmanager.NewMockManager(ctrl)
		mgr.EXPECT().GetClient().Return(c)

		ensurer = NewEnsurer(mgr, logger)
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

		It("should add missing elements to kube-apiserver deployment (k8s < 1.27)", func() {
			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s126, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, "1.26.0")
		})

		It("should add missing elements to kube-apiserver deployment (k8s >= 1.27, < 1.30)", func() {
			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s127, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, "1.27.1")
		})

		It("should add missing elements to kube-apiserver deployment (k8s = 1.30)", func() {
			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s130, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, "1.30.1")
		})

		It("should add missing elements to kube-apiserver deployment (k8s >= 1.31)", func() {
			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s131, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, "1.31.1")
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

			Expect(ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s126, dep, nil)).To(Not(HaveOccurred()))
			checkKubeAPIServerDeployment(dep, "1.26.0")
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

		It("should add missing elements to kube-controller-manager deployment (k8s < 1.27)", func() {
			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s126, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, "1.26.0")
		})

		It("should add missing elements to kube-controller-manager deployment (k8s >= 1.27, < 1.30)", func() {
			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s127, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, "1.27.1")
		})

		It("should add missing elements to kube-controller-manager deployment (k8s = 1.30)", func() {
			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s130, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, "1.30.1")
		})

		It("should add missing elements to kube-controller-manager deployment (k8s >= 1.30)", func() {
			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s131, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, "1.31.1")
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
								},
							},
						},
					},
				},
			}

			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s126, dep, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkKubeControllerManagerDeployment(dep, "1.26.0")
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
		})

		It("should add missing elements to kube-scheduler deployment (k8s < 1.27)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s126, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.26.0")
		})

		It("should add missing elements to kube-scheduler deployment (k8s >= 1.27, < 1.30)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s127, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.27.1")
		})

		It("should add missing elements to kube-scheduler deployment (k8s = 1.30)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s130, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.30.1")
		})

		It("should add missing elements to kube-scheduler deployment (k8s >= 1.31)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s131, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.31.1")
		})
	})

	Describe("#EnsureClusterAutoscalerDeployment", func() {
		var dep *appsv1.Deployment

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
		})

		It("should add missing elements to cluster-autoscaler deployment (k8s < 1.27)", func() {
			err := ensurer.EnsureClusterAutoscalerDeployment(ctx, eContextK8s126, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkClusterAutoscalerDeployment(dep, "1.26.0")
		})

		It("should add missing elements to cluster-autoscaler deployment (k8s >= 1.27, < 1.30)", func() {
			err := ensurer.EnsureClusterAutoscalerDeployment(ctx, eContextK8s127, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkClusterAutoscalerDeployment(dep, "1.27.1")
		})

		It("should add missing elements to cluster-autoscaler deployment (k8s = 1.30)", func() {
			err := ensurer.EnsureClusterAutoscalerDeployment(ctx, eContextK8s130, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkClusterAutoscalerDeployment(dep, "1.30.1")
		})

		It("should add missing elements to cluster-autoscaler deployment (k8s >= 1.31)", func() {
			err := ensurer.EnsureClusterAutoscalerDeployment(ctx, eContextK8s131, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkClusterAutoscalerDeployment(dep, "1.31.0")
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
			func(gctx gcontext.GardenContext, cloudProvider string, withACRConfig bool, _ bool) {
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

				opts, err := ensurer.EnsureKubeletServiceUnitOptions(ctx, gctx, nil, oldUnitOptions, nil)
				Expect(err).To(Not(HaveOccurred()))
				Expect(opts).To(Equal(newUnitOptions))
			},

			Entry("kubelet >= 1.27, w/ acr", eContextK8s127, "external", true, false),
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
			func(gctx gcontext.GardenContext, kubeletVersion *semver.Version, expectedFeatureGates map[string]bool) {
				newKubeletConfig := &kubeletconfigv1beta1.KubeletConfiguration{
					FeatureGates: map[string]bool{
						"Foo": true,
					},
					EnableControllerAttachDetach: ptr.To(true),
				}
				kubeletConfig := *oldKubeletConfig

				for featureGate, value := range expectedFeatureGates {
					newKubeletConfig.FeatureGates[featureGate] = value
				}

				err := ensurer.EnsureKubeletConfiguration(ctx, gctx, kubeletVersion, &kubeletConfig, nil)
				Expect(err).To(Not(HaveOccurred()))
				Expect(&kubeletConfig).To(Equal(newKubeletConfig))
			},

			Entry("kubelet k8s < 1.27",
				eContextK8s126,
				semver.MustParse("1.26.0"),
				map[string]bool{
					"CSIMigration":                    true,
					"CSIMigrationAzureDisk":           true,
					"CSIMigrationAzureFile":           true,
					"InTreePluginAzureDiskUnregister": true,
					"InTreePluginAzureFileUnregister": true,
				},
			),
			Entry("kubelet k8s >=1.27, < 1.30",
				eContextK8s127,
				semver.MustParse("1.27.0"),
				map[string]bool{
					"CSIMigrationAzureFile":           true,
					"InTreePluginAzureDiskUnregister": true,
					"InTreePluginAzureFileUnregister": true,
				},
			),
			Entry("kubelet k8s = 1.30",
				eContextK8s130,
				semver.MustParse("1.30.0"),
				map[string]bool{
					"InTreePluginAzureDiskUnregister": true,
					"InTreePluginAzureFileUnregister": true,
				},
			),
			Entry("kubelet k8s >= 1.31",
				eContextK8s131,
				semver.MustParse("1.31.0"),
				map[string]bool{},
			),
		)
	})

	Describe("#ShouldProvisionKubeletCloudProviderConfig", func() {
		It("should return false ", func() {
			Expect(ensurer.ShouldProvisionKubeletCloudProviderConfig(ctx, eContextK8s127, semver.MustParse("1.27.0"))).To(BeFalse())
		})
	})

	Describe("#EnsureKubeletCloudProviderConfig", func() {
		var (
			objKey = client.ObjectKey{Namespace: namespace, Name: azure.CloudProviderDiskConfigName}
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: azure.CloudProviderDiskConfigName},
				Data:       map[string][]byte{"abc": []byte("xyz"), azure.CloudProviderConfigMapKey: []byte(cloudProviderConfigContent)},
			}

			existingData = ptr.To("[LoadBalancer]\nlb-version=v2\nlb-provider:\n")
			emptydata    = ptr.To("")
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

	Describe("#EnsureMachineControllerManagerDeployment", func() {
		var (
			ensurer    genericmutator.Ensurer
			deployment *appsv1.Deployment
		)

		BeforeEach(func() {
			deployment = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: "foo"}}
		})

		BeforeEach(func() {
			mgr.EXPECT().GetClient().Return(c)
			ensurer = NewEnsurer(mgr, logger)
			DeferCleanup(testutils.WithVar(&ImageVector, imagevector.ImageVector{{
				Name:       "machine-controller-manager-provider-azure",
				Repository: ptr.To("foo"),
				Tag:        ptr.To("bar"),
			}}))
		})

		It("should inject the sidecar container", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cloudprovider",
					Namespace: deployment.Namespace,
				},
			}
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(secret), secret).DoAndReturn(clientGet(secret))
			Expect(deployment.Spec.Template.Spec.Containers).To(BeEmpty())
			Expect(ensurer.EnsureMachineControllerManagerDeployment(context.TODO(), nil, deployment, nil)).To(BeNil())
			expectedContainer := machinecontrollermanager.ProviderSidecarContainer(deployment.Namespace, "provider-azure", "foo:bar")
			expectedContainer.Args = append(expectedContainer.Args, "--machine-pv-reattach-timeout=150s")
			Expect(deployment.Spec.Template.Spec.Containers).To(ConsistOf(expectedContainer))
		})

		It("should inject the sidecar container and mount the workload identity token volume", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cloudprovider",
					Namespace: deployment.Namespace,
				},
			}
			returnedSecret := secret.DeepCopy()
			returnedSecret.Labels = map[string]string{
				"security.gardener.cloud/purpose": "workload-identity-token-requestor",
			}
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(secret), secret).DoAndReturn(clientGet(returnedSecret))
			Expect(deployment.Spec.Template.Spec.Containers).To(BeEmpty())
			Expect(ensurer.EnsureMachineControllerManagerDeployment(context.TODO(), nil, deployment, nil)).To(BeNil())
			expectedContainer := machinecontrollermanager.ProviderSidecarContainer(deployment.Namespace, "provider-azure", "foo:bar")
			expectedContainer.VolumeMounts = append(expectedContainer.VolumeMounts, corev1.VolumeMount{
				Name:      "workload-identity",
				MountPath: "/var/run/secrets/gardener.cloud/workload-identity",
			})

			expectedContainer.Args = append(expectedContainer.Args, "--machine-pv-reattach-timeout=150s")
			Expect(deployment.Spec.Template.Spec.Containers).To(ConsistOf(expectedContainer))
			Expect(deployment.Spec.Template.Spec.Volumes).To(ConsistOf(corev1.Volume{
				Name: "workload-identity",
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{
							{
								Secret: &corev1.SecretProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: v1beta1constants.SecretNameCloudProvider,
									},
									Items: []corev1.KeyToPath{
										{
											Key:  "token",
											Path: "token",
										},
									},
								},
							},
						},
					},
				},
			}))
		})
	})

	Describe("#EnsureMachineControllerManagerVPA", func() {
		var (
			ensurer genericmutator.Ensurer
			vpa     *vpaautoscalingv1.VerticalPodAutoscaler
		)

		BeforeEach(func() {
			vpa = &vpaautoscalingv1.VerticalPodAutoscaler{}
		})

		BeforeEach(func() {
			mgr.EXPECT().GetClient().Return(c)
			ensurer = NewEnsurer(mgr, logger)
		})

		It("should inject the sidecar container policy", func() {
			Expect(vpa.Spec.ResourcePolicy).To(BeNil())
			Expect(ensurer.EnsureMachineControllerManagerVPA(context.TODO(), nil, vpa, nil)).To(BeNil())

			ccv := vpaautoscalingv1.ContainerControlledValuesRequestsOnly
			Expect(vpa.Spec.ResourcePolicy.ContainerPolicies).To(ConsistOf(vpaautoscalingv1.ContainerResourcePolicy{
				ContainerName:    "machine-controller-manager-provider-azure",
				ControlledValues: &ccv,
			}))
		})
	})
})

func checkKubeAPIServerDeployment(dep *appsv1.Deployment, k8sVersion string) {
	k8sVersionAtLeast127, _ := version.CompareVersions(k8sVersion, ">=", "1.27")
	k8sVersionAtLeast130, _ := version.CompareVersions(k8sVersion, ">=", "1.30")
	k8sVersionAtLeast131, _ := version.CompareVersions(k8sVersion, ">=", "1.31")

	// Check that the kube-apiserver container still exists and contains all needed command line args,
	// env vars, and volume mounts
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-apiserver")
	Expect(c).To(Not(BeNil()))

	switch {
	case k8sVersionAtLeast131:
		Expect(c.Command).NotTo(ContainElement(HavePrefix("--feature-gates")))
	case k8sVersionAtLeast130:
		Expect(c.Command).To(ContainElement("--feature-gates=InTreePluginAzureDiskUnregister=true,InTreePluginAzureFileUnregister=true"))
	case k8sVersionAtLeast127:
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigrationAzureFile=true,InTreePluginAzureDiskUnregister=true,InTreePluginAzureFileUnregister=true"))
	default: // < 1.27
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationAzureDisk=true,CSIMigrationAzureFile=true,InTreePluginAzureDiskUnregister=true,InTreePluginAzureFileUnregister=true"))
	}

	Expect(c.Command).NotTo(ContainElement("--cloud-provider=azure"))
	Expect(c.Command).NotTo(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
	if !k8sVersionAtLeast131 {
		Expect(c.Command).NotTo(test.ContainElementWithPrefixContaining("--enable-admission-plugins=", "PersistentVolumeLabel", ","))
		Expect(c.Command).To(test.ContainElementWithPrefixContaining("--disable-admission-plugins=", "PersistentVolumeLabel", ","))
	}
	Expect(c.VolumeMounts).NotTo(ContainElement(cloudProviderConfigVolumeMount))
	Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(cloudProviderConfigVolume))
	Expect(dep.Spec.Template.Annotations).To(BeNil())
	Expect(dep.Spec.Template.Labels).To(HaveKeyWithValue("networking.resources.gardener.cloud/to-csi-snapshot-validation-tcp-443", "allowed"))

}

func checkKubeControllerManagerDeployment(dep *appsv1.Deployment, k8sVersion string) {
	k8sVersionAtLeast127, _ := version.CompareVersions(k8sVersion, ">=", "1.27")
	k8sVersionAtLeast130, _ := version.CompareVersions(k8sVersion, ">=", "1.30")
	k8sVersionAtLeast131, _ := version.CompareVersions(k8sVersion, ">=", "1.31")

	// Check that the kube-controller-manager container still exists and contains all needed command line args,
	// env vars, and volume mounts
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-controller-manager")
	Expect(c).To(Not(BeNil()))

	Expect(c.Command).To(ContainElement("--cloud-provider=external"))

	switch {
	case k8sVersionAtLeast131:
		Expect(c.Command).NotTo(ContainElement(HavePrefix("--feature-gates")))
	case k8sVersionAtLeast130:
		Expect(c.Command).To(ContainElement("--feature-gates=InTreePluginAzureDiskUnregister=true,InTreePluginAzureFileUnregister=true"))
	case k8sVersionAtLeast127:
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigrationAzureFile=true,InTreePluginAzureDiskUnregister=true,InTreePluginAzureFileUnregister=true"))
	default: // < 1.27
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationAzureDisk=true,CSIMigrationAzureFile=true,InTreePluginAzureDiskUnregister=true,InTreePluginAzureFileUnregister=true"))
	}

	Expect(c.Command).NotTo(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
	Expect(c.Command).NotTo(ContainElement("--external-cloud-volume-plugin=azure"))
	Expect(dep.Spec.Template.Labels).To(BeNil())
	Expect(dep.Spec.Template.Spec.Volumes).To(BeEmpty())
	Expect(c.VolumeMounts).NotTo(ContainElement(cloudProviderConfigVolumeMount))
	Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(cloudProviderConfigVolume))
	Expect(c.VolumeMounts).NotTo(ContainElement(etcSSLVolumeMount))
	Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(etcSSLVolume))
	Expect(c.VolumeMounts).NotTo(ContainElement(usrShareCaCertsVolumeMount))
	Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(usrShareCaCertsVolume))
}

func checkKubeSchedulerDeployment(dep *appsv1.Deployment, k8sVersion string) {
	k8sVersionAtLeast127, _ := version.CompareVersions(k8sVersion, ">=", "1.27")
	k8sVersionAtLeast130, _ := version.CompareVersions(k8sVersion, ">=", "1.30")
	k8sVersionAtLeast131, _ := version.CompareVersions(k8sVersion, ">=", "1.31")

	// Check that the kube-scheduler container still exists and contains all needed command line args.
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-scheduler")
	Expect(c).To(Not(BeNil()))

	switch {
	case k8sVersionAtLeast131:
		Expect(c.Command).NotTo(ContainElement(HavePrefix("--feature-gates")))
	case k8sVersionAtLeast130:
		Expect(c.Command).To(ContainElement("--feature-gates=InTreePluginAzureDiskUnregister=true,InTreePluginAzureFileUnregister=true"))
	case k8sVersionAtLeast127:
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigrationAzureFile=true,InTreePluginAzureDiskUnregister=true,InTreePluginAzureFileUnregister=true"))
	default: // < 1.27
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationAzureDisk=true,CSIMigrationAzureFile=true,InTreePluginAzureDiskUnregister=true,InTreePluginAzureFileUnregister=true"))
	}
}

func checkClusterAutoscalerDeployment(dep *appsv1.Deployment, k8sVersion string) {
	k8sVersionAtLeast127, _ := version.CompareVersions(k8sVersion, ">=", "1.27")
	k8sVersionAtLeast130, _ := version.CompareVersions(k8sVersion, ">=", "1.30")
	k8sVersionAtLeast131, _ := version.CompareVersions(k8sVersion, ">=", "1.31")

	// Check that the cluster-autoscaler container still exists and contains all needed command line args.
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "cluster-autoscaler")
	Expect(c).To(Not(BeNil()))

	switch {
	case k8sVersionAtLeast131:
		Expect(c.Command).NotTo(ContainElement(HavePrefix("--feature-gates")))
	case k8sVersionAtLeast130:
		Expect(c.Command).To(ContainElement("--feature-gates=InTreePluginAzureDiskUnregister=true,InTreePluginAzureFileUnregister=true"))
	case k8sVersionAtLeast127:
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigrationAzureFile=true,InTreePluginAzureDiskUnregister=true,InTreePluginAzureFileUnregister=true"))
	default: // < 1.27
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationAzureDisk=true,CSIMigrationAzureFile=true,InTreePluginAzureDiskUnregister=true,InTreePluginAzureFileUnregister=true"))
	}
}

func clientGet(result runtime.Object) interface{} {
	return func(_ context.Context, _ client.ObjectKey, obj runtime.Object, _ ...client.GetOption) error {
		switch obj.(type) {
		case *corev1.Secret:
			*obj.(*corev1.Secret) = *result.(*corev1.Secret)
		case *corev1.ConfigMap:
			*obj.(*corev1.ConfigMap) = *result.(*corev1.ConfigMap)
		}
		return nil
	}
}
