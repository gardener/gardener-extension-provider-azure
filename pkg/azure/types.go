// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package azure

import (
	"path/filepath"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

const (
	// Name is the name of the Azure provider.
	Name = "provider-azure"

	// ShootVmoUsageAnnotation is an annotation assigned to the Shoot resource which indicates if VMO should be used.
	ShootVmoUsageAnnotation = "alpha.azure.provider.extensions.gardener.cloud/vmo"

	// NetworkLayoutZoneMigrationAnnotation is used when migrating from a single subnet network layout to a multiple subnet network layout to indicate the zone that the existing subnet should be assigned to.
	NetworkLayoutZoneMigrationAnnotation = "migration.azure.provider.extensions.gardener.cloud/zone"

	// CloudControllerManagerImageName is the name of the cloud-controller-manager image.
	CloudControllerManagerImageName = "cloud-controller-manager"
	// CloudNodeManagerImageName is the name of the cloud-node-manager image.
	CloudNodeManagerImageName = "cloud-node-manager"
	// CSIDriverDiskImageName is the name of the csi-driver-disk image.
	CSIDriverDiskImageName = "csi-driver-disk"
	// CSIDriverFileImageName is the name of the csi-driver-file image.
	CSIDriverFileImageName = "csi-driver-file"
	// CSIProvisionerImageName is the name of the csi-provisioner image.
	CSIProvisionerImageName = "csi-provisioner"
	// CSIAttacherImageName is the name of the csi-attacher image.
	CSIAttacherImageName = "csi-attacher"
	// CSISnapshotterImageName is the name of the csi-snapshotter image.
	CSISnapshotterImageName = "csi-snapshotter"
	// CSISnapshotControllerImageName is the name of the csi-snapshot-controller image.
	CSISnapshotControllerImageName = "csi-snapshot-controller"
	// CSIResizerImageName is the name of the csi-resizer image.
	CSIResizerImageName = "csi-resizer"
	// CSINodeDriverRegistrarImageName is the name of the csi-node-driver-registrar image.
	CSINodeDriverRegistrarImageName = "csi-node-driver-registrar"
	// CSILivenessProbeImageName is the name of the csi-liveness-probe image.
	CSILivenessProbeImageName = "csi-liveness-probe"
	// CSISnapshotValidationWebhookImageName is the name of the csi-snapshot-validation-webhook image.
	CSISnapshotValidationWebhookImageName = "csi-snapshot-validation-webhook"
	// MachineControllerManagerImageName is the name of the MachineControllerManager image.
	MachineControllerManagerImageName = "machine-controller-manager"
	// MachineControllerManagerProviderAzureImageName is the name of the MachineController Azure image.
	MachineControllerManagerProviderAzureImageName = "machine-controller-manager-provider-azure"
	// TerraformerImageName is the name of the Terraformer image.
	TerraformerImageName = "terraformer"
	// RemedyControllerImageName is the name of the remedy-controller image.
	RemedyControllerImageName = "remedy-controller-azure"

	// SubscriptionIDKey is the key for the subscription ID.
	SubscriptionIDKey = "subscriptionID"
	// TenantIDKey is the key for the tenant ID.
	TenantIDKey = "tenantID"
	// ClientIDKey is the key for the client ID.
	ClientIDKey = "clientID"
	// ClientSecretKey is the key for the client secret.
	ClientSecretKey = "clientSecret"

	// DNSSubscriptionIDKey is the key for the subscription ID in DNS secrets.
	DNSSubscriptionIDKey = "AZURE_SUBSCRIPTION_ID"
	// DNSTenantIDKey is the key for the tenant ID in DNS secrets.
	DNSTenantIDKey = "AZURE_TENANT_ID"
	// DNSClientIDKey is the key for the client ID in DNS secrets.
	DNSClientIDKey = "AZURE_CLIENT_ID"
	// DNSClientSecretKey is the key for the client secret in DNS secrets.
	DNSClientSecretKey = "AZURE_CLIENT_SECRET"

	// StorageAccount is a constant for the key in a cloud provider secret and backup secret that holds the Azure account name.
	StorageAccount = "storageAccount"
	// StorageKey is a constant for the key in a cloud provider secret and backup secret that holds the Azure secret storage access key.
	StorageKey = "storageKey"

	// AzureBlobStorageHostName is the host name for azure blob storage service.
	AzureBlobStorageHostName = "blob.core.windows.net"

	// MachineSetTagKey is the name of the infrastructure resource tag for machine sets.
	MachineSetTagKey = "machineset.azure.extensions.gardener.cloud"

	// AllowEgressName is the name of the service for allowing egress traffic.
	AllowEgressName = "allow-egress"
	// CloudProviderConfigName is the name of the secret containing the cloud provider config.
	CloudProviderConfigName = "cloud-provider-config"
	// CloudProviderDiskConfigName is the name of the secret containing the cloud provider config for disk/volume handling.
	CloudProviderDiskConfigName = "cloud-provider-disk-config"
	// CloudProviderConfigMapKey is the key storing the cloud provider config as value in the cloud provider configmap.
	CloudProviderConfigMapKey = "cloudprovider.conf"
	// CloudProviderAcrConfigName is the name of the configmap containing the cloud provider config to configure the kubelet to get acr config.
	CloudProviderAcrConfigName = "kubelet-acr-config"
	// CloudProviderAcrConfigMapKey is the key storing the cloud provider config as value in the acr cloud provider configmap.
	CloudProviderAcrConfigMapKey = "acr.conf"
	// CloudControllerManagerName is a constant for the name of the CloudController deployed by the worker controller.
	CloudControllerManagerName = "cloud-controller-manager"
	// CSIControllerName is a constant for the chart name for a CSI controller deployment in the seed.
	CSIControllerName = "csi-driver-controller"
	// CSIControllerDiskName is a constant for the name of the Disk CSI controller deployment in the seed.
	CSIControllerDiskName = "csi-driver-controller-disk"
	// CSIControllerFileName is a constant for the name of the File CSI controller deployment in the seed.
	CSIControllerFileName = "csi-driver-controller-file"
	// CSIControllerObservabilityConfigName is a constant for the name of the ConfigMap containing monitoring and logging stack configurations for csi-driver.
	CSIControllerObservabilityConfigName = "csi-driver-controller-observability-config"
	// CSINodeName is a constant for the chart name for a CSI node deployment in the shoot.
	CSINodeName = "csi-driver-node"
	// CSINodeDiskName is a constant for the name of the Disk CSI node deployment in the shoot.
	CSINodeDiskName = "csi-driver-node-disk"
	// CSINodeFileName is a constant for the name of the File CSI node deployment in the shoot.
	CSINodeFileName = "csi-driver-node-file"
	// CSIDriverName is a constant for the name of the csi-driver component.
	CSIDriverName = "csi-driver"
	// CSIProvisionerName is a constant for the name of the csi-provisioner component.
	CSIProvisionerName = "csi-provisioner"
	// CSIAttacherName is a constant for the name of the csi-attacher component.
	CSIAttacherName = "csi-attacher"
	// CSISnapshotterName is a constant for the name of the csi-snapshotter component.
	CSISnapshotterName = "csi-snapshotter"
	// CSISnapshotControllerName is a constant for the name of the csi-snapshot-controller component.
	CSISnapshotControllerName = "csi-snapshot-controller"
	// CSIResizerName is a constant for the name of the csi-resizer component.
	CSIResizerName = "csi-resizer"
	// CSINodeDriverRegistrarName is a constant for the name of the csi-node-driver-registrar component.
	CSINodeDriverRegistrarName = "csi-node-driver-registrar"
	// CSILivenessProbeName is a constant for the name of the csi-liveness-probe component.
	CSILivenessProbeName = "csi-liveness-probe"
	// CSISnapshotValidation is the constant for the name of the csi-snapshot-validation-webhook component.
	CSISnapshotValidation = "csi-snapshot-validation"
	// MachineControllerManagerName is a constant for the name of the machine-controller-manager.
	MachineControllerManagerName = "machine-controller-manager"
	// MachineControllerManagerVpaName is the name of the VerticalPodAutoscaler of the machine-controller-manager deployment.
	MachineControllerManagerVpaName = "machine-controller-manager-vpa"
	// MachineControllerManagerMonitoringConfigName is the name of the ConfigMap containing monitoring stack configurations for machine-controller-manager.
	MachineControllerManagerMonitoringConfigName = "machine-controller-manager-monitoring-config"
	// RemedyControllerName is a constant for the name of the remedy-controller.
	RemedyControllerName = "remedy-controller-azure"
	// DisableRemedyControllerAnnotation disables the Azure remedy controller (enabled by default)
	DisableRemedyControllerAnnotation = "azure.provider.extensions.gardener.cloud/disable-remedy-controller"
	// ExtensionPurposeLabel is a label to define the purpose of a resource for the extension.
	ExtensionPurposeLabel = "azure.provider.extensions.gardener.cloud/purpose"
	// ExtensionPurposeServicePrincipalSecret is the label value for a Secret resource
	// that hold service principal information to a corresponding AD tenant.
	ExtensionPurposeServicePrincipalSecret = "tenant-service-principal-secret"

	// CSIMigrationKubernetesVersion is a constant for the Kubernetes version for which the Shoot's CSI migration will be
	// performed.
	CSIMigrationKubernetesVersion = "1.21"
)

var (
	// ChartsPath is the path to the charts
	ChartsPath = filepath.Join("charts")
	// InternalChartsPath is the path to the internal charts
	InternalChartsPath = filepath.Join(ChartsPath, "internal")

	// UsernamePrefix is a constant for the username prefix of components deployed by Azure.
	UsernamePrefix = extensionsv1alpha1.SchemeGroupVersion.Group + ":" + Name + ":"
)
