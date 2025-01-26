// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	extensionssecretsmanager "github.com/gardener/gardener/extensions/pkg/util/secret/manager"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	securityv1alpha1constants "github.com/gardener/gardener/pkg/apis/security/v1alpha1/constants"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/chart"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	autoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-azure/charts"
	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	azureapihelper "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener-extension-provider-azure/pkg/features"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

// Object names

const (
	caNameControlPlane               = "ca-" + azure.Name + "-controlplane"
	cloudControllerManagerServerName = azure.CloudControllerManagerName + "-server"
	csiSnapshotValidationServerName  = azure.CSISnapshotValidationName + "-server"
)

func secretConfigsFunc(namespace string) []extensionssecretsmanager.SecretConfigWithOptions {
	return []extensionssecretsmanager.SecretConfigWithOptions{
		{
			Config: &secretutils.CertificateSecretConfig{
				Name:       caNameControlPlane,
				CommonName: caNameControlPlane,
				CertType:   secretutils.CACert,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.Persist()},
		},
		{
			Config: &secretutils.CertificateSecretConfig{
				Name:                        cloudControllerManagerServerName,
				CommonName:                  azure.CloudControllerManagerName,
				DNSNames:                    kutil.DNSNamesForService(azure.CloudControllerManagerName, namespace),
				CertType:                    secretutils.ServerCert,
				SkipPublishingCACertificate: true,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.SignedByCA(caNameControlPlane)},
		},
		{
			Config: &secretutils.CertificateSecretConfig{
				Name:                        csiSnapshotValidationServerName,
				CommonName:                  azure.UsernamePrefix + azure.CSISnapshotValidationName,
				DNSNames:                    kutil.DNSNamesForService(azure.CSISnapshotValidationName, namespace),
				CertType:                    secretutils.ServerCert,
				SkipPublishingCACertificate: true,
			},
			// use current CA for signing server cert to prevent mismatches when dropping the old CA from the webhook
			// config in phase Completing
			Options: []secretsmanager.GenerateOption{secretsmanager.SignedByCA(caNameControlPlane, secretsmanager.UseCurrentCA)},
		},
	}
}

func shootAccessSecretsFunc(namespace string) []*gutil.AccessSecret {
	return []*gutil.AccessSecret{
		gutil.NewShootAccessSecret(azure.CloudControllerManagerName, namespace),
		gutil.NewShootAccessSecret(azure.CSIControllerDiskName, namespace),
		gutil.NewShootAccessSecret(azure.CSIControllerFileName, namespace),
		gutil.NewShootAccessSecret(azure.CSIProvisionerName, namespace),
		gutil.NewShootAccessSecret(azure.CSIAttacherName, namespace),
		gutil.NewShootAccessSecret(azure.CSISnapshotterName, namespace),
		gutil.NewShootAccessSecret(azure.CSIResizerName, namespace),
		gutil.NewShootAccessSecret(azure.CSISnapshotControllerName, namespace),
		gutil.NewShootAccessSecret(azure.CSISnapshotValidationName, namespace),
		gutil.NewShootAccessSecret(azure.RemedyControllerName, namespace),
	}
}

var (
	configChart = &chart.Chart{
		Name:       "cloud-provider-config",
		EmbeddedFS: charts.InternalChart,
		Path:       filepath.Join(charts.InternalChartsPath, "cloud-provider-config"),
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
		Name:       "seed-controlplane",
		EmbeddedFS: charts.InternalChart,
		Path:       filepath.Join(charts.InternalChartsPath, "seed-controlplane"),
		SubCharts: []*chart.Chart{
			{
				Name:   azure.CloudControllerManagerName,
				Images: []string{azure.CloudControllerManagerImageName},
				Objects: []*chart.Object{
					{Type: &corev1.Service{}, Name: azure.CloudControllerManagerName},
					{Type: &appsv1.Deployment{}, Name: azure.CloudControllerManagerName},
					{Type: &corev1.ConfigMap{}, Name: azure.CloudControllerManagerName + "-observability-config"},
					{Type: &monitoringv1.ServiceMonitor{}, Name: "shoot-cloud-controller-manager"},
					{Type: &monitoringv1.PrometheusRule{}, Name: "shoot-cloud-controller-manager"},
					{Type: &autoscalingv1.VerticalPodAutoscaler{}, Name: azure.CloudControllerManagerName + "-vpa"},
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
					azure.CSISnapshotValidationWebhookImageName,
				},
				Objects: []*chart.Object{
					// csi-driver-controllers
					{Type: &appsv1.Deployment{}, Name: azure.CSIControllerDiskName},
					{Type: &appsv1.Deployment{}, Name: azure.CSIControllerFileName},
					{Type: &autoscalingv1.VerticalPodAutoscaler{}, Name: azure.CSIControllerDiskName + "-vpa"},
					{Type: &autoscalingv1.VerticalPodAutoscaler{}, Name: azure.CSIControllerFileName + "-vpa"},
					// csi-snapshot-controller
					{Type: &appsv1.Deployment{}, Name: azure.CSISnapshotControllerName},
					{Type: &autoscalingv1.VerticalPodAutoscaler{}, Name: azure.CSISnapshotControllerName + "-vpa"},
					// csi-snapshot-validation-webhook
					{Type: &appsv1.Deployment{}, Name: azure.CSISnapshotValidationName},
					{Type: &corev1.Service{}, Name: azure.CSISnapshotValidationName},
				},
			},
			{
				Name:   azure.RemedyControllerName,
				Images: []string{azure.RemedyControllerImageName},
				Objects: []*chart.Object{
					{Type: &appsv1.Deployment{}, Name: azure.RemedyControllerName},
					{Type: &corev1.ConfigMap{}, Name: azure.RemedyControllerName + "-config"},
					{Type: &autoscalingv1.VerticalPodAutoscaler{}, Name: azure.RemedyControllerName + "-vpa"},
					{Type: &rbacv1.Role{}, Name: azure.RemedyControllerName},
					{Type: &rbacv1.RoleBinding{}, Name: azure.RemedyControllerName},
					{Type: &corev1.ServiceAccount{}, Name: azure.RemedyControllerName},
				},
			},
		},
	}

	controlPlaneShootChart = &chart.Chart{
		Name:       "shoot-system-components",
		EmbeddedFS: charts.InternalChart,
		Path:       filepath.Join(charts.InternalChartsPath, "shoot-system-components"),
		SubCharts: []*chart.Chart{
			{
				Name: "allow-egress",
				Objects: []*chart.Object{
					{Type: &corev1.Service{}, Name: "allow-udp-egress"},
					{Type: &corev1.Service{}, Name: "allow-tcp-egress"},
				},
			},
			{
				Name: azure.CloudControllerManagerName,
				Images: []string{
					azure.CloudNodeManagerImageName,
				},
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
					{Type: &storagev1.CSIDriver{}, Name: "disk.csi.azure.com"},
					{Type: &storagev1.CSIDriver{}, Name: "file.csi.azure.com"},
					{Type: &rbacv1.ClusterRole{}, Name: azure.UsernamePrefix + azure.CSIDriverName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: azure.UsernamePrefix + azure.CSIDriverName},
					{Type: &rbacv1.ClusterRole{}, Name: azure.UsernamePrefix + azure.CSIControllerFileName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: azure.UsernamePrefix + azure.CSIControllerFileName},
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
					// csi-snapshot-validation-webhook
					{Type: &admissionregistrationv1.ValidatingWebhookConfiguration{}, Name: azure.CSISnapshotValidationName},
					{Type: &rbacv1.ClusterRole{}, Name: azure.UsernamePrefix + azure.CSISnapshotValidationName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: azure.UsernamePrefix + azure.CSISnapshotValidationName},
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
		Name:       "shoot-crds",
		EmbeddedFS: charts.InternalChart,
		Path:       filepath.Join(charts.InternalChartsPath, "shoot-crds"),
		SubCharts: []*chart.Chart{
			{
				Name: "volumesnapshots",
				Objects: []*chart.Object{
					{Type: &apiextensionsv1.CustomResourceDefinition{}, Name: "volumesnapshotclasses.snapshot.storage.k8s.io"},
					{Type: &apiextensionsv1.CustomResourceDefinition{}, Name: "volumesnapshotcontents.snapshot.storage.k8s.io"},
					{Type: &apiextensionsv1.CustomResourceDefinition{}, Name: "volumesnapshots.snapshot.storage.k8s.io"},
				},
			},
		},
	}

	storageClassChart = &chart.Chart{
		Name:       "shoot-storageclasses",
		EmbeddedFS: charts.InternalChart,
		Path:       filepath.Join(charts.InternalChartsPath, "shoot-storageclasses"),
	}
)

// NewValuesProvider creates a new ValuesProvider for the generic actuator.
func NewValuesProvider(mgr manager.Manager) genericactuator.ValuesProvider {
	return &valuesProvider{
		client:  mgr.GetClient(),
		decoder: serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
	}
}

// valuesProvider is a ValuesProvider that provides azure-specific values for the 2 charts applied by the generic actuator.
type valuesProvider struct {
	genericactuator.NoopValuesProvider
	client  k8sclient.Client
	decoder runtime.Decoder
}

// GetConfigChartValues returns the values for the config chart applied by the generic actuator.
func (vp *valuesProvider) GetConfigChartValues(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) (map[string]interface{}, error) {
	// Decode providerConfig
	cpConfig := &apisazure.ControlPlaneConfig{}
	if cp.Spec.ProviderConfig != nil {
		if _, _, err := vp.decoder.Decode(cp.Spec.ProviderConfig.Raw, nil, cpConfig); err != nil {
			return nil, fmt.Errorf("could not decode providerConfig of controlplane '%s': %w", k8sclient.ObjectKeyFromObject(cp), err)
		}
	}

	// Decode infrastructureProviderStatus
	var (
		infraStatus = &apisazure.InfrastructureStatus{}
		err         error
	)
	if cp.Spec.InfrastructureProviderStatus != nil {
		if infraStatus, err = azureapihelper.InfrastructureStatusFromRaw(cp.Spec.InfrastructureProviderStatus); err != nil {
			return nil, fmt.Errorf("could not decode infrastructureProviderStatus of controlplane '%s': %w", k8sclient.ObjectKeyFromObject(cp), err)
		}
	}

	// Get client auth
	auth, _, err := internal.GetClientAuthData(ctx, vp.client, cp.Spec.SecretRef, false)
	if err != nil {
		return nil, fmt.Errorf("could not get service account from secret '%s/%s': %w", cp.Spec.SecretRef.Namespace, cp.Spec.SecretRef.Name, err)
	}

	// Check if the configmap for the acr access need to be removed.
	if infraStatus.Identity == nil || !infraStatus.Identity.ACRAccess {
		if err := vp.removeAcrConfig(ctx, cp.Namespace); err != nil {
			return nil, fmt.Errorf("could not remove acr config map: %w", err)
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
	secretsReader secretsmanager.Reader,
	checksums map[string]string,
	scaledDown bool,
) (map[string]interface{}, error) {
	// Decode providerConfig
	cpConfig := &apisazure.ControlPlaneConfig{}
	if cp.Spec.ProviderConfig != nil {
		if _, _, err := vp.decoder.Decode(cp.Spec.ProviderConfig.Raw, nil, cpConfig); err != nil {
			return nil, fmt.Errorf("could not decode providerConfig of controlplane '%s': %w", k8sclient.ObjectKeyFromObject(cp), err)
		}
	}

	cpConfigSecret := &corev1.Secret{}
	if err := vp.client.Get(ctx, k8sclient.ObjectKey{Namespace: cp.Namespace, Name: azure.CloudProviderConfigName}, cpConfigSecret); err != nil {
		return nil, err
	}
	checksums[azure.CloudProviderConfigName] = utils.ComputeChecksum(cpConfigSecret.Data)

	// Decode infrastructureProviderStatus
	var (
		infraStatus = &apisazure.InfrastructureStatus{}
		err         error
	)
	if cp.Spec.InfrastructureProviderStatus != nil {
		if infraStatus, err = azureapihelper.InfrastructureStatusFromRaw(cp.Spec.InfrastructureProviderStatus); err != nil {
			return nil, fmt.Errorf("could not decode infrastructureProviderStatus of controlplane '%s': %w", k8sclient.ObjectKeyFromObject(cp), err)
		}
	}

	// TODO(rfranzke): Delete this in a future release.
	if err := kutil.DeleteObject(ctx, vp.client, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "csi-driver-controller-observability-config", Namespace: cp.Namespace}}); err != nil {
		return nil, fmt.Errorf("failed deleting legacy csi-driver-controller-observability-config ConfigMap: %w", err)
	}

	// TODO(rfranzke): Delete this after August 2024.
	gep19Monitoring := vp.client.Get(ctx, k8sclient.ObjectKey{Name: "prometheus-shoot", Namespace: cp.Namespace}, &appsv1.StatefulSet{}) == nil
	if gep19Monitoring {
		if err := kutil.DeleteObject(ctx, vp.client, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cloud-controller-manager-observability-config", Namespace: cp.Namespace}}); err != nil {
			return nil, fmt.Errorf("failed deleting cloud-controller-manager-observability-config ConfigMap: %w", err)
		}
		if err := kutil.DeleteObject(ctx, vp.client, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "remedy-controller-azure-monitoring-config", Namespace: cp.Namespace}}); err != nil {
			return nil, fmt.Errorf("failed deleting remedy-controller-azure-monitoring-config ConfigMap: %w", err)
		}
	}

	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: cp.Spec.SecretRef.Name, Namespace: cp.Spec.SecretRef.Namespace}}
	if err := vp.client.Get(ctx, k8sclient.ObjectKeyFromObject(secret), secret); err != nil {
		return nil, fmt.Errorf("failed getting controlplane secret: %w", err)
	}
	useWorkloadIdentity := false
	if secret.ObjectMeta.Labels != nil && secret.ObjectMeta.Labels[securityv1alpha1constants.LabelPurpose] == securityv1alpha1constants.LabelPurposeWorkloadIdentityTokenRequestor {
		useWorkloadIdentity = true
	}

	return getControlPlaneChartValues(cpConfig, cp, cluster, secretsReader, checksums, scaledDown, infraStatus, gep19Monitoring, useWorkloadIdentity)
}

// GetControlPlaneShootChartValues returns the values for the control plane shoot chart applied by the generic actuator.
func (vp *valuesProvider) GetControlPlaneShootChartValues(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	secretsReader secretsmanager.Reader,
	_ map[string]string,
) (map[string]interface{}, error) {
	return getControlPlaneShootChartValues(ctx, cp, cluster, secretsReader, vp.client)
}

// GetControlPlaneShootCRDsChartValues returns the values for the control plane shoot CRDs chart applied by the generic actuator.
func (vp *valuesProvider) GetControlPlaneShootCRDsChartValues(
	_ context.Context,
	_ *extensionsv1alpha1.ControlPlane,
	_ *extensionscontroller.Cluster,
) (map[string]interface{}, error) {
	return map[string]interface{}{
		"volumesnapshots": map[string]interface{}{
			"enabled": true,
		},
	}, nil
}

// GetStorageClassesChartValues returns the values for the storage classes chart applied by the generic actuator.
func (vp *valuesProvider) GetStorageClassesChartValues(
	_ context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	_ *extensionscontroller.Cluster,
) (map[string]interface{}, error) {
	// Decode providerConfig
	cpConfig := &apisazure.ControlPlaneConfig{}
	if cp.Spec.ProviderConfig != nil {
		if _, _, err := vp.decoder.Decode(cp.Spec.ProviderConfig.Raw, nil, cpConfig); err != nil {
			return nil, fmt.Errorf("could not decode providerConfig of controlplane '%s': %w", k8sclient.ObjectKeyFromObject(cp), err)
		}
	}

	values := map[string]interface{}{}
	if cpConfig.Storage != nil {
		values["managedDefaultStorageClass"] = ptr.Deref(cpConfig.Storage.ManagedDefaultStorageClass, true)
		values["managedDefaultVolumeSnapshotClass"] = ptr.Deref(cpConfig.Storage.ManagedDefaultVolumeSnapshotClass, true)
	}

	return values, nil
}

func (vp *valuesProvider) removeAcrConfig(ctx context.Context, namespace string) error {
	cm := corev1.ConfigMap{}
	cm.SetName(azure.CloudProviderAcrConfigName)
	cm.SetNamespace(namespace)
	return k8sclient.IgnoreNotFound(vp.client.Delete(ctx, &cm))
}

// getConfigChartValues collects and returns the configuration chart values.
func getConfigChartValues(infraStatus *apisazure.InfrastructureStatus, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster, ca *internal.ClientAuth) (map[string]interface{}, error) {
	subnetName, routeTableName, securityGroupName, err := getInfraNames(infraStatus)
	if err != nil {
		return nil, fmt.Errorf("could not determine subnet, availability set, route table or security group name from infrastructureStatus of controlplane '%s': %w", k8sclient.ObjectKeyFromObject(cp), err)
	}

	var maxNodes int32
	for _, worker := range cluster.Shoot.Spec.Provider.Workers {
		maxNodes = maxNodes + worker.Maximum
	}

	var useWorkloadIdentity = false
	if ca.TokenRetriever != nil {
		useWorkloadIdentity = true
	}

	// Collect config chart values.
	values := map[string]interface{}{
		"tenantId":            ca.TenantID,
		"subscriptionId":      ca.SubscriptionID,
		"aadClientId":         ca.ClientID,
		"aadClientSecret":     ca.ClientSecret,
		"useWorkloadIdentity": useWorkloadIdentity,
		"resourceGroup":       infraStatus.ResourceGroup.Name,
		"vnetName":            infraStatus.Networks.VNet.Name,
		"subnetName":          subnetName,
		"routeTableName":      routeTableName,
		"securityGroupName":   securityGroupName,
		"region":              cp.Spec.Region,
		"maxNodes":            maxNodes,
	}

	cloudConfiguration, err := azureclient.CloudConfiguration(nil, &cluster.Shoot.Spec.Region)
	if err != nil {
		return nil, err
	}

	values["cloud"] = cloudInstanceName(*cloudConfiguration)

	if infraStatus.Networks.VNet.ResourceGroup != nil {
		values["vnetResourceGroup"] = *infraStatus.Networks.VNet.ResourceGroup
	}

	if infraStatus.Identity != nil && infraStatus.Identity.ACRAccess {
		values["acrIdentityClientId"] = infraStatus.Identity.ClientID
	}

	return appendMachineSetValues(values, infraStatus), nil
}

func cloudInstanceName(cloudConfiguration apisazure.CloudConfiguration) string {
	switch {
	case cloudConfiguration.Name == apisazure.AzureChinaCloudName:
		return "AZURECHINACLOUD"
	case cloudConfiguration.Name == apisazure.AzureGovCloudName:
		return "AZUREUSGOVERNMENT"
	default:
		return "AZUREPUBLICCLOUD"
	}
}

func appendMachineSetValues(values map[string]interface{}, infraStatus *apisazure.InfrastructureStatus) map[string]interface{} {
	values["vmType"] = "standard"
	if azureapihelper.IsVmoRequired(infraStatus) {
		values["vmType"] = "vmss"
		return values
	}

	if primaryAvailabilitySet, err := azureapihelper.FindAvailabilitySetByPurpose(infraStatus.AvailabilitySets, apisazure.PurposeNodes); err == nil {
		values["availabilitySetName"] = primaryAvailabilitySet.Name
	}

	return values
}

// getInfraNames determines the subnet, availability set, route table and security group names from the given infrastructure status.
func getInfraNames(infraStatus *apisazure.InfrastructureStatus) (string, string, string, error) {
	_, nodesSubnet, err := azureapihelper.FindSubnetByPurposeAndZone(infraStatus.Networks.Subnets, apisazure.PurposeNodes, nil)
	if err != nil {
		return "", "", "", fmt.Errorf("could not determine subnet for purpose 'nodes': %w", err)
	}
	nodesRouteTable, err := azureapihelper.FindRouteTableByPurpose(infraStatus.RouteTables, apisazure.PurposeNodes)
	if err != nil {
		return "", "", "", fmt.Errorf("could not determine route table for purpose 'nodes': %w", err)
	}
	nodesSecurityGroup, err := azureapihelper.FindSecurityGroupByPurpose(infraStatus.SecurityGroups, apisazure.PurposeNodes)
	if err != nil {
		return "", "", "", fmt.Errorf("could not determine security group for purpose 'nodes': %w", err)
	}

	return nodesSubnet.Name, nodesRouteTable.Name, nodesSecurityGroup.Name, nil
}

// getControlPlaneChartValues collects and returns the control plane chart values.
func getControlPlaneChartValues(
	cpConfig *apisazure.ControlPlaneConfig,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	secretsReader secretsmanager.Reader,
	checksums map[string]string,
	scaledDown bool,
	infraStatus *apisazure.InfrastructureStatus,
	gep19Monitoring bool,
	useWorkloadIdentity bool,
) (
	map[string]interface{},
	error,
) {
	ccm, err := getCCMChartValues(cpConfig, cp, cluster, secretsReader, checksums, scaledDown, gep19Monitoring, useWorkloadIdentity)
	if err != nil {
		return nil, err
	}

	csi, err := getCSIControllerChartValues(cluster, secretsReader, scaledDown, infraStatus, checksums)
	if err != nil {
		return nil, err
	}

	remedy, err := getRemedyControllerChartValues(cluster, checksums, scaledDown, gep19Monitoring, useWorkloadIdentity)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"global": map[string]interface{}{
			"genericTokenKubeconfigSecretName": extensionscontroller.GenericTokenKubeconfigSecretNameFromCluster(cluster),
		},
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
	secretsReader secretsmanager.Reader,
	checksums map[string]string,
	scaledDown bool,
	gep19Monitoring bool,
	useWorkloadIdentity bool,
) (map[string]interface{}, error) {
	serverSecret, found := secretsReader.Get(cloudControllerManagerServerName)
	if !found {
		return nil, fmt.Errorf("secret %q not found", cloudControllerManagerServerName)
	}

	values := map[string]interface{}{
		"enabled":           true,
		"replicas":          extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
		"clusterName":       cp.Namespace,
		"kubernetesVersion": cluster.Shoot.Spec.Kubernetes.Version,
		"podNetwork":        strings.Join(extensionscontroller.GetPodNetwork(cluster), ","),
		"podAnnotations": map[string]interface{}{
			"checksum/secret-" + v1beta1constants.SecretNameCloudProvider: checksums[v1beta1constants.SecretNameCloudProvider],
			"checksum/secret-" + azure.CloudProviderConfigName:            checksums[azure.CloudProviderConfigName],
		},
		"podLabels": map[string]interface{}{
			v1beta1constants.LabelPodMaintenanceRestart: "true",
		},
		"tlsCipherSuites": kutil.TLSCipherSuites,
		"secrets": map[string]interface{}{
			"server": serverSecret.Name,
		},
		"gep19Monitoring":     gep19Monitoring,
		"useWorkloadIdentity": useWorkloadIdentity,
	}

	if cpConfig.CloudControllerManager != nil {
		values["featureGates"] = cpConfig.CloudControllerManager.FeatureGates
	}

	return values, nil
}

// getCSIControllerChartValues collects and returns the CSIController chart values.
func getCSIControllerChartValues(
	cluster *extensionscontroller.Cluster,
	secretsReader secretsmanager.Reader,
	scaledDown bool,
	infraStatus *apisazure.InfrastructureStatus,
	checksums map[string]string,
) (map[string]interface{}, error) {
	serverSecret, found := secretsReader.Get(csiSnapshotValidationServerName)
	if !found {
		return nil, fmt.Errorf("secret %q not found", csiSnapshotValidationServerName)
	}

	values := map[string]interface{}{
		"enabled": true,
		"podAnnotations": map[string]interface{}{
			"checksum/secret-" + azure.CloudProviderConfigName: checksums[azure.CloudProviderConfigName],
		},
		"replicas": extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
		"csiSnapshotController": map[string]interface{}{
			"replicas": extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
		},
		"csiSnapshotValidationWebhook": map[string]interface{}{
			"replicas": extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
			"secrets": map[string]interface{}{
				"server": serverSecret.Name,
			},
			"topologyAwareRoutingEnabled": gardencorev1beta1helper.IsTopologyAwareRoutingForShootControlPlaneEnabled(cluster.Seed, cluster.Shoot),
		},
	}

	k8sVersion, err := semver.NewVersion(cluster.Shoot.Spec.Kubernetes.Version)
	if err != nil {
		return nil, err
	}
	if versionutils.ConstraintK8sGreaterEqual131.Check(k8sVersion) {
		if _, ok := cluster.Shoot.Annotations[azure.AnnotationEnableVolumeAttributesClass]; ok {
			values["csiResizer"] = map[string]interface{}{
				"featureGates": map[string]string{
					"VolumeAttributesClass": "true",
				},
			}
			values["csiProvisioner"] = map[string]interface{}{
				"featureGates": map[string]string{
					"VolumeAttributesClass": "true",
				},
			}
		}
	}

	if azureapihelper.IsVmoRequired(infraStatus) {
		values["vmType"] = "vmss"
	} else {
		values["vmType"] = "standard"
	}

	return values, nil
}

// getRemedyControllerChartValues collects and returns the remedy controller chart values.
func getRemedyControllerChartValues(
	cluster *extensionscontroller.Cluster,
	checksums map[string]string,
	scaledDown bool,
	gep19Monitoring bool,
	useWorkloadIdentity bool,
) (map[string]interface{}, error) {
	disableRemedyController :=
		cluster.Shoot.Annotations[azure.DisableRemedyControllerAnnotation] == "true" ||
			features.ExtensionFeatureGate.Enabled(features.DisableRemedyController)

	if disableRemedyController {
		return map[string]interface{}{"enabled": true, "replicas": 0}, nil
	}
	return map[string]interface{}{
		"enabled":  true,
		"replicas": extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
		"podAnnotations": map[string]interface{}{
			"checksum/secret-" + azure.CloudProviderConfigName: checksums[azure.CloudProviderConfigName],
		},
		"gep19Monitoring":     gep19Monitoring,
		"useWorkloadIdentity": useWorkloadIdentity,
	}, nil
}

// getControlPlaneShootChartValues collects and returns the control plane shoot chart values.
func getControlPlaneShootChartValues(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	secretsReader secretsmanager.Reader,
	client k8sclient.Client,
) (
	map[string]interface{},
	error,
) {
	var (
		infraStatus                     = &apisazure.InfrastructureStatus{}
		cloudProviderDiskConfig         string
		cloudProviderDiskConfigChecksum string
		caBundle                        string
		err                             error
	)
	if cp.Spec.InfrastructureProviderStatus != nil {
		if infraStatus, err = azureapihelper.InfrastructureStatusFromRaw(cp.Spec.InfrastructureProviderStatus); err != nil {
			return nil, fmt.Errorf("could not decode infrastructureProviderStatus of controlplane '%s': %w", k8sclient.ObjectKeyFromObject(cp), err)
		}
	}

	secret := &corev1.Secret{}
	if err := client.Get(ctx, k8sclient.ObjectKey{Namespace: cp.Namespace, Name: azure.CloudProviderDiskConfigName}, secret); err != nil {
		return nil, err
	}

	cloudProviderDiskConfig = string(secret.Data[azure.CloudProviderConfigMapKey])
	cloudProviderDiskConfigChecksum = utils.ComputeChecksum(secret.Data)

	caSecret, found := secretsReader.Get(caNameControlPlane)
	if !found {
		return nil, fmt.Errorf("secret %q not found", caNameControlPlane)
	}
	caBundle = string(caSecret.Data[secretutils.DataKeyCertificateBundle])

	disableRemedyController := cluster.Shoot.Annotations[azure.DisableRemedyControllerAnnotation] == "true" ||
		features.ExtensionFeatureGate.Enabled(features.DisableRemedyController)

	return map[string]interface{}{
		// the allow-egress chart is enabled in all cases **except**:
		// - when the shoot is using AVSets due to using basic loadbalancers (see https://github.com/gardener/gardener-extension-provider-azure/issues/1).
		// - when the outbound connectivity is done via a NATGateway (currently meaning that all worker subnets have a NATGateway attached).
		azure.AllowEgressName: map[string]interface{}{
			"enabled": (infraStatus.Zoned || azureapihelper.IsVmoRequired(infraStatus)) && infraStatus.Networks.OutboundAccessType == apisazure.OutboundAccessTypeLoadBalancer,
		},
		azure.CloudControllerManagerName: map[string]interface{}{
			"enabled":    true,
			"vpaEnabled": gardencorev1beta1helper.ShootWantsVerticalPodAutoscaler(cluster.Shoot),
		},
		azure.CSINodeName: map[string]interface{}{
			"enabled":           true,
			"kubernetesVersion": cluster.Shoot.Spec.Kubernetes.Version,
			"podAnnotations": map[string]interface{}{
				"checksum/configmap-" + azure.CloudProviderDiskConfigName: cloudProviderDiskConfigChecksum,
			},
			"cloudProviderConfig": cloudProviderDiskConfig,
			"webhookConfig": map[string]interface{}{
				"url":      "https://" + azure.CSISnapshotValidationName + "." + cp.Namespace + "/volumesnapshot",
				"caBundle": caBundle,
			},
		},
		azure.RemedyControllerName: map[string]interface{}{
			"enabled": !disableRemedyController,
		},
	}, err
}
