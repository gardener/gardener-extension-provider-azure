# Using the Azure provider extension with Gardener as an operator

The [`core.gardener.cloud/v1beta1.CloudProfile` resource](https://github.com/gardener/gardener/blob/master/example/30-cloudprofile.yaml) declares a `providerConfig` field that is meant to contain provider-specific configuration.
The [`core.gardener.cloud/v1beta1.Seed` resource](https://github.com/gardener/gardener/blob/master/example/50-seed.yaml) is structured similarly.
Additionally, it allows configuring settings for the backups of the main etcds' data of shoot clusters control planes running in this seed cluster.

This document explains the necessary configuration for the Azure provider extension.

## `CloudProfile` resource

This section describes, how the configuration for `CloudProfile`s looks like for Azure by providing an example `CloudProfile` manifest with minimal configuration that can be used to allow the creation of Azure shoot clusters.

### `CloudProfileConfig`

The cloud profile configuration contains information about the real machine image IDs in the Azure environment (image `urn`, `id`, `communityGalleryImageID` or `sharedGalleryImageID`).
You have to map every version that you specify in `.spec.machineImages[].versions` to an available VM image in your subscription.
The VM image can be either from the [Azure Marketplace](https://azuremarketplace.microsoft.com/en-us/marketplace/apps?filters=virtual-machine-images) and will then get identified via a `urn`, it can be a custom VM image from a shared image gallery and is then identified  `sharedGalleryImageID`, or it can be from a community image gallery and is then identified by its `communityGalleryImageID`. You can use `id` field also to specifiy the image location in the azure compute gallery (in which case it would have a different kind of path) but it is not recommended as it sometimes faces problems in cross subscription image sharing.
For each machine image version an `architecture` field can be specified which specifies the CPU architecture of the machine on which given machine image can be used.

An example `CloudProfileConfig` for the Azure extension looks as follows:

```yaml
apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
kind: CloudProfileConfig
countUpdateDomains:
- region: westeurope
  count: 5
countFaultDomains:
- region: westeurope
  count: 3
machineTypes:
- name: Standard_D3_v2
  acceleratedNetworking: true
- name: Standard_X
machineImages:
- name: coreos
  versions:
  - version: 2135.6.0
    urn: "CoreOS:CoreOS:Stable:2135.6.0"
    # architecture: amd64 # optional
    acceleratedNetworking: true
- name: myimage
  versions:
  - version: 1.0.0
    id: "/subscriptions/<subscription ID where the gallery is located>/resourceGroups/myGalleryRG/providers/Microsoft.Compute/galleries/myGallery/images/myImageDefinition/versions/1.0.0"
- name: GardenLinuxCommunityImage
  versions:
  - version: 1.0.0
    communityGalleryImageID: "/CommunityGalleries/gardenlinux-567905d8-921f-4a85-b423-1fbf4e249d90/Images/gardenlinux/Versions/576.1.1"
- name: SharedGalleryImageName
  versions:
    - version: 1.0.0
      sharedGalleryImageID: "/SharedGalleries/sharedGalleryName/Images/sharedGalleryImageName/Versions/sharedGalleryImageVersionName"
```

The cloud profile configuration contains information about the update via `.countUpdateDomains[]` and failure domain via `.countFaultDomains[]` counts in the Azure regions you want to offer.

The `.machineTypes[]` list contain provider specific information to the machine types e.g. if the machine type support [Azure Accelerated Networking](https://docs.microsoft.com/en-us/azure/virtual-network/create-vm-accelerated-networking-cli), see `.machineTypes[].acceleratedNetworking`.

Additionally, it contains the real machine image identifiers in the Azure environment. You can provide either URN for Azure Market Place images or id of [Shared Image Gallery](https://docs.microsoft.com/en-us/azure/virtual-machines/linux/shared-image-galleries) images.
When Shared Image Gallery is used, you have to ensure that the image is available in the desired regions and the end-user subscriptions have access to the image or to the whole gallery.
You have to map every version that you specify in `.spec.machineImages[].versions` here such that the Azure extension knows the machine image identifiers for every version you want to offer.
Furthermore, you can specify for each image version via `.machineImages[].versions[].acceleratedNetworking` if Azure Accelerated Networking is supported.

### Example `CloudProfile` manifest

The possible values for `.spec.volumeTypes[].name` on Azure are `Standard_LRS`, `StandardSSD_LRS` and `Premium_LRS`. There is another volume type called `UltraSSD_LRS` but this type is not supported to use as os disk. If an end user select a volume type whose name is not equal to one of the valid values then the machine will be created with the default volume type which belong to the selected machine type. Therefore it is recommended to configure only the valid values for the `.spec.volumeType[].name` in the `CloudProfile`.

Please find below an example `CloudProfile` manifest:

```yaml
apiVersion: core.gardener.cloud/v1beta1
kind: CloudProfile
metadata:
  name: azure
spec:
  type: azure
  kubernetes:
    versions:
    - version: 1.28.2
    - version: 1.23.8
      expirationDate: "2022-10-31T23:59:59Z"
  machineImages:
  - name: coreos
    versions:
    - version: 2135.6.0
  machineTypes:
  - name: Standard_D3_v2
    cpu: "4"
    gpu: "0"
    memory: 14Gi
  - name: Standard_D4_v3
    cpu: "4"
    gpu: "0"
    memory: 16Gi
  volumeTypes:
  - name: Standard_LRS
    class: standard
    usable: true
  - name: StandardSSD_LRS
    class: premium
    usable: false
  - name: Premium_LRS
    class: premium
    usable: false
  regions:
  - name: westeurope
  providerConfig:
    apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
    kind: CloudProfileConfig
    machineTypes:
    - name: Standard_D3_v2
      acceleratedNetworking: true
    - name: Standard_D4_v3
    countUpdateDomains:
    - region: westeurope
      count: 5
    countFaultDomains:
    - region: westeurope
      count: 3
    machineImages:
    - name: coreos
      versions:
      - version: 2303.3.0
        urn: CoreOS:CoreOS:Stable:2303.3.0
        # architecture: amd64 # optional
        acceleratedNetworking: true
      - version: 2135.6.0
        urn: "CoreOS:CoreOS:Stable:2135.6.0"
        # architecture: amd64 # optional
```

## `Seed` resource

This provider extension does not support any provider configuration for the `Seed`'s `.spec.provider.providerConfig` field.
However, it supports managing of backup infrastructure, i.e., you can specify a configuration for the `.spec.backup` field.

### Backup configuration

A Seed of type `azure` can be configured to perform backups for the main etcds' of the shoot clusters control planes using Azure Blob storage.

The location/region where the backups will be stored defaults to the region of the Seed (`spec.provider.region`), but can also be explicitly configured via the field `spec.backup.region`.
The region of the backup can be different from where the Seed cluster is running.
However, usually it makes sense to pick the same region for the backup bucket as used for the Seed cluster.

Please find below an example `Seed` manifest (partly) that configures backups using Azure Blob storage.

```yaml
---
apiVersion: core.gardener.cloud/v1beta1
kind: Seed
metadata:
  name: my-seed
spec:
  provider:
    type: azure
    region: westeurope
  backup:
    provider: azure
    region: westeurope # default region
    secretRef:
      name: backup-credentials
      namespace: garden
  ...
```
The referenced secret has to contain the provider credentials of the Azure subscription.
Please take a look [here](https://docs.microsoft.com/en-us/azure/active-directory/develop/howto-create-service-principal-portal) on how to create an Azure Application, Service Principle and how to obtain credentials.
The example below demonstrates how the secret has to look like.

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

#### Permissions for Azure Blob storage

Please make sure the Azure application has the following IAM roles.
- [Contributor](https://docs.microsoft.com/en-us/azure/role-based-access-control/built-in-roles#contributor)

## Miscellaneous

### Gardener managed Service Principals

The operators of the Gardener Azure extension can provide a list of managed service principals (technical users) that can be used for Azure Shoots.
This eliminates the need for users to provide own service principals for their clusters.

The user would need to grant the managed service principal access to their subscription with proper permissions.

As service principals are managed in an Azure Active Directory for each supported Active Directory, an own service principal needs to be provided.

In case the user provides an own service principal in the Shoot secret, this one will be used instead of the managed one provided by the operator.

Each managed service principal will be maintained in a `Secret` like that:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: service-principal-my-tenant
  namespace: extension-provider-azure
  labels:
    azure.provider.extensions.gardener.cloud/purpose: tenant-service-principal-secret
data:
  tenantID: base64(my-tenant)
  clientID: base64(my-service-princiapl-id)
  clientSecret: base64(my-service-princiapl-secret)
type: Opaque
```

The user needs to provide in its Shoot secret a `tenantID` and `subscriptionID`.

The managed service principal will be assigned based on the `tenantID`.
In case there is a managed service principal secret with a matching `tenantID`, this one will be used for the Shoot.
If there is no matching managed service principal secret then the next Shoot operation will fail.

One of the benefits of having managed service principals is that the operator controls the lifecycle of the service principal and can rotate its secrets.

After the service principal secret has been rotated and the corresponding secret is updated, all Shoot clusters using it need to be reconciled or the last operation to be retried.

### In-place vs Rolling-updates of Shoot Workers

Changes to the `Shoot` worker-pools are applied in-place where possible. In case this is not possible a rolling update of the workers will be performed to apply the new configuration, as outlined in [the Gardener documentation](https://github.com/gardener/gardener/blob/master/docs/usage/shoot-operations/shoot_updates.md#in-place-vs-rolling-updates). The exact fields that trigger this behaviour depend on whether the feature gate `NewWorkerPoolHash` is enabled. If it is not enabled, the fields mentioned in the [Gardener doc](https://github.com/gardener/gardener/blob/master/docs/usage/shoot-operations/shoot_updates.md#rolling-update-triggers) are used, with a few additions:

- `.spec.provider.infrastructureConfig.identity`
- `.spec.provider.workers[].dataVolumes[].size` and `.spec.provider.workers[].dataVolumes[].type` (only the affected worker pool)

If the feature gate _is_ enabled, instead of the complete provider config only the fields explicitly mentioned above are used, with the addition of

- `.spec.provider.workers[].providerConfig.diagnosticsProfile`
