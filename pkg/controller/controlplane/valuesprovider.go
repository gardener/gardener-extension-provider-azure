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
	"path/filepath"
	"strings"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	extensionssecretsmanager "github.com/gardener/gardener/extensions/pkg/util/secret/manager"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/chart"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	autoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-azure/charts"
	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	azureapihelper "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
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
		EmbeddedFS: &charts.InternalChart,
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
		EmbeddedFS: &charts.InternalChart,
		Path:       filepath.Join(charts.InternalChartsPath, "seed-controlplane"),
		SubCharts: []*chart.Chart{
			{
				Name:   azure.CloudControllerManagerName,
				Images: []string{azure.CloudControllerManagerImageName},
				Objects: []*chart.Object{
					{Type: &corev1.Service{}, Name: azure.CloudControllerManagerName},
					{Type: &appsv1.Deployment{}, Name: azure.CloudControllerManagerName},
					{Type: &corev1.ConfigMap{}, Name: azure.CloudControllerManagerName + "-observability-config"},
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
					{Type: &corev1.ConfigMap{}, Name: azure.CSIControllerObservabilityConfigName},
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
		EmbeddedFS: &charts.InternalChart,
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
		EmbeddedFS: &charts.InternalChart,
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
		EmbeddedFS: &charts.InternalChart,
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
	client  client.Client
	decoder runtime.Decoder
}

// GetConfigChartValues returns the values for the config chart applied by the generic actuator.
func (vp *valuesProvider) GetConfigChartValues(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) (map[string]interface{}, error) {
	// Decode providerConfig
	cpConfig := &apisazure.ControlPlaneConfig{}
	if cp.Spec.ProviderConfig != nil {
		if _, _, err := vp.decoder.Decode(cp.Spec.ProviderConfig.Raw, nil, cpConfig); err != nil {
			return nil, fmt.Errorf("could not decode providerConfig of controlplane '%s': %w", kutil.ObjectName(cp), err)
		}
	}

	// Decode infrastructureProviderStatus
	var (
		infraStatus = &apisazure.InfrastructureStatus{}
		err         error
	)
	if cp.Spec.InfrastructureProviderStatus != nil {
		if infraStatus, err = azureapihelper.InfrastructureStatusFromRaw(cp.Spec.InfrastructureProviderStatus); err != nil {
			return nil, fmt.Errorf("could not decode infrastructureProviderStatus of controlplane '%s': %w", kutil.ObjectName(cp), err)
		}
	}

	// Get client auth
	auth, err := internal.GetClientAuthData(ctx, vp.client, cp.Spec.SecretRef, false)
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
			return nil, fmt.Errorf("could not decode providerConfig of controlplane '%s': %w", kutil.ObjectName(cp), err)
		}
	}

	cpConfigSecret := &corev1.Secret{}
	if err := vp.client.Get(ctx, kutil.Key(cp.Namespace, azure.CloudProviderConfigName), cpConfigSecret); err != nil {
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
			return nil, fmt.Errorf("could not decode infrastructureProviderStatus of controlplane '%s': %w", kutil.ObjectName(cp), err)
		}
	}

	// TODO(oliver-goetz): Delete this in a future release.
	if err := kutil.DeleteObject(ctx, vp.client, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-kube-apiserver-to-csi-snapshot-validation", Namespace: cp.Namespace}}); err != nil {
		return nil, fmt.Errorf("failed deleting legacy csi-snapshot-validation network policy: %w", err)
	}

	return getControlPlaneChartValues(cpConfig, cp, cluster, secretsReader, checksums, scaledDown, infraStatus)
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
			return nil, fmt.Errorf("could not decode providerConfig of controlplane '%s': %w", kutil.ObjectName(cp), err)
		}
	}

	values := map[string]interface{}{}
	if cpConfig.Storage != nil {
		values["managedDefaultStorageClass"] = pointer.BoolDeref(cpConfig.Storage.ManagedDefaultStorageClass, true)
		values["managedDefaultVolumeSnapshotClass"] = pointer.BoolDeref(cpConfig.Storage.ManagedDefaultVolumeSnapshotClass, true)
	}

	return values, nil
}

func (vp *valuesProvider) removeAcrConfig(ctx context.Context, namespace string) error {
	cm := corev1.ConfigMap{}
	cm.SetName(azure.CloudProviderAcrConfigName)
	cm.SetNamespace(namespace)
	return client.IgnoreNotFound(vp.client.Delete(ctx, &cm))
}

// getConfigChartValues collects and returns the configuration chart values.
func getConfigChartValues(infraStatus *apisazure.InfrastructureStatus, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster, ca *internal.ClientAuth) (map[string]interface{}, error) {
	subnetName, routeTableName, securityGroupName, err := getInfraNames(infraStatus)
	if err != nil {
		return nil, fmt.Errorf("could not determine subnet, availability set, route table or security group name from infrastructureStatus of controlplane '%s': %w", kutil.ObjectName(cp), err)
	}

	var maxNodes int32
	for _, worker := range cluster.Shoot.Spec.Provider.Workers {
		maxNodes = maxNodes + worker.Maximum
	}

	// Collect config chart values.
	values := map[string]interface{}{
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
) (
	map[string]interface{},
	error,
) {
	ccm, err := getCCMChartValues(cpConfig, cp, cluster, secretsReader, checksums, scaledDown)
	if err != nil {
		return nil, err
	}

	csi, err := getCSIControllerChartValues(cluster, secretsReader, scaledDown, infraStatus, checksums)
	if err != nil {
		return nil, err
	}

	remedy, err := getRemedyControllerChartValues(cluster, checksums, scaledDown)
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
		"podNetwork":        extensionscontroller.GetPodNetwork(cluster),
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
) (map[string]interface{}, error) {
	disableRemedyController := cluster.Shoot.Annotations[azure.DisableRemedyControllerAnnotation] == "true"
	if disableRemedyController {
		return map[string]interface{}{"enabled": true, "replicas": 0}, nil
	}
	return map[string]interface{}{
		"enabled":  true,
		"replicas": extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
		"podAnnotations": map[string]interface{}{
			"checksum/secret-" + azure.CloudProviderConfigName: checksums[azure.CloudProviderConfigName],
		},
	}, nil
}

// getControlPlaneShootChartValues collects and returns the control plane shoot chart values.
func getControlPlaneShootChartValues(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	secretsReader secretsmanager.Reader,
	client client.Client,
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
			return nil, fmt.Errorf("could not decode infrastructureProviderStatus of controlplane '%s': %w", kutil.ObjectName(cp), err)
		}
	}

	secret := &corev1.Secret{}
	if err := client.Get(ctx, kutil.Key(cp.Namespace, azure.CloudProviderDiskConfigName), secret); err != nil {
		return nil, err
	}

	cloudProviderDiskConfig = string(secret.Data[azure.CloudProviderConfigMapKey])
	cloudProviderDiskConfigChecksum = utils.ComputeChecksum(secret.Data)

	caSecret, found := secretsReader.Get(caNameControlPlane)
	if !found {
		return nil, fmt.Errorf("secret %q not found", caNameControlPlane)
	}
	caBundle = string(caSecret.Data[secretutils.DataKeyCertificateBundle])

	disableRemedyController := cluster.Shoot.Annotations[azure.DisableRemedyControllerAnnotation] == "true"
	pspDisabled := gardencorev1beta1helper.IsPSPDisabled(cluster.Shoot)

	return map[string]interface{}{
		"global": map[string]interface{}{
			"vpaEnabled": gardencorev1beta1helper.ShootWantsVerticalPodAutoscaler(cluster.Shoot),
		},
		azure.AllowEgressName: map[string]interface{}{"enabled": infraStatus.Zoned || azureapihelper.IsVmoRequired(infraStatus)},
		azure.CloudControllerManagerName: map[string]interface{}{
			"enabled":     true,
			"pspDisabled": pspDisabled,
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
			"pspDisabled": pspDisabled,
		},
		azure.RemedyControllerName: map[string]interface{}{
			"enabled": !disableRemedyController,
		},
	}, err
}
