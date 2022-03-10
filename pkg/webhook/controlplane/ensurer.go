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
	"fmt"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"

	"github.com/Masterminds/semver"
	"github.com/coreos/go-systemd/v22/unit"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/csimigration"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	oscutils "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/version"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	acrConfigPath = "/var/lib/kubelet/acr.conf"
)

// NewEnsurer creates a new controlplane ensurer.
func NewEnsurer(logger logr.Logger) genericmutator.Ensurer {
	return &ensurer{
		logger: logger.WithName("azure-controlplane-ensurer"),
	}
}

type ensurer struct {
	genericmutator.NoopEnsurer
	client client.Client
	logger logr.Logger
}

// InjectClient injects the given client into the ensurer.
func (e *ensurer) InjectClient(client client.Client) error {
	e.client = client
	return nil
}

// EnsureKubeAPIServerDeployment ensures that the kube-apiserver deployment conforms to the provider requirements.
func (e *ensurer) EnsureKubeAPIServerDeployment(ctx context.Context, gctx gcontext.GardenContext, new, _ *appsv1.Deployment) error {
	template := &new.Spec.Template
	ps := &template.Spec

	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return err
	}

	csiEnabled, csiMigrationComplete, err := csimigration.CheckCSIConditions(cluster, azure.CSIMigrationKubernetesVersion)
	if err != nil {
		return err
	}

	if c := extensionswebhook.ContainerWithName(ps.Containers, "kube-apiserver"); c != nil {
		ensureKubeAPIServerCommandLineArgs(c, csiEnabled, csiMigrationComplete)
		ensureKubeAPIServerVolumeMounts(c, csiEnabled, csiMigrationComplete)
	}

	ensureKubeAPIServerVolumes(ps, csiEnabled, csiMigrationComplete)
	return e.ensureChecksumAnnotations(ctx, &new.Spec.Template, new.Namespace, csiEnabled, csiMigrationComplete)
}

// EnsureKubeControllerManagerDeployment ensures that the kube-controller-manager deployment conforms to the provider requirements.
func (e *ensurer) EnsureKubeControllerManagerDeployment(ctx context.Context, gctx gcontext.GardenContext, new, _ *appsv1.Deployment) error {
	template := &new.Spec.Template
	ps := &template.Spec

	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return err
	}

	csiEnabled, csiMigrationComplete, err := csimigration.CheckCSIConditions(cluster, azure.CSIMigrationKubernetesVersion)
	if err != nil {
		return err
	}

	if c := extensionswebhook.ContainerWithName(ps.Containers, "kube-controller-manager"); c != nil {
		ensureKubeControllerManagerCommandLineArgs(c, csiEnabled, csiMigrationComplete)
		ensureKubeControllerManagerVolumeMounts(c, cluster.Shoot.Spec.Kubernetes.Version, csiEnabled, csiMigrationComplete)
	}

	ensureKubeControllerManagerLabels(template, csiEnabled, csiMigrationComplete)
	ensureKubeControllerManagerVolumes(ps, cluster.Shoot.Spec.Kubernetes.Version, csiEnabled, csiMigrationComplete)
	return e.ensureChecksumAnnotations(ctx, &new.Spec.Template, new.Namespace, csiEnabled, csiMigrationComplete)
}

// EnsureKubeSchedulerDeployment ensures that the kube-scheduler deployment conforms to the provider requirements.
func (e *ensurer) EnsureKubeSchedulerDeployment(ctx context.Context, gctx gcontext.GardenContext, new, _ *appsv1.Deployment) error {
	template := &new.Spec.Template
	ps := &template.Spec

	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return err
	}

	csiEnabled, csiMigrationComplete, err := csimigration.CheckCSIConditions(cluster, azure.CSIMigrationKubernetesVersion)
	if err != nil {
		return err
	}

	if c := extensionswebhook.ContainerWithName(ps.Containers, "kube-scheduler"); c != nil {
		ensureKubeSchedulerCommandLineArgs(c, csiEnabled, csiMigrationComplete)
	}
	return nil
}

func ensureKubeAPIServerCommandLineArgs(c *corev1.Container, csiEnabled, csiMigrationComplete bool) {
	if csiEnabled {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigration=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationAzureDisk=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationAzureFile=true", ",")

		if csiMigrationComplete {
			c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
				"InTreePluginAzureDiskUnregister=true", ",")
			c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
				"InTreePluginAzureFileUnregister=true", ",")
			c.Command = extensionswebhook.EnsureNoStringWithPrefix(c.Command, "--cloud-provider=")
			c.Command = extensionswebhook.EnsureNoStringWithPrefix(c.Command, "--cloud-config=")
			c.Command = extensionswebhook.EnsureNoStringWithPrefixContains(c.Command, "--enable-admission-plugins=",
				"PersistentVolumeLabel", ",")
			c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--disable-admission-plugins=",
				"PersistentVolumeLabel", ",")
			return
		}
	}

	c.Command = extensionswebhook.EnsureStringWithPrefix(c.Command, "--cloud-provider=", "azure")
	c.Command = extensionswebhook.EnsureStringWithPrefix(c.Command, "--cloud-config=",
		"/etc/kubernetes/cloudprovider/cloudprovider.conf")
	c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--enable-admission-plugins=",
		"PersistentVolumeLabel", ",")
	c.Command = extensionswebhook.EnsureNoStringWithPrefixContains(c.Command, "--disable-admission-plugins=",
		"PersistentVolumeLabel", ",")
}

func ensureKubeControllerManagerCommandLineArgs(c *corev1.Container, csiEnabled, csiMigrationComplete bool) {
	c.Command = extensionswebhook.EnsureStringWithPrefix(c.Command, "--cloud-provider=", "external")

	if csiEnabled {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigration=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationAzureDisk=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationAzureFile=true", ",")

		if csiMigrationComplete {
			c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
				"InTreePluginAzureDiskUnregister=true", ",")
			c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
				"InTreePluginAzureFileUnregister=true", ",")
			c.Command = extensionswebhook.EnsureNoStringWithPrefix(c.Command, "--cloud-config=")
			c.Command = extensionswebhook.EnsureNoStringWithPrefix(c.Command, "--external-cloud-volume-plugin=")
			return
		}
	}

	c.Command = extensionswebhook.EnsureStringWithPrefix(c.Command, "--cloud-config=",
		"/etc/kubernetes/cloudprovider/cloudprovider.conf")
	c.Command = extensionswebhook.EnsureStringWithPrefix(c.Command, "--external-cloud-volume-plugin=", "azure")
}

func ensureKubeSchedulerCommandLineArgs(c *corev1.Container, csiEnabled, csiMigrationComplete bool) {
	if csiEnabled {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigration=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationAzureDisk=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationAzureFile=true", ",")

		if csiMigrationComplete {
			c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
				"InTreePluginAzureDiskUnregister=true", ",")
			c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
				"InTreePluginAzureFileUnregister=true", ",")
			return
		}
	}
}

func ensureKubeControllerManagerLabels(t *corev1.PodTemplateSpec, csiEnabled, csiMigrationComplete bool) {
	// TODO: This can be removed in a future version.
	delete(t.Labels, v1beta1constants.LabelNetworkPolicyToBlockedCIDRs)

	if csiEnabled && csiMigrationComplete {
		delete(t.Labels, v1beta1constants.LabelNetworkPolicyToPublicNetworks)
		delete(t.Labels, v1beta1constants.LabelNetworkPolicyToPrivateNetworks)
		return
	}

	t.Labels = extensionswebhook.EnsureAnnotationOrLabel(t.Labels, v1beta1constants.LabelNetworkPolicyToPublicNetworks, v1beta1constants.LabelNetworkPolicyAllowed)
	t.Labels = extensionswebhook.EnsureAnnotationOrLabel(t.Labels, v1beta1constants.LabelNetworkPolicyToPrivateNetworks, v1beta1constants.LabelNetworkPolicyAllowed)
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

func ensureKubeAPIServerVolumeMounts(c *corev1.Container, csiEnabled, csiMigrationComplete bool) {
	if csiEnabled && csiMigrationComplete {
		c.VolumeMounts = extensionswebhook.EnsureNoVolumeMountWithName(c.VolumeMounts, cloudProviderConfigVolumeMount.Name)
		return
	}

	c.VolumeMounts = extensionswebhook.EnsureVolumeMountWithName(c.VolumeMounts, cloudProviderConfigVolumeMount)
}

func ensureKubeControllerManagerVolumeMounts(c *corev1.Container, version string, csiEnabled, csiMigrationComplete bool) {
	if csiEnabled && csiMigrationComplete {
		c.VolumeMounts = extensionswebhook.EnsureNoVolumeMountWithName(c.VolumeMounts, cloudProviderConfigVolumeMount.Name)
		c.VolumeMounts = extensionswebhook.EnsureNoVolumeMountWithName(c.VolumeMounts, etcSSLVolumeMount.Name)
		c.VolumeMounts = extensionswebhook.EnsureNoVolumeMountWithName(c.VolumeMounts, usrShareCaCertsVolumeMount.Name)
		return
	}

	c.VolumeMounts = extensionswebhook.EnsureVolumeMountWithName(c.VolumeMounts, cloudProviderConfigVolumeMount)
	if mustMountEtcSSLFolder(version) {
		c.VolumeMounts = extensionswebhook.EnsureVolumeMountWithName(c.VolumeMounts, etcSSLVolumeMount)
		// some distros have symlinks from /etc/ssl/certs to /usr/share/ca-certificates
		c.VolumeMounts = extensionswebhook.EnsureVolumeMountWithName(c.VolumeMounts, usrShareCaCertsVolumeMount)
	}
}

func ensureKubeAPIServerVolumes(ps *corev1.PodSpec, csiEnabled, csiMigrationComplete bool) {
	if csiEnabled && csiMigrationComplete {
		ps.Volumes = extensionswebhook.EnsureNoVolumeWithName(ps.Volumes, cloudProviderConfigVolume.Name)
		return
	}

	ps.Volumes = extensionswebhook.EnsureVolumeWithName(ps.Volumes, cloudProviderConfigVolume)
}

func ensureKubeControllerManagerVolumes(ps *corev1.PodSpec, version string, csiEnabled, csiMigrationComplete bool) {
	if csiEnabled && csiMigrationComplete {
		ps.Volumes = extensionswebhook.EnsureNoVolumeWithName(ps.Volumes, cloudProviderConfigVolume.Name)
		ps.Volumes = extensionswebhook.EnsureNoVolumeWithName(ps.Volumes, etcSSLVolume.Name)
		ps.Volumes = extensionswebhook.EnsureNoVolumeWithName(ps.Volumes, usrShareCaCertsVolume.Name)
		return
	}

	ps.Volumes = extensionswebhook.EnsureVolumeWithName(ps.Volumes, cloudProviderConfigVolume)
	if mustMountEtcSSLFolder(version) {
		ps.Volumes = extensionswebhook.EnsureVolumeWithName(ps.Volumes, etcSSLVolume)
		// some distros have symlinks from /etc/ssl/certs to /usr/share/ca-certificates
		ps.Volumes = extensionswebhook.EnsureVolumeWithName(ps.Volumes, usrShareCaCertsVolume)
	}
}

// Beginning with 1.17 Gardener no longer uses the hyperkube image for the Kubernetes control plane components.
// The hyperkube image contained all the well-known root CAs, but the dedicated images don't. This is why we
// mount the /etc/ssl folder from the host here.
func mustMountEtcSSLFolder(version string) bool {
	k8sVersionAtLeast117, err := versionutils.CompareVersions(version, ">=", "1.17")
	if err != nil {
		return false
	}
	return k8sVersionAtLeast117
}

func (e *ensurer) ensureChecksumAnnotations(ctx context.Context, template *corev1.PodTemplateSpec, namespace string, csiEnabled, csiMigrationComplete bool) error {
	if csiEnabled && csiMigrationComplete {
		delete(template.Annotations, "checksum/configmap-"+azure.CloudProviderConfigName)
		return nil
	}

	return controlplane.EnsureSecretChecksumAnnotation(ctx, template, e.client, namespace, azure.CloudProviderConfigName)
}

// EnsureKubeletServiceUnitOptions ensures that the kubelet.service unit options conform to the provider requirements.
func (e *ensurer) EnsureKubeletServiceUnitOptions(ctx context.Context, gctx gcontext.GardenContext, kubeletVersion *semver.Version, new, _ []*unit.UnitOption) ([]*unit.UnitOption, error) {
	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return nil, err
	}

	csiEnabled, _, err := csimigration.CheckCSIConditions(cluster, azure.CSIMigrationKubernetesVersion)
	if err != nil {
		return nil, err
	}

	if opt := extensionswebhook.UnitOptionWithSectionAndName(new, "Service", "ExecStart"); opt != nil {
		command := extensionswebhook.DeserializeCommandLine(opt.Value)
		command, err := e.ensureKubeletCommandLineArgs(ctx, cluster, command, csiEnabled, kubeletVersion)
		if err != nil {
			return nil, err
		}
		opt.Value = extensionswebhook.SerializeCommandLine(command, 1, " \\\n    ")
	}
	return new, nil
}

func (e *ensurer) ensureKubeletCommandLineArgs(ctx context.Context, cluster *extensionscontroller.Cluster, command []string, csiEnabled bool, kubeletVersion *semver.Version) ([]string, error) {
	if csiEnabled {
		command = extensionswebhook.EnsureStringWithPrefix(command, "--cloud-provider=", "external")

		if !version.ConstraintK8sGreaterEqual123.Check(kubeletVersion) {
			command = extensionswebhook.EnsureStringWithPrefix(command, "--enable-controller-attach-detach=", "true")
		}
	} else {
		command = extensionswebhook.EnsureStringWithPrefix(command, "--cloud-provider=", "azure")
		command = extensionswebhook.EnsureStringWithPrefix(command, "--cloud-config=", "/var/lib/kubelet/cloudprovider.conf")
	}

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
func (e *ensurer) EnsureKubeletConfiguration(ctx context.Context, gctx gcontext.GardenContext, kubeletVersion *semver.Version, new, _ *kubeletconfigv1beta1.KubeletConfiguration) error {
	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return err
	}

	csiEnabled, _, err := csimigration.CheckCSIConditions(cluster, azure.CSIMigrationKubernetesVersion)
	if err != nil {
		return err
	}

	if csiEnabled {
		if new.FeatureGates == nil {
			new.FeatureGates = make(map[string]bool)
		}

		new.FeatureGates["CSIMigration"] = true
		new.FeatureGates["CSIMigrationAzureDisk"] = true
		new.FeatureGates["CSIMigrationAzureFile"] = true
		// kubelets of new worker nodes can directly be started with the `InTreePluginAzure<*>Unregister` feature gates
		new.FeatureGates["InTreePluginAzureDiskUnregister"] = true
		new.FeatureGates["InTreePluginAzureFileUnregister"] = true

		if version.ConstraintK8sGreaterEqual123.Check(kubeletVersion) {
			new.EnableControllerAttachDetach = pointer.Bool(true)
		}
	}

	return nil
}

// ShouldProvisionKubeletCloudProviderConfig returns true if the cloud provider config file should be added to the kubelet configuration.
func (e *ensurer) ShouldProvisionKubeletCloudProviderConfig(ctx context.Context, gctx gcontext.GardenContext, _ *semver.Version) bool {
	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return false
	}

	csiEnabled, _, err := csimigration.CheckCSIConditions(cluster, azure.CSIMigrationKubernetesVersion)
	if err != nil {
		return false
	}

	return !csiEnabled
}

// EnsureKubeletCloudProviderConfig ensures that the cloud provider config file conforms to the provider requirements.
func (e *ensurer) EnsureKubeletCloudProviderConfig(ctx context.Context, _ gcontext.GardenContext, _ *semver.Version, data *string, namespace string) error {
	secret := &corev1.Secret{}
	if err := e.client.Get(ctx, kutil.Key(namespace, azure.CloudProviderDiskConfigName), secret); err != nil {
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
		Permissions: pointer.Int32Ptr(0644),
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
	if err := e.client.Get(ctx, kutil.Key(cluster.Shoot.Status.TechnicalID, azure.CloudProviderAcrConfigName), cm); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("could not get acr cloudprovider configmap '%s/%s': %w", cluster.Shoot.Status.TechnicalID, azure.CloudProviderAcrConfigName, err)
	}

	return cm, nil
}
