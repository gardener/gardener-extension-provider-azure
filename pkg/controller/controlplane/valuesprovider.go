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
	"path/filepath"
	"strings"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	azureapihelper "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/chart"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/version"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiserver/pkg/authentication/user"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Object names

var (
	controlPlaneSecrets = &secrets.Secrets{
		CertificateSecretConfigs: map[string]*secrets.CertificateSecretConfig{
			v1beta1constants.SecretNameCACluster: {
				Name:       v1beta1constants.SecretNameCACluster,
				CommonName: "kubernetes",
				CertType:   secrets.CACert,
			},
		},
		SecretConfigsFunc: func(cas map[string]*secrets.Certificate, clusterName string) []secrets.ConfigInterface {
			return []secrets.ConfigInterface{
				&secrets.ControlPlaneSecretConfig{
					CertificateSecretConfig: &secrets.CertificateSecretConfig{
						Name:         azure.CloudControllerManagerName,
						CommonName:   "system:cloud-controller-manager",
						Organization: []string{user.SystemPrivilegedGroup},
						CertType:     secrets.ClientCert,
						SigningCA:    cas[v1beta1constants.SecretNameCACluster],
					},
					KubeConfigRequest: &secrets.KubeConfigRequest{
						ClusterName:  clusterName,
						APIServerURL: v1beta1constants.DeploymentNameKubeAPIServer,
					},
				},
				&secrets.ControlPlaneSecretConfig{
					CertificateSecretConfig: &secrets.CertificateSecretConfig{
						Name:       azure.CloudControllerManagerName + "-server",
						CommonName: azure.CloudControllerManagerName,
						DNSNames:   kutil.DNSNamesForService(azure.CloudControllerManagerName, clusterName),
						CertType:   secrets.ServerCert,
						SigningCA:  cas[v1beta1constants.SecretNameCACluster],
					},
				},
				&secrets.ControlPlaneSecretConfig{
					CertificateSecretConfig: &secrets.CertificateSecretConfig{
						Name:       azure.CSIControllerFileName,
						CommonName: azure.UsernamePrefix + azure.CSIControllerFileName,
						CertType:   secrets.ClientCert,
						SigningCA:  cas[v1beta1constants.SecretNameCACluster],
					},
					KubeConfigRequest: &secrets.KubeConfigRequest{
						ClusterName:  clusterName,
						APIServerURL: v1beta1constants.DeploymentNameKubeAPIServer,
					},
				},
				&secrets.ControlPlaneSecretConfig{
					CertificateSecretConfig: &secrets.CertificateSecretConfig{
						Name:       azure.CSIProvisionerName,
						CommonName: azure.UsernamePrefix + azure.CSIProvisionerName,
						CertType:   secrets.ClientCert,
						SigningCA:  cas[v1beta1constants.SecretNameCACluster],
					},
					KubeConfigRequest: &secrets.KubeConfigRequest{
						ClusterName:  clusterName,
						APIServerURL: v1beta1constants.DeploymentNameKubeAPIServer,
					},
				},
				&secrets.ControlPlaneSecretConfig{
					CertificateSecretConfig: &secrets.CertificateSecretConfig{
						Name:       azure.CSIAttacherName,
						CommonName: azure.UsernamePrefix + azure.CSIAttacherName,
						CertType:   secrets.ClientCert,
						SigningCA:  cas[v1beta1constants.SecretNameCACluster],
					},
					KubeConfigRequest: &secrets.KubeConfigRequest{
						ClusterName:  clusterName,
						APIServerURL: v1beta1constants.DeploymentNameKubeAPIServer,
					},
				},
				&secrets.ControlPlaneSecretConfig{
					CertificateSecretConfig: &secrets.CertificateSecretConfig{
						Name:       azure.CSISnapshotterName,
						CommonName: azure.UsernamePrefix + azure.CSISnapshotterName,
						CertType:   secrets.ClientCert,
						SigningCA:  cas[v1beta1constants.SecretNameCACluster],
					},
					KubeConfigRequest: &secrets.KubeConfigRequest{
						ClusterName:  clusterName,
						APIServerURL: v1beta1constants.DeploymentNameKubeAPIServer,
					},
				},
				&secrets.ControlPlaneSecretConfig{
					CertificateSecretConfig: &secrets.CertificateSecretConfig{
						Name:       azure.CSIResizerName,
						CommonName: azure.UsernamePrefix + azure.CSIResizerName,
						CertType:   secrets.ClientCert,
						SigningCA:  cas[v1beta1constants.SecretNameCACluster],
					},
					KubeConfigRequest: &secrets.KubeConfigRequest{
						ClusterName:  clusterName,
						APIServerURL: v1beta1constants.DeploymentNameKubeAPIServer,
					},
				},
				&secrets.ControlPlaneSecretConfig{
					CertificateSecretConfig: &secrets.CertificateSecretConfig{
						Name:       azure.CSISnapshotControllerName,
						CommonName: azure.UsernamePrefix + azure.CSISnapshotControllerName,
						CertType:   secrets.ClientCert,
						SigningCA:  cas[v1beta1constants.SecretNameCACluster],
					},
					KubeConfigRequest: &secrets.KubeConfigRequest{
						ClusterName:  clusterName,
						APIServerURL: v1beta1constants.DeploymentNameKubeAPIServer,
					},
				},
				&secrets.ControlPlaneSecretConfig{
					CertificateSecretConfig: &secrets.CertificateSecretConfig{
						Name:       azure.RemedyControllerName,
						CommonName: azure.UsernamePrefix + azure.RemedyControllerName,
						CertType:   secrets.ClientCert,
						SigningCA:  cas[v1beta1constants.SecretNameCACluster],
					},
					KubeConfigRequest: &secrets.KubeConfigRequest{
						ClusterName:  clusterName,
						APIServerURL: v1beta1constants.DeploymentNameKubeAPIServer,
					},
				},
			}
		},
	}

	configChart = &chart.Chart{
		Name: "cloud-provider-config",
		Path: filepath.Join(azure.InternalChartsPath, "cloud-provider-config"),
		Objects: []*chart.Object{
			{
				Type: &corev1.Secret{},
				Name: azure.CloudProviderConfigName,
			},
			{
				Type: &corev1.Secret{},
				Name: azure.CloudProviderDiskConfigName,
			},
		},
	}

	controlPlaneChart = &chart.Chart{
		Name: "seed-controlplane",
		Path: filepath.Join(azure.InternalChartsPath, "seed-controlplane"),
		SubCharts: []*chart.Chart{
			{
				Name:   azure.CloudControllerManagerName,
				Images: []string{azure.CloudControllerManagerImageName},
				Objects: []*chart.Object{
					{Type: &corev1.Service{}, Name: azure.CloudControllerManagerName},
					{Type: &appsv1.Deployment{}, Name: azure.CloudControllerManagerName},
					{Type: &corev1.ConfigMap{}, Name: azure.CloudControllerManagerName + "-observability-config"},
					{Type: &autoscalingv1beta2.VerticalPodAutoscaler{}, Name: azure.CloudControllerManagerName + "-vpa"},
				},
			},
			{
				Name: azure.CSIControllerName,
				Images: []string{
					azure.CSIDriverDiskImageName,
					azure.CSIDriverFileImageName,
					azure.CSIProvisionerImageName,
					azure.CSIAttacherImageName,
					azure.CSISnapshotterImageName,
					azure.CSIResizerImageName,
					azure.CSILivenessProbeImageName,
					azure.CSISnapshotControllerImageName,
				},
				Objects: []*chart.Object{
					// csi-driver-controllers
					{Type: &appsv1.Deployment{}, Name: azure.CSIControllerDiskName},
					{Type: &appsv1.Deployment{}, Name: azure.CSIControllerFileName},
					{Type: &corev1.ConfigMap{}, Name: azure.CSIControllerObservabilityConfigName},
					{Type: &autoscalingv1beta2.VerticalPodAutoscaler{}, Name: azure.CSIControllerDiskName + "-vpa"},
					{Type: &autoscalingv1beta2.VerticalPodAutoscaler{}, Name: azure.CSIControllerFileName + "-vpa"},
					// csi-snapshot-controller
					{Type: &appsv1.Deployment{}, Name: azure.CSISnapshotControllerName},
					{Type: &autoscalingv1beta2.VerticalPodAutoscaler{}, Name: azure.CSISnapshotControllerName + "-vpa"},
				},
			},
			{
				Name:   azure.RemedyControllerName,
				Images: []string{azure.RemedyControllerImageName},
				Objects: []*chart.Object{
					{Type: &appsv1.Deployment{}, Name: azure.RemedyControllerName},
					{Type: &corev1.ConfigMap{}, Name: azure.RemedyControllerName + "-config"},
					{Type: &autoscalingv1beta2.VerticalPodAutoscaler{}, Name: azure.RemedyControllerName + "-vpa"},
					{Type: &rbacv1.Role{}, Name: azure.RemedyControllerName},
					{Type: &rbacv1.RoleBinding{}, Name: azure.RemedyControllerName},
					{Type: &corev1.ServiceAccount{}, Name: azure.RemedyControllerName},
				},
			},
		},
	}

	controlPlaneShootChart = &chart.Chart{
		Name: "shoot-system-components",
		Path: filepath.Join(azure.InternalChartsPath, "shoot-system-components"),
		SubCharts: []*chart.Chart{
			{
				Name: "allow-udp-egress",
				Objects: []*chart.Object{
					{Type: &corev1.Service{}, Name: "allow-udp-egress"},
				},
			},
			{
				Name: azure.CloudControllerManagerName,
				Objects: []*chart.Object{
					{Type: &rbacv1.ClusterRole{}, Name: "system:controller:cloud-node-controller"},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: "system:controller:cloud-node-controller"},
				},
			},
			{
				Name: azure.CSINodeName,
				Images: []string{
					azure.CSIDriverDiskImageName,
					azure.CSIDriverFileImageName,
					azure.CSINodeDriverRegistrarImageName,
					azure.CSILivenessProbeImageName,
				},
				Objects: []*chart.Object{
					// csi-driver
					{Type: &corev1.ConfigMap{}, Name: azure.CloudProviderDiskConfigName},
					{Type: &corev1.ServiceAccount{}, Name: azure.CSINodeDiskName},
					{Type: &corev1.ServiceAccount{}, Name: azure.CSINodeFileName},
					{Type: &appsv1.DaemonSet{}, Name: azure.CSINodeDiskName},
					{Type: &appsv1.DaemonSet{}, Name: azure.CSINodeFileName},
					{Type: &storagev1beta1.CSIDriver{}, Name: "disk.csi.azure.com"},
					{Type: &storagev1beta1.CSIDriver{}, Name: "file.csi.azure.com"},
					{Type: &rbacv1.ClusterRole{}, Name: azure.UsernamePrefix + azure.CSIDriverName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: azure.UsernamePrefix + azure.CSIDriverName},
					{Type: &rbacv1.ClusterRole{}, Name: azure.UsernamePrefix + azure.CSIControllerFileName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: azure.UsernamePrefix + azure.CSIControllerFileName},
					{Type: &policyv1beta1.PodSecurityPolicy{}, Name: strings.Replace(azure.UsernamePrefix+azure.CSIDriverName, ":", ".", -1)},
					{Type: extensionscontroller.GetVerticalPodAutoscalerObject(), Name: azure.CSINodeDiskName},
					{Type: extensionscontroller.GetVerticalPodAutoscalerObject(), Name: azure.CSINodeFileName},
					// csi-provisioner
					{Type: &rbacv1.ClusterRole{}, Name: azure.UsernamePrefix + azure.CSIProvisionerName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: azure.UsernamePrefix + azure.CSIProvisionerName},
					{Type: &rbacv1.Role{}, Name: azure.UsernamePrefix + azure.CSIProvisionerName},
					{Type: &rbacv1.RoleBinding{}, Name: azure.UsernamePrefix + azure.CSIProvisionerName},
					// csi-attacher
					{Type: &rbacv1.ClusterRole{}, Name: azure.UsernamePrefix + azure.CSIAttacherName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: azure.UsernamePrefix + azure.CSIAttacherName},
					{Type: &rbacv1.Role{}, Name: azure.UsernamePrefix + azure.CSIAttacherName},
					{Type: &rbacv1.RoleBinding{}, Name: azure.UsernamePrefix + azure.CSIAttacherName},
					// csi-snapshotter
					{Type: &rbacv1.ClusterRole{}, Name: azure.UsernamePrefix + azure.CSISnapshotterName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: azure.UsernamePrefix + azure.CSISnapshotterName},
					{Type: &rbacv1.Role{}, Name: azure.UsernamePrefix + azure.CSISnapshotterName},
					{Type: &rbacv1.RoleBinding{}, Name: azure.UsernamePrefix + azure.CSISnapshotterName},
					// csi-snapshot-controller
					{Type: &rbacv1.ClusterRole{}, Name: azure.UsernamePrefix + azure.CSISnapshotControllerName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: azure.UsernamePrefix + azure.CSISnapshotControllerName},
					{Type: &rbacv1.Role{}, Name: azure.UsernamePrefix + azure.CSISnapshotControllerName},
					{Type: &rbacv1.RoleBinding{}, Name: azure.UsernamePrefix + azure.CSISnapshotControllerName},
					// csi-resizer
					{Type: &rbacv1.ClusterRole{}, Name: azure.UsernamePrefix + azure.CSIResizerName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: azure.UsernamePrefix + azure.CSIResizerName},
					{Type: &rbacv1.Role{}, Name: azure.UsernamePrefix + azure.CSIResizerName},
					{Type: &rbacv1.RoleBinding{}, Name: azure.UsernamePrefix + azure.CSIResizerName},
				},
			},
			{
				Name: azure.RemedyControllerName,
				Objects: []*chart.Object{
					{Type: &rbacv1.ClusterRole{}, Name: azure.UsernamePrefix + azure.RemedyControllerName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: azure.UsernamePrefix + azure.RemedyControllerName},
				},
			},
		},
	}

	controlPlaneShootCRDsChart = &chart.Chart{
		Name: "shoot-crds",
		Path: filepath.Join(azure.InternalChartsPath, "shoot-crds"),
		SubCharts: []*chart.Chart{
			{
				Name: "volumesnapshots",
				Objects: []*chart.Object{
					{Type: &apiextensionsv1beta1.CustomResourceDefinition{}, Name: "volumesnapshotclasses.snapshot.storage.k8s.io"},
					{Type: &apiextensionsv1beta1.CustomResourceDefinition{}, Name: "volumesnapshotcontents.snapshot.storage.k8s.io"},
					{Type: &apiextensionsv1beta1.CustomResourceDefinition{}, Name: "volumesnapshots.snapshot.storage.k8s.io"},
				},
			},
		},
	}

	storageClassChart = &chart.Chart{
		Name: "shoot-storageclasses",
		Path: filepath.Join(azure.InternalChartsPath, "shoot-storageclasses"),
	}
)

// NewValuesProvider creates a new ValuesProvider for the generic actuator.
func NewValuesProvider(logger logr.Logger) genericactuator.ValuesProvider {
	return &valuesProvider{
		logger: logger.WithName("azure-values-provider"),
	}
}

// valuesProvider is a ValuesProvider that provides azure-specific values for the 2 charts applied by the generic actuator.
type valuesProvider struct {
	genericactuator.NoopValuesProvider
	logger logr.Logger
}

// GetConfigChartValues returns the values for the config chart applied by the generic actuator.
func (vp *valuesProvider) GetConfigChartValues(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) (map[string]interface{}, error) {
	// Decode providerConfig
	cpConfig := &apisazure.ControlPlaneConfig{}
	if cp.Spec.ProviderConfig != nil {
		if _, _, err := vp.Decoder().Decode(cp.Spec.ProviderConfig.Raw, nil, cpConfig); err != nil {
			return nil, errors.Wrapf(err, "could not decode providerConfig of controlplane '%s'", kutil.ObjectName(cp))
		}
	}

	// Decode infrastructureProviderStatus
	infraStatus := &apisazure.InfrastructureStatus{}
	if _, _, err := vp.Decoder().Decode(cp.Spec.InfrastructureProviderStatus.Raw, nil, infraStatus); err != nil {
		return nil, errors.Wrapf(err, "could not decode infrastructureProviderStatus of controlplane '%s'", kutil.ObjectName(cp))
	}

	// Get client auth
	auth, err := internal.GetClientAuthData(ctx, vp.Client(), cp.Spec.SecretRef)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get service account from secret '%s/%s'", cp.Spec.SecretRef.Namespace, cp.Spec.SecretRef.Name)
	}

	// Check if the configmap for the acr access need to be removed.
	if infraStatus.Identity == nil || !infraStatus.Identity.ACRAccess {
		if err := vp.removeAcrConfig(ctx, cp.Namespace); err != nil {
			return nil, errors.Wrap(err, "could not remove acr config map")
		}
	}

	// Get config chart values
	return getConfigChartValues(infraStatus, cp, cluster, auth)
}

// GetControlPlaneChartValues returns the values for the control plane chart applied by the generic actuator.
func (vp *valuesProvider) GetControlPlaneChartValues(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	checksums map[string]string,
	scaledDown bool,
) (map[string]interface{}, error) {
	// Decode providerConfig
	cpConfig := &apisazure.ControlPlaneConfig{}
	if cp.Spec.ProviderConfig != nil {
		if _, _, err := vp.Decoder().Decode(cp.Spec.ProviderConfig.Raw, nil, cpConfig); err != nil {
			return nil, errors.Wrapf(err, "could not decode providerConfig of controlplane '%s'", kutil.ObjectName(cp))
		}
	}

	cpConfigSecret := &corev1.Secret{}
	if err := vp.Client().Get(ctx, kutil.Key(cp.Namespace, azure.CloudProviderConfigName), cpConfigSecret); err != nil {
		return nil, err
	}
	checksums[azure.CloudProviderConfigName] = utils.ComputeChecksum(cpConfigSecret.Data)

	// TODO: Remove this code in next version. Delete old config
	if err := vp.deleteCCMMonitoringConfig(ctx, cp.Namespace); err != nil {
		return nil, err
	}

	return getControlPlaneChartValues(cpConfig, cp, cluster, checksums, scaledDown)
}

// GetControlPlaneShootChartValues returns the values for the control plane shoot chart applied by the generic actuator.
func (vp *valuesProvider) GetControlPlaneShootChartValues(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	checksums map[string]string,
) (map[string]interface{}, error) {
	// Decode infrastructureProviderStatus
	infraStatus := &apisazure.InfrastructureStatus{}
	if _, _, err := vp.Decoder().Decode(cp.Spec.InfrastructureProviderStatus.Raw, nil, infraStatus); err != nil {
		return nil, errors.Wrapf(err, "could not decode infrastructureProviderStatus of controlplane '%s'", kutil.ObjectName(cp))
	}

	k8sVersionLessThan121, err := version.CompareVersions(cluster.Shoot.Spec.Kubernetes.Version, "<", "1.21")
	if err != nil {
		return nil, err
	}

	var (
		cloudProviderDiskConfig         string
		cloudProviderDiskConfigChecksum string
	)

	if !k8sVersionLessThan121 {
		secret := &corev1.Secret{}
		if err := vp.Client().Get(ctx, kutil.Key(cp.Namespace, azure.CloudProviderDiskConfigName), secret); err != nil {
			return nil, err
		}

		cloudProviderDiskConfig = string(secret.Data[azure.CloudProviderConfigMapKey])
		cloudProviderDiskConfigChecksum = utils.ComputeChecksum(secret.Data)
	}

	disableRemedyController := cluster.Shoot.Annotations[azure.DisableRemedyControllerAnnotation] == "true"

	return getControlPlaneShootChartValues(cluster, infraStatus, k8sVersionLessThan121, disableRemedyController, cloudProviderDiskConfig, cloudProviderDiskConfigChecksum), nil
}

// GetControlPlaneShootCRDsChartValues returns the values for the control plane shoot CRDs chart applied by the generic actuator.
func (vp *valuesProvider) GetControlPlaneShootCRDsChartValues(
	_ context.Context,
	_ *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (map[string]interface{}, error) {
	k8sVersionLessThan121, err := version.CompareVersions(cluster.Shoot.Spec.Kubernetes.Version, "<", "1.21")
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"volumesnapshots": map[string]interface{}{
			"enabled": !k8sVersionLessThan121,
		},
	}, nil
}

// GetStorageClassesChartValues returns the values for the storage classes chart applied by the generic actuator.
func (vp *valuesProvider) GetStorageClassesChartValues(
	_ context.Context,
	_ *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (map[string]interface{}, error) {
	k8sVersionLessThan121, err := version.CompareVersions(cluster.Shoot.Spec.Kubernetes.Version, "<", "1.21")
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"useLegacyProvisioner": k8sVersionLessThan121,
	}, nil
}

func (vp *valuesProvider) removeAcrConfig(ctx context.Context, namespace string) error {
	cm := corev1.ConfigMap{}
	cm.SetName(azure.CloudProviderAcrConfigName)
	cm.SetNamespace(namespace)
	return client.IgnoreNotFound(vp.Client().Delete(ctx, &cm))
}

// getConfigChartValues collects and returns the configuration chart values.
func getConfigChartValues(infraStatus *apisazure.InfrastructureStatus, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster, ca *internal.ClientAuth) (map[string]interface{}, error) {
	subnetName, routeTableName, securityGroupName, err := getInfraNames(infraStatus)
	if err != nil {
		return nil, errors.Wrapf(err, "could not determine subnet, availability set, route table or security group name from infrastructureStatus of controlplane '%s'", kutil.ObjectName(cp))
	}

	var maxNodes int32
	for _, worker := range cluster.Shoot.Spec.Provider.Workers {
		maxNodes = maxNodes + worker.Maximum
	}

	// Collect config chart values.
	values := map[string]interface{}{
		"kubernetesVersion": cluster.Shoot.Spec.Kubernetes.Version,
		"tenantId":          ca.TenantID,
		"subscriptionId":    ca.SubscriptionID,
		"aadClientId":       ca.ClientID,
		"aadClientSecret":   ca.ClientSecret,
		"resourceGroup":     infraStatus.ResourceGroup.Name,
		"vnetName":          infraStatus.Networks.VNet.Name,
		"subnetName":        subnetName,
		"routeTableName":    routeTableName,
		"securityGroupName": securityGroupName,
		"region":            cp.Spec.Region,
		"maxNodes":          maxNodes,
	}

	if infraStatus.Networks.VNet.ResourceGroup != nil {
		values["vnetResourceGroup"] = *infraStatus.Networks.VNet.ResourceGroup
	}

	if infraStatus.Identity != nil && infraStatus.Identity.ACRAccess {
		values["acrIdentityClientId"] = infraStatus.Identity.ClientID
	}

	return appendMachineSetValues(values, infraStatus), nil
}

func appendMachineSetValues(values map[string]interface{}, infraStatus *apisazure.InfrastructureStatus) map[string]interface{} {
	if azureapihelper.IsVmoRequired(infraStatus) {
		values["vmType"] = "vmss"
		return values
	}

	if primaryAvailabilitySet, err := azureapihelper.FindAvailabilitySetByPurpose(infraStatus.AvailabilitySets, apisazure.PurposeNodes); err == nil {
		values["availabilitySetName"] = primaryAvailabilitySet.Name
		return values
	}

	return values
}

// getInfraNames determines the subnet, availability set, route table and security group names from the given infrastructure status.
func getInfraNames(infraStatus *apisazure.InfrastructureStatus) (string, string, string, error) {
	nodesSubnet, err := azureapihelper.FindSubnetByPurpose(infraStatus.Networks.Subnets, apisazure.PurposeNodes)
	if err != nil {
		return "", "", "", errors.Wrapf(err, "could not determine subnet for purpose 'nodes'")
	}
	nodesRouteTable, err := azureapihelper.FindRouteTableByPurpose(infraStatus.RouteTables, apisazure.PurposeNodes)
	if err != nil {
		return "", "", "", errors.Wrapf(err, "could not determine route table for purpose 'nodes'")
	}
	nodesSecurityGroup, err := azureapihelper.FindSecurityGroupByPurpose(infraStatus.SecurityGroups, apisazure.PurposeNodes)
	if err != nil {
		return "", "", "", errors.Wrapf(err, "could not determine security group for purpose 'nodes'")
	}

	return nodesSubnet.Name, nodesRouteTable.Name, nodesSecurityGroup.Name, nil
}

// getControlPlaneChartValues collects and returns the control plane chart values.
func getControlPlaneChartValues(
	cpConfig *apisazure.ControlPlaneConfig,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	checksums map[string]string,
	scaledDown bool,
) (map[string]interface{}, error) {
	ccm, err := getCCMChartValues(cpConfig, cp, cluster, checksums, scaledDown)
	if err != nil {
		return nil, err
	}

	csi, err := getCSIControllerChartValues(cluster, checksums, scaledDown)
	if err != nil {
		return nil, err
	}

	remedy, err := getRemedyControllerChartValues(cluster, checksums, scaledDown)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		azure.CloudControllerManagerName: ccm,
		azure.CSIControllerName:          csi,
		azure.RemedyControllerName:       remedy,
	}, nil
}

// getCCMChartValues collects and returns the CCM chart values.
func getCCMChartValues(
	cpConfig *apisazure.ControlPlaneConfig,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	checksums map[string]string,
	scaledDown bool,
) (map[string]interface{}, error) {
	values := map[string]interface{}{
		"enabled":           true,
		"replicas":          extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
		"clusterName":       cp.Namespace,
		"kubernetesVersion": cluster.Shoot.Spec.Kubernetes.Version,
		"podNetwork":        extensionscontroller.GetPodNetwork(cluster),
		"podAnnotations": map[string]interface{}{
			"checksum/secret-" + azure.CloudControllerManagerName:             checksums[azure.CloudControllerManagerName],
			"checksum/secret-" + azure.CloudControllerManagerName + "-server": checksums[azure.CloudControllerManagerName+"-server"],
			"checksum/secret-" + v1beta1constants.SecretNameCloudProvider:     checksums[v1beta1constants.SecretNameCloudProvider],
			"checksum/secret-" + azure.CloudProviderConfigName:                checksums[azure.CloudProviderConfigName],
		},
		"podLabels": map[string]interface{}{
			v1beta1constants.LabelPodMaintenanceRestart: "true",
		},
	}

	if cpConfig.CloudControllerManager != nil {
		values["featureGates"] = cpConfig.CloudControllerManager.FeatureGates
	}

	return values, nil
}

// getCSIControllerChartValues collects and returns the CSIController chart values.
func getCSIControllerChartValues(
	cluster *extensionscontroller.Cluster,
	checksums map[string]string,
	scaledDown bool,
) (map[string]interface{}, error) {
	k8sVersionLessThan121, err := version.CompareVersions(cluster.Shoot.Spec.Kubernetes.Version, "<", "1.21")
	if err != nil {
		return nil, err
	}

	if k8sVersionLessThan121 {
		return map[string]interface{}{"enabled": false}, nil
	}

	return map[string]interface{}{
		"enabled":  true,
		"replicas": extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
		"podAnnotations": map[string]interface{}{
			"checksum/secret-" + azure.CSIControllerFileName:   checksums[azure.CSIControllerFileName],
			"checksum/secret-" + azure.CSIProvisionerName:      checksums[azure.CSIProvisionerName],
			"checksum/secret-" + azure.CSIAttacherName:         checksums[azure.CSIAttacherName],
			"checksum/secret-" + azure.CSISnapshotterName:      checksums[azure.CSISnapshotterName],
			"checksum/secret-" + azure.CSIResizerName:          checksums[azure.CSIResizerName],
			"checksum/secret-" + azure.CloudProviderConfigName: checksums[azure.CloudProviderConfigName],
		},
		"csiSnapshotController": map[string]interface{}{
			"replicas": extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
			"podAnnotations": map[string]interface{}{
				"checksum/secret-" + azure.CSISnapshotControllerName: checksums[azure.CSISnapshotControllerName],
			},
		},
	}, nil
}

// getRemedyControllerChartValues collects and returns the remedy controller chart values.
func getRemedyControllerChartValues(
	cluster *extensionscontroller.Cluster,
	checksums map[string]string,
	scaledDown bool,
) (map[string]interface{}, error) {
	disableRemedyController := cluster.Shoot.Annotations[azure.DisableRemedyControllerAnnotation] == "true"
	if disableRemedyController {
		return map[string]interface{}{"enabled": false}, nil
	}

	return map[string]interface{}{
		"enabled":  true,
		"replicas": extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
		"podAnnotations": map[string]interface{}{
			"checksum/secret-" + azure.RemedyControllerName:    checksums[azure.RemedyControllerName],
			"checksum/secret-" + azure.CloudProviderConfigName: checksums[azure.CloudProviderConfigName],
		},
	}, nil
}

// getControlPlaneShootChartValues collects and returns the control plane shoot chart values.
func getControlPlaneShootChartValues(
	cluster *extensionscontroller.Cluster,
	infraStatus *apisazure.InfrastructureStatus,
	k8sVersionLessThan121 bool,
	disableRemedyController bool,
	cloudProviderDiskConfig string,
	cloudProviderDiskConfigChecksum string,
) map[string]interface{} {
	return map[string]interface{}{
		azure.AllowUDPEgressName:         map[string]interface{}{"enabled": infraStatus.Zoned},
		azure.CloudControllerManagerName: map[string]interface{}{"enabled": true},
		azure.CSINodeName: map[string]interface{}{
			"enabled":    !k8sVersionLessThan121,
			"vpaEnabled": gardencorev1beta1helper.ShootWantsVerticalPodAutoscaler(cluster.Shoot),
			"podAnnotations": map[string]interface{}{
				"checksum/configmap-" + azure.CloudProviderDiskConfigName: cloudProviderDiskConfigChecksum,
			},
			"cloudProviderConfig": cloudProviderDiskConfig,
		},
		azure.RemedyControllerName: map[string]interface{}{"enabled": !disableRemedyController},
	}
}
