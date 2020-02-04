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

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	azureapihelper "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	extensionscontroller "github.com/gardener/gardener-extensions/pkg/controller"
	"github.com/gardener/gardener-extensions/pkg/controller/controlplane"
	"github.com/gardener/gardener-extensions/pkg/controller/controlplane/genericactuator"
	"github.com/gardener/gardener-extensions/pkg/util"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/chart"
	"github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apiserver/pkg/authentication/user"
)

// Object names
const (
	cloudControllerManagerDeploymentName = "cloud-controller-manager"
	cloudControllerManagerServerName     = "cloud-controller-manager-server"
)

var controlPlaneSecrets = &secrets.Secrets{
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
					Name:         cloudControllerManagerDeploymentName,
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
					Name:       cloudControllerManagerServerName,
					CommonName: cloudControllerManagerDeploymentName,
					DNSNames:   controlplane.DNSNamesForService(cloudControllerManagerDeploymentName, clusterName),
					CertType:   secrets.ServerCert,
					SigningCA:  cas[v1beta1constants.SecretNameCACluster],
				},
			},
		}
	},
}

var configChart = &chart.Chart{
	Name: "cloud-provider-config",
	Path: filepath.Join(azure.InternalChartsPath, "cloud-provider-config"),
	Objects: []*chart.Object{
		{
			Type: &corev1.ConfigMap{},
			Name: azure.CloudProviderConfigName,
		},
		{
			Type: &corev1.ConfigMap{},
			Name: azure.CloudProviderKubeletConfigName,
		},
	},
}

var ccmChart = &chart.Chart{
	Name:   "cloud-controller-manager",
	Path:   filepath.Join(azure.InternalChartsPath, "cloud-controller-manager"),
	Images: []string{azure.CloudControllerManagerImageName},
	Objects: []*chart.Object{
		{Type: &corev1.Service{}, Name: "cloud-controller-manager"},
		{Type: &appsv1.Deployment{}, Name: "cloud-controller-manager"},
		{Type: &corev1.ConfigMap{}, Name: "cloud-controller-manager-monitoring-config"},
	},
}

var ccmShootChart = &chart.Chart{
	Name: "cloud-controller-manager-shoot",
	Path: filepath.Join(azure.InternalChartsPath, "controlplane-shoot", "cloud-controller-manager-shoot"),
	Objects: []*chart.Object{
		{Type: &rbacv1.ClusterRole{}, Name: "system:controller:cloud-node-controller"},
		{Type: &rbacv1.ClusterRoleBinding{}, Name: "system:controller:cloud-node-controller"},
	},
}

var allowUDPEgressChart = &chart.Chart{
	Name: "allow-udp-egress",
	Path: filepath.Join(azure.InternalChartsPath, "controlplane-shoot", "allow-udp-egress"),
}

var controlPlaneShootChart = &chart.Chart{
	Name:      "controlplane-shoot",
	Path:      filepath.Join(azure.InternalChartsPath, "controlplane-shoot"),
	SubCharts: []*chart.Chart{ccmShootChart, allowUDPEgressChart},
}

var storageClassChart = &chart.Chart{
	Name: "shoot-storageclasses",
	Path: filepath.Join(azure.InternalChartsPath, "shoot-storageclasses"),
}

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
func (vp *valuesProvider) GetConfigChartValues(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (map[string]interface{}, error) {
	// Decode providerConfig
	cpConfig := &apisazure.ControlPlaneConfig{}
	if cp.Spec.ProviderConfig != nil {
		if _, _, err := vp.Decoder().Decode(cp.Spec.ProviderConfig.Raw, nil, cpConfig); err != nil {
			return nil, errors.Wrapf(err, "could not decode providerConfig of controlplane '%s'", util.ObjectName(cp))
		}
	}

	// Decode infrastructureProviderStatus
	infraStatus := &apisazure.InfrastructureStatus{}
	if _, _, err := vp.Decoder().Decode(cp.Spec.InfrastructureProviderStatus.Raw, nil, infraStatus); err != nil {
		return nil, errors.Wrapf(err, "could not decode infrastructureProviderStatus of controlplane '%s'", util.ObjectName(cp))
	}

	// Get client auth
	auth, err := internal.GetClientAuthData(ctx, vp.Client(), cp.Spec.SecretRef)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get service account from secret '%s/%s'", cp.Spec.SecretRef.Namespace, cp.Spec.SecretRef.Name)
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
			return nil, errors.Wrapf(err, "could not decode providerConfig of controlplane '%s'", util.ObjectName(cp))
		}
	}

	// Get CCM chart values
	return getCCMChartValues(cpConfig, cp, cluster, checksums, scaledDown)
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
		return nil, errors.Wrapf(err, "could not decode infrastructureProviderStatus of controlplane '%s'", util.ObjectName(cp))
	}

	return getControlPlaneShootChartValues(infraStatus)
}

// getConfigChartValues collects and returns the configuration chart values.
func getConfigChartValues(
	infraStatus *apisazure.InfrastructureStatus,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	ca *internal.ClientAuth,
) (map[string]interface{}, error) {
	subnetName, routeTableName, securityGroupName, err := getInfraNames(infraStatus)
	if err != nil {
		return nil, errors.Wrapf(err, "could not determine subnet, availability set, route table or security group name from infrastructureStatus of controlplane '%s'", util.ObjectName(cp))
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

	// Add AvailabilitySet config if the cluster is not zoned.
	if !infraStatus.Zoned {
		nodesAvailabilitySet, err := azureapihelper.FindAvailabilitySetByPurpose(infraStatus.AvailabilitySets, apisazure.PurposeNodes)
		if err != nil {
			return nil, errors.Wrapf(err, "could not determine availability set for purpose 'nodes'")
		}
		values["availabilitySetName"] = nodesAvailabilitySet.Name
	}

	return values, nil
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
		"replicas":          extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
		"clusterName":       cp.Namespace,
		"kubernetesVersion": cluster.Shoot.Spec.Kubernetes.Version,
		"podNetwork":        extensionscontroller.GetPodNetwork(cluster),
		"podAnnotations": map[string]interface{}{
			"checksum/secret-cloud-controller-manager":        checksums[cloudControllerManagerDeploymentName],
			"checksum/secret-cloud-controller-manager-server": checksums[cloudControllerManagerServerName],
			"checksum/secret-cloudprovider":                   checksums[v1beta1constants.SecretNameCloudProvider],
			"checksum/configmap-cloud-provider-config":        checksums[azure.CloudProviderConfigName],
		},
	}

	if cpConfig.CloudControllerManager != nil {
		values["featureGates"] = cpConfig.CloudControllerManager.FeatureGates
	}

	return values, nil
}

// getControlPlaneShootChartValues collects and returns the control plane shoot chart values.
func getControlPlaneShootChartValues(
	infraStatus *apisazure.InfrastructureStatus,
) (map[string]interface{}, error) {
	values := map[string]interface{}{
		"allow-udp-egress": map[string]interface{}{
			"enabled": infraStatus.Zoned,
		},
	}

	return values, nil
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
