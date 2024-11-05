// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/coreos/go-systemd/v22/unit"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	oscutils "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/utils"
	"github.com/gardener/gardener/pkg/component/nodemanagement/machinecontrollermanager"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/version"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-azure/imagevector"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

const (
	acrConfigPath = "/var/lib/kubelet/acr.conf"
)

var (
	// constraintK8sLess131 is a version constraint for versions < 1.31.
	//
	// TODO(ialidzhikov): Replace with versionutils.ConstraintK8sLess131 when vendoring a gardener/gardener version
	// that contains https://github.com/gardener/gardener/pull/10472.
	constraintK8sLess131 *semver.Constraints
)

func init() {
	var err error
	constraintK8sLess131, err = semver.NewConstraint("< 1.31-0")
	utilruntime.Must(err)
}

// NewEnsurer creates a new controlplane ensurer.
func NewEnsurer(mgr manager.Manager, logger logr.Logger) genericmutator.Ensurer {
	return &ensurer{
		client: mgr.GetClient(),
		logger: logger.WithName("azure-controlplane-ensurer"),
	}
}

type ensurer struct {
	genericmutator.NoopEnsurer
	client client.Client
	logger logr.Logger
}

// ImageVector is exposed for testing.
var ImageVector = imagevector.ImageVector()

// EnsureMachineControllerManagerDeployment ensures that the machine-controller-manager deployment conforms to the provider requirements.
func (e *ensurer) EnsureMachineControllerManagerDeployment(_ context.Context, _ gcontext.GardenContext, newObj, _ *appsv1.Deployment) error {
	image, err := ImageVector.FindImage(azure.MachineControllerManagerProviderAzureImageName)
	if err != nil {
		return err
	}

	sidecarContainer := machinecontrollermanager.ProviderSidecarContainer(newObj.Namespace, azure.Name, image.String())
	sidecarContainer.Args = append(sidecarContainer.Args, "--machine-pv-reattach-timeout=150s")

	newObj.Spec.Template.Spec.Containers = extensionswebhook.EnsureContainerWithName(newObj.Spec.Template.Spec.Containers, sidecarContainer)
	return nil
}

// EnsureMachineControllerManagerVPA ensures that the machine-controller-manager VPA conforms to the provider requirements.
func (e *ensurer) EnsureMachineControllerManagerVPA(_ context.Context, _ gcontext.GardenContext, newObj, _ *vpaautoscalingv1.VerticalPodAutoscaler) error {
	if newObj.Spec.ResourcePolicy == nil {
		newObj.Spec.ResourcePolicy = &vpaautoscalingv1.PodResourcePolicy{}
	}

	newObj.Spec.ResourcePolicy.ContainerPolicies = extensionswebhook.EnsureVPAContainerResourcePolicyWithName(
		newObj.Spec.ResourcePolicy.ContainerPolicies,
		machinecontrollermanager.ProviderSidecarVPAContainerPolicy(azure.Name),
	)
	return nil
}

// EnsureKubeAPIServerDeployment ensures that the kube-apiserver deployment conforms to the provider requirements.
func (e *ensurer) EnsureKubeAPIServerDeployment(ctx context.Context, gctx gcontext.GardenContext, new, _ *appsv1.Deployment) error {
	template := &new.Spec.Template
	ps := &template.Spec

	// TODO: This label approach is deprecated and no longer needed in the future. Remove it as soon as gardener/gardener@v1.75 has been released.
	metav1.SetMetaDataLabel(&new.Spec.Template.ObjectMeta, gutil.NetworkPolicyLabel(azure.CSISnapshotValidationName, 443), v1beta1constants.LabelNetworkPolicyAllowed)

	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return err
	}

	k8sVersion, err := semver.NewVersion(cluster.Shoot.Spec.Kubernetes.Version)
	if err != nil {
		return err
	}

	if c := extensionswebhook.ContainerWithName(ps.Containers, "kube-apiserver"); c != nil {
		ensureKubeAPIServerCommandLineArgs(c, k8sVersion)
	}

	return e.ensureChecksumAnnotations(&new.Spec.Template)
}

// EnsureKubeControllerManagerDeployment ensures that the kube-controller-manager deployment conforms to the provider requirements.
func (e *ensurer) EnsureKubeControllerManagerDeployment(ctx context.Context, gctx gcontext.GardenContext, new, _ *appsv1.Deployment) error {
	template := &new.Spec.Template
	ps := &template.Spec

	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return err
	}

	k8sVersion, err := semver.NewVersion(cluster.Shoot.Spec.Kubernetes.Version)
	if err != nil {
		return err
	}

	if c := extensionswebhook.ContainerWithName(ps.Containers, "kube-controller-manager"); c != nil {
		ensureKubeControllerManagerCommandLineArgs(c, k8sVersion)
		ensureKubeControllerManagerVolumeMounts(c, cluster.Shoot.Spec.Kubernetes.Version)
	}

	ensureKubeControllerManagerLabels(template)
	ensureKubeControllerManagerVolumes(ps, cluster.Shoot.Spec.Kubernetes.Version)
	return e.ensureChecksumAnnotations(&new.Spec.Template)
}

// EnsureKubeSchedulerDeployment ensures that the kube-scheduler deployment conforms to the provider requirements.
func (e *ensurer) EnsureKubeSchedulerDeployment(ctx context.Context, gctx gcontext.GardenContext, new, _ *appsv1.Deployment) error {
	template := &new.Spec.Template
	ps := &template.Spec

	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return err
	}

	k8sVersion, err := semver.NewVersion(cluster.Shoot.Spec.Kubernetes.Version)
	if err != nil {
		return err
	}

	if c := extensionswebhook.ContainerWithName(ps.Containers, "kube-scheduler"); c != nil {
		ensureKubeSchedulerCommandLineArgs(c, k8sVersion)
	}
	return nil
}

// EnsureClusterAutoscalerDeployment ensures that the cluster-autoscaler deployment conforms to the provider requirements.
func (e *ensurer) EnsureClusterAutoscalerDeployment(ctx context.Context, gctx gcontext.GardenContext, new, _ *appsv1.Deployment) error {
	template := &new.Spec.Template
	ps := &template.Spec

	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return err
	}

	k8sVersion, err := semver.NewVersion(cluster.Shoot.Spec.Kubernetes.Version)
	if err != nil {
		return err
	}

	if c := extensionswebhook.ContainerWithName(ps.Containers, "cluster-autoscaler"); c != nil {
		ensureClusterAutoscalerCommandLineArgs(c, k8sVersion)
	}
	return nil
}

func ensureKubeAPIServerCommandLineArgs(c *corev1.Container, k8sVersion *semver.Version) {
	if version.ConstraintK8sLess127.Check(k8sVersion) {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigration=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationAzureDisk=true", ",")
	}
	if version.ConstraintK8sLess130.Check(k8sVersion) {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationAzureFile=true", ",")
	}
	if constraintK8sLess131.Check(k8sVersion) {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"InTreePluginAzureDiskUnregister=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"InTreePluginAzureFileUnregister=true", ",")
	}

	c.Command = extensionswebhook.EnsureNoStringWithPrefix(c.Command, "--cloud-provider=")
	c.Command = extensionswebhook.EnsureNoStringWithPrefix(c.Command, "--cloud-config=")
	if constraintK8sLess131.Check(k8sVersion) {
		c.Command = extensionswebhook.EnsureNoStringWithPrefixContains(c.Command, "--enable-admission-plugins=",
			"PersistentVolumeLabel", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--disable-admission-plugins=",
			"PersistentVolumeLabel", ",")
	}
}

func ensureKubeControllerManagerCommandLineArgs(c *corev1.Container, k8sVersion *semver.Version) {
	c.Command = extensionswebhook.EnsureStringWithPrefix(c.Command, "--cloud-provider=", "external")

	if version.ConstraintK8sLess127.Check(k8sVersion) {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigration=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationAzureDisk=true", ",")
	}
	if version.ConstraintK8sLess130.Check(k8sVersion) {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationAzureFile=true", ",")
	}
	if constraintK8sLess131.Check(k8sVersion) {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"InTreePluginAzureDiskUnregister=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"InTreePluginAzureFileUnregister=true", ",")
	}

	c.Command = extensionswebhook.EnsureNoStringWithPrefix(c.Command, "--cloud-config=")
	c.Command = extensionswebhook.EnsureNoStringWithPrefix(c.Command, "--external-cloud-volume-plugin=")
}

func ensureKubeSchedulerCommandLineArgs(c *corev1.Container, k8sVersion *semver.Version) {
	if version.ConstraintK8sLess127.Check(k8sVersion) {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigration=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationAzureDisk=true", ",")
	}
	if version.ConstraintK8sLess130.Check(k8sVersion) {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationAzureFile=true", ",")
	}
	if constraintK8sLess131.Check(k8sVersion) {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"InTreePluginAzureDiskUnregister=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"InTreePluginAzureFileUnregister=true", ",")
	}
}

func ensureClusterAutoscalerCommandLineArgs(c *corev1.Container, k8sVersion *semver.Version) {
	if version.ConstraintK8sLess127.Check(k8sVersion) {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigration=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationAzureDisk=true", ",")
	}
	if version.ConstraintK8sLess130.Check(k8sVersion) {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationAzureFile=true", ",")
	}
	if constraintK8sLess131.Check(k8sVersion) {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"InTreePluginAzureDiskUnregister=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"InTreePluginAzureFileUnregister=true", ",")
	}
}

func ensureKubeControllerManagerLabels(t *corev1.PodTemplateSpec) {
	// TODO: This can be removed in a future version.
	delete(t.Labels, v1beta1constants.LabelNetworkPolicyToBlockedCIDRs)

	delete(t.Labels, v1beta1constants.LabelNetworkPolicyToPublicNetworks)
	delete(t.Labels, v1beta1constants.LabelNetworkPolicyToPrivateNetworks)
}

var (
	etcSSLName        = "etc-ssl"
	etcSSLVolumeMount = corev1.VolumeMount{
		Name:      etcSSLName,
		MountPath: "/etc/ssl",
		ReadOnly:  true,
	}
	etcSSLVolume = corev1.Volume{
		Name: etcSSLName,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/etc/ssl",
				Type: &directoryOrCreate,
			},
		},
	}

	usrShareCaCerts            = "usr-share-cacerts"
	directoryOrCreate          = corev1.HostPathDirectoryOrCreate
	usrShareCaCertsVolumeMount = corev1.VolumeMount{
		Name:      usrShareCaCerts,
		MountPath: "/usr/share/ca-certificates",
		ReadOnly:  true,
	}
	usrShareCaCertsVolume = corev1.Volume{
		Name: usrShareCaCerts,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/usr/share/ca-certificates",
				Type: &directoryOrCreate,
			},
		},
	}

	cloudProviderConfigVolumeMount = corev1.VolumeMount{
		Name:      azure.CloudProviderConfigName,
		MountPath: "/etc/kubernetes/cloudprovider",
	}
	cloudProviderConfigVolume = corev1.Volume{
		Name: azure.CloudProviderConfigName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: azure.CloudProviderConfigName,
			},
		},
	}
)

func ensureKubeControllerManagerVolumeMounts(c *corev1.Container, _ string) {
	c.VolumeMounts = extensionswebhook.EnsureNoVolumeMountWithName(c.VolumeMounts, etcSSLVolumeMount.Name)
	c.VolumeMounts = extensionswebhook.EnsureNoVolumeMountWithName(c.VolumeMounts, usrShareCaCertsVolumeMount.Name)
}

func ensureKubeControllerManagerVolumes(ps *corev1.PodSpec, _ string) {
	ps.Volumes = extensionswebhook.EnsureNoVolumeWithName(ps.Volumes, etcSSLVolume.Name)
	ps.Volumes = extensionswebhook.EnsureNoVolumeWithName(ps.Volumes, usrShareCaCertsVolume.Name)
}

func (e *ensurer) ensureChecksumAnnotations(template *corev1.PodTemplateSpec) error {
	delete(template.Annotations, "checksum/configmap-"+azure.CloudProviderConfigName)
	return nil
}

// EnsureKubeletServiceUnitOptions ensures that the kubelet.service unit options conform to the provider requirements.
func (e *ensurer) EnsureKubeletServiceUnitOptions(ctx context.Context, gctx gcontext.GardenContext, _ *semver.Version, new, _ []*unit.UnitOption) ([]*unit.UnitOption, error) {
	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return nil, err
	}

	if opt := extensionswebhook.UnitOptionWithSectionAndName(new, "Service", "ExecStart"); opt != nil {
		command := extensionswebhook.DeserializeCommandLine(opt.Value)
		command, err := e.ensureKubeletCommandLineArgs(ctx, cluster, command)
		if err != nil {
			return nil, err
		}
		opt.Value = extensionswebhook.SerializeCommandLine(command, 1, " \\\n    ")
	}
	return new, nil
}

func (e *ensurer) ensureKubeletCommandLineArgs(ctx context.Context, cluster *extensionscontroller.Cluster, command []string) ([]string, error) {
	command = extensionswebhook.EnsureStringWithPrefix(command, "--cloud-provider=", "external")

	acrConfigMap, err := e.getAcrConfigMap(ctx, cluster)
	if err != nil {
		return nil, err
	}

	if acrConfigMap != nil {
		command = extensionswebhook.EnsureStringWithPrefix(command, "--azure-container-registry-config=", acrConfigPath)
	}

	return command, nil
}

// EnsureKubeletConfiguration ensures that the kubelet configuration conforms to the provider requirements.
func (e *ensurer) EnsureKubeletConfiguration(_ context.Context, _ gcontext.GardenContext, kubeletVersion *semver.Version, new, _ *kubeletconfigv1beta1.KubeletConfiguration) error {
	if version.ConstraintK8sLess127.Check(kubeletVersion) {
		setKubeletConfigurationFeatureGate(new, "CSIMigration", true)
		setKubeletConfigurationFeatureGate(new, "CSIMigrationAzureDisk", true)
	}

	if version.ConstraintK8sLess130.Check(kubeletVersion) {
		setKubeletConfigurationFeatureGate(new, "CSIMigrationAzureFile", true)
	}
	if constraintK8sLess131.Check(kubeletVersion) {
		setKubeletConfigurationFeatureGate(new, "InTreePluginAzureDiskUnregister", true)
		setKubeletConfigurationFeatureGate(new, "InTreePluginAzureFileUnregister", true)
	}

	new.EnableControllerAttachDetach = ptr.To(true)

	return nil
}

func setKubeletConfigurationFeatureGate(kubeletConfiguration *kubeletconfigv1beta1.KubeletConfiguration, featureGate string, value bool) {
	if kubeletConfiguration.FeatureGates == nil {
		kubeletConfiguration.FeatureGates = make(map[string]bool)
	}

	kubeletConfiguration.FeatureGates[featureGate] = value
}

// ShouldProvisionKubeletCloudProviderConfig returns true if the cloud provider config file should be added to the kubelet configuration.
func (e *ensurer) ShouldProvisionKubeletCloudProviderConfig(_ context.Context, _ gcontext.GardenContext, _ *semver.Version) bool {
	return false
}

// EnsureKubeletCloudProviderConfig ensures that the cloud provider config file conforms to the provider requirements.
func (e *ensurer) EnsureKubeletCloudProviderConfig(ctx context.Context, _ gcontext.GardenContext, _ *semver.Version, data *string, namespace string) error {
	secret := &corev1.Secret{}
	if err := e.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: azure.CloudProviderDiskConfigName}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			e.logger.Info("secret not found", "name", azure.CloudProviderDiskConfigName, "namespace", namespace)
			return nil
		}

		return fmt.Errorf("could not get secret '%s/%s': %w", namespace, azure.CloudProviderDiskConfigName, err)
	}

	// Check if "cloudprovider.conf" is present
	if len(secret.Data[azure.CloudProviderConfigMapKey]) == 0 {
		return nil
	}

	// Overwrite data variable
	*data = string(secret.Data[azure.CloudProviderConfigMapKey])
	return nil
}

// EnsureAdditionalFiles ensures additional systemd files
func (e *ensurer) EnsureAdditionalFiles(ctx context.Context, gctx gcontext.GardenContext, new, _ *[]extensionsv1alpha1.File) error {
	return e.ensureAcrConfigFile(ctx, gctx, new)
}

func (e *ensurer) ensureAcrConfigFile(ctx context.Context, gctx gcontext.GardenContext, files *[]extensionsv1alpha1.File) error {
	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return err
	}

	// Check if the ACR configmap exists, if not nothing to do.
	cm, err := e.getAcrConfigMap(ctx, cluster)
	if err != nil {
		return err
	}
	if cm == nil {
		return nil
	}

	// Write the content of the file.
	fciCodec := oscutils.NewFileContentInlineCodec()
	fci, err := fciCodec.Encode([]byte(cm.Data[azure.CloudProviderAcrConfigMapKey]), string(extensionsv1alpha1.B64FileCodecID))
	if err != nil {
		return fmt.Errorf("could not encode acr cloud provider config: %w", err)
	}

	// Remove old ACR systemd file(s) before adding a new one.
	for i, f := range *files {
		if f.Path == acrConfigPath {
			l := *files
			*files = append(l[:i], l[i+1:]...)
		}
	}

	// Add new ACR systemd file.
	*files = append(*files, extensionsv1alpha1.File{
		Path:        acrConfigPath,
		Permissions: ptr.To[uint32](0644),
		Content: extensionsv1alpha1.FileContent{
			Inline: fci,
		},
	})
	return nil
}

func (e *ensurer) getAcrConfigMap(ctx context.Context, cluster *extensionscontroller.Cluster) (*corev1.ConfigMap, error) {
	if cluster == nil || cluster.Shoot == nil {
		return nil, fmt.Errorf("could not get cluster resource or cluster resource is invalid")
	}

	cm := &corev1.ConfigMap{}
	if err := e.client.Get(ctx, client.ObjectKey{Namespace: cluster.Shoot.Status.TechnicalID, Name: azure.CloudProviderAcrConfigName}, cm); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("could not get acr cloudprovider configmap '%s/%s': %w", cluster.Shoot.Status.TechnicalID, azure.CloudProviderAcrConfigName, err)
	}

	return cm, nil
}
