# Using the Azure provider extension with Gardener as end-user

The [`core.gardener.cloud/v1beta1.Shoot` resource](https://github.com/gardener/gardener/blob/master/example/90-shoot.yaml) declares a few fields that are meant to contain provider-specific configuration.

This document describes the configurable options for Azure and provides an example `Shoot` manifest with minimal configuration that can be used to create an Azure cluster (modulo the landscape-specific information like cloud profile names, secret binding names, etc.).

## Azure Provider Credentials

In order for Gardener to create a Kubernetes cluster using Azure infrastructure components, a Shoot has to provide credentials with sufficient permissions to the desired Azure subscription.
Every shoot cluster references a `SecretBinding` which itself references a `Secret`, and this `Secret` contains the provider credentials of the Azure subscription.
The `SecretBinding` is configurable in the [Shoot cluster](https://github.com/gardener/gardener/blob/master/example/90-shoot.yaml) with the field `secretBindingName`.

Create an [Azure Application and Service Principle](https://docs.microsoft.com/en-us/azure/active-directory/develop/howto-create-service-principal-portal) and obtain its credentials.
Please make sure the Azure application has the following IAM roles. 
- [Contributor](https://docs.microsoft.com/en-us/azure/role-based-access-control/built-in-roles#contributor)

The example below demonstrates how the secret containing the client credentials of the Azure Application has to look like:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: core-azure
  namespace: garden-dev
type: Opaque
data:
  clientID: base64(client-id)
  clientSecret: base64(client-secret)
  subscriptionID: base64(subscription-id)
  tenantID: base64(tenant-id)
```

⚠️ Depending on your API usage it can be problematic to reuse the same Service Principal for different Shoot clusters due to rate limits.
Please consider spreading your Shoots over Service Principals from different Azure subscriptions if you are hitting those limits.

## `InfrastructureConfig`

The infrastructure configuration mainly describes how the network layout looks like in order to create the shoot worker nodes in a later step, thus, prepares everything relevant to create VMs, load balancers, volumes, etc.

An example `InfrastructureConfig` for the Azure extension looks as follows:

```yaml
apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
kind: InfrastructureConfig
networks:
  vnet: # specify either 'name' and 'resourceGroup' or 'cidr'
    # name: my-vnet
    # resourceGroup: my-vnet-resource-group
    cidr: 10.250.0.0/16
  workers: 10.250.0.0/19
  # natGateway:
  #   enabled: false
  #   idleConnectionTimeoutMinutes: 4
  #   zone: 1
  #   ipAddresses:
  #   - name: my-public-ip-name
  #     resourceGroup: my-public-ip-resource-group
  #     zone: 1
  # serviceEndpoints:
  # - Microsoft.Test
zoned: false
# resourceGroup:
#   name: mygroup
#identity:
#  name: my-identity-name
#  resourceGroup: my-identity-resource-group
#  acrAccess: true
```

Currently, it's not yet possible to deploy into existing resource groups, but in the future it will.
The `.resourceGroup.name` field will allow specifying the name of an already existing resource group that the shoot cluster and all infrastructure resources will be deployed to.

Via the `.zoned` boolean you can tell whether you want to use Azure availability zones or not.
If you don't use zones then an availability set will be created and only basic load balancers will be used.
Zoned clusters use standard load balancers.

The `networks.vnet` section describes whether you want to create the shoot cluster in an already existing VNet or whether to create a new one:

* If `networks.vnet.name` and `networks.vnet.resourceGroup` are given then you have to specify the VNet name and VNet resource group name of the existing VNet that was created by other means (manually, other tooling, ...).
* If `networks.vnet.cidr` is given then you have to specify the VNet CIDR of a new VNet that will be created during shoot creation.
You can freely choose a private CIDR range.
* Either `networks.vnet.name` and `neworks.vnet.resourceGroup` or `networks.vnet.cidr` must be present, but not both at the same time.

The `networks.workers` section describes the CIDR for a subnet that is used for all shoot worker nodes, i.e., VMs which later run your applications.
The specified CIDR range must be contained in the VNet CIDR specified above, or the VNet CIDR of your already existing VNet.
You can freely choose this CIDR and it is your responsibility to properly design the network layout to suit your needs.

In the `networks.serviceEndpoints[]` list you can specify the list of Azure service endpoints which shall be associated with the worker subnet. All available service endpoints and their technical names can be found in the (Azure Service Endpoint documentation](https://docs.microsoft.com/en-us/azure/virtual-network/virtual-network-service-endpoints-overview).

The `networks.natGateway` section contains configuration for the Azure NatGateway which can be attached to the worker subnet of a Shoot cluster. Here are some key information about the usage of the NatGateway for a Shoot cluster:
- NatGateway usage is optional and can be enabled or disabled via `.networks.natGateway.enabled`.
- If the NatGateway is not used then the egress connections initiated within the Shoot cluster will be nated via the LoadBalancer of the clusters (default Azure behaviour, see [here](https://docs.microsoft.com/en-us/azure/load-balancer/load-balancer-outbound-connections#scenarios)).
- NatGateway is only available for zonal clusters `.zoned=true`.
- The NatGateway is currently **not** zone redundantly deployed. That mean the NatGateway of a Shoot cluster will always be in just one zone. This zone can be optionally selected via `.networks.natGateway.zone`.
- **Caution:** Modifying the `.networks.natGateway.zone` setting requires a recreation of the NatGateway and the managed public ip (automatically used if no own public ip is specified, see below). That mean you will most likely get a different public ip for egress connections.
- It is possible to bring own zonal public ip(s) via `networks.natGateway.ipAddresses`. Those public ip(s) need to be in the same zone as the NatGateway (see `networks.natGateway.zone`) and be of SKU `standard`. For each public ip the `name`, the `resourceGroup` and the `zone` need to be specified.
- The field `networks.natGateway.idleConnectionTimeoutMinutes` allows the configuration of NAT Gateway's idle connection timeout property. The idle timeout value can be adjusted from 4 minutes, up to 120 minutes. Omitting this property will set the idle timeout to its default value according to [NAT Gateway's documentation](https://docs.microsoft.com/en-us/azure/virtual-network/nat-gateway-resource#timers).

In the `identity` section you can specify an [Azure user-assigned managed identity](https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/overview#how-does-the-managed-identities-for-azure-resources-work) which should be attached to all cluster worker machines. With `identity.name` you can specify the name of the identity and with `identity.resourceGroup` you can specify the resource group which contains the identity resource on Azure. The identity need to be created by the user upfront (manually, other tooling, ...). Gardener/Azure Extension will only use the referenced one and won't create an identity. Furthermore the identity have to be in the same subscription as the Shoot cluster. Via the `identity.acrAccess` you can configure the worker machines to use the passed identity for pulling from an [Azure Container Registry (ACR)](https://docs.microsoft.com/en-us/azure/container-registry/container-registry-intro).
**Caution:** Adding, exchanging or removing the identity will require a rolling update of all worker machines in the Shoot cluster.

Apart from the VNet and the worker subnet the Azure extension will also create a dedicated resource group, route tables, security groups, and an availability set (if not using zoned clusters).

## `ControlPlaneConfig`

The control plane configuration mainly contains values for the Azure-specific control plane components.
Today, the only component deployed by the Azure extension is the `cloud-controller-manager`.

An example `ControlPlaneConfig` for the Azure extension looks as follows:

```yaml
apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
kind: ControlPlaneConfig
cloudControllerManager:
  featureGates:
    CustomResourceValidation: true
```

The `cloudControllerManager.featureGates` contains a map of explicitly enabled or disabled feature gates.
For production usage it's not recommend to use this field at all as you can enable alpha features or disable beta/stable features, potentially impacting the cluster stability.
If you don't want to configure anything for the `cloudControllerManager` simply omit the key in the YAML specification.

## `WorkerConfig`

The Azure extension does not support a specific `WorkerConfig` yet, however, it supports encryption for volumes plus support for additional data volumes per machine.
Please note that you cannot specify the `encrypted` flag for Azure disks as they are encrypted by default/out-of-the-box.
For each data volume, you have to specify a name.
The following YAML is a snippet of a `Shoot` resource:

```yaml
spec:
  provider:
    workers:
    - name: cpu-worker
      ...
      volume:
        type: Standard_LRS
        size: 20Gi
      dataVolumes:
      - name: kubelet-dir
        type: Standard_LRS
        size: 25Gi
```

## Example `Shoot` manifest (non-zoned)

Please find below an example `Shoot` manifest for a non-zoned cluster:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: johndoe-azure
  namespace: garden-dev
spec:
  cloudProfileName: azure
  region: westeurope
  secretBindingName: core-azure
  provider:
    type: azure
    infrastructureConfig:
      apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
      kind: InfrastructureConfig
      networks:
        vnet:
          cidr: 10.250.0.0/16
        workers: 10.250.0.0/19
      zoned: false
    controlPlaneConfig:
      apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
      kind: ControlPlaneConfig
    workers:
    - name: worker-xoluy
      machine:
        type: Standard_D4_v3
      minimum: 2
      maximum: 2
      volume:
        size: 50Gi
        type: Standard_LRS
  networking:
    type: calico
    pods: 100.96.0.0/11
    nodes: 10.250.0.0/16
    services: 100.64.0.0/13
  kubernetes:
    version: 1.16.1
  maintenance:
    autoUpdate:
      kubernetesVersion: true
      machineImageVersion: true
  addons:
    kubernetesDashboard:
      enabled: true
    nginxIngress:
      enabled: true
```

## Example `Shoot` manifest (zoned)

Please find below an example `Shoot` manifest for a zoned cluster:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
metadata:
  name: johndoe-azure
  namespace: garden-dev
spec:
  cloudProfileName: azure
  region: westeurope
  secretBindingName: core-azure
  provider:
    type: azure
    infrastructureConfig:
      apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
      kind: InfrastructureConfig
      networks:
        vnet:
          cidr: 10.250.0.0/16
        workers: 10.250.0.0/19
      zoned: true
    controlPlaneConfig:
      apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
      kind: ControlPlaneConfig
    workers:
    - name: worker-xoluy
      machine:
        type: Standard_D4_v3
      minimum: 2
      maximum: 2
      volume:
        size: 50Gi
        type: Standard_LRS
      zones:
      - "1"
      - "2"
  networking:
    type: calico
    pods: 100.96.0.0/11
    nodes: 10.250.0.0/16
    services: 100.64.0.0/13
  kubernetes:
    version: 1.16.1
  maintenance:
    autoUpdate:
      kubernetesVersion: true
      machineImageVersion: true
  addons:
    kubernetesDashboard:
      enabled: true
    nginxIngress:
      enabled: true
```

## CSI volume provisioners

Every Azure shoot cluster that has at least Kubernetes v1.21 will be deployed with the Azure Disk CSI driver and the Azure File CSI driver.
Both are compatible with the legacy in-tree volume provisioners that were deprecated by the Kubernetes community and will be removed in future versions of Kubernetes.
End-users might want to update their custom `StorageClass`es to the new `disk.csi.azure.com` or `file.csi.azure.com` provisioner, respectively.
Shoot clusters with Kubernetes v1.20 or less will use the in-tree `kubernetes.io/azure-disk` and `kubernetes.io/azure-file` volume provisioners in the kube-controller-manager and the kubelet.

## Miscellaneous

### Azure Accelerated Networking

All worker machines of the cluster will be automatically configured to use [Azure Accelerated Networking](https://docs.microsoft.com/en-us/azure/virtual-network/create-vm-accelerated-networking-cli) if the prerequisites are fulfilled.
The prerequisites are that the cluster must be zoned, and the used machine type and operating system image version are compatible for Accelerated Networking.
`Availability Set` based shoot clusters will not be enabled for accelerated networking even if the machine type and operating system support it, this is necessary because all machines from the availability set must be scheduled on special hardware, more daitls can be found [here](https://github.com/MicrosoftDocs/azure-docs/issues/10536).
Supported machine types are listed in the CloudProfile in `.spec.providerConfig.machineTypes[].acceleratedNetworking` and the supported operating system image versions are defined in `.spec.providerConfig.machineImages[].versions[].acceleratedNetworking`.

### Preview: Shoot clusters with VMSS Orchestration Mode VM (VMO)

Azure Shoot clusters can be created with machines which are attached to an [Azure Virtual Machine ScaleSet orchestraion mode VM (VMO)](https://docs.microsoft.com/en-us/azure/virtual-machine-scale-sets/orchestration-modes).
The orchestraion mode VM of Virtual Machine ScaleSet is currently in preview mode and not yet general available on Azure.

Azure VMO are intended to replace Azure AvailabilitySet for non-zoned Azure Shoot clusters in the mid-term as VMO come with less disadvantages like no blocking machine operations or compability with `Standard` SKU loadbalancer etc.

To configure an Azure Shoot cluster which make use of VMO you need to do the following:
- The `InfrastructureConfig` of the Shoot configuration need to contain `.zoned=false`
- Shoot resource need to have the following annotation assigned: `alpha.azure.provider.extensions.gardener.cloud/vmo=true`

Some key facts about VMO based clusters:
- Unlike regular non-zonal Azure Shoot clusters, which have a primary AvailabilitySet which is shared between all machines in all worker pools of a Shoot cluster, a VMO based cluster has an own VMO for each workerpool
- In case the configuration of the VMO will change (e.g. amount of fault domains in a region change; configured in the CloudProfile) all machines of the worker pool need to be rolled
- It is not possible to migrate an existing primary AvailabilitySet based Shoot cluster to VMO based Shoot cluster and vice versa
- VMO based clusters are using `Standard` SKU LoadBalancers instead of `Basic` SKU LoadBalancers for AvailabilitySet based Shoot clusters
