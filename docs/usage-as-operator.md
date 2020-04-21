# Using the Azure provider extension with Gardener as operator

The [`core.gardener.cloud/v1alpha1.CloudProfile` resource](https://github.com/gardener/gardener/blob/master/example/30-cloudprofile.yaml) declares a `providerConfig` field that is meant to contain provider-specific configuration.

In this document we are describing how this configuration looks like for Azure and provide an example `CloudProfile` manifest with minimal configuration that you can use to allow creating Azure shoot clusters.

## `CloudProfileConfig`



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
    acceleratedNetworking: true
- name: myimage
  versions:
  - version: 1.0.0
    id: "/subscriptions/<subscription ID where the gallery is located>/resourceGroups/myGalleryRG/providers/Microsoft.Compute/galleries/myGallery/images/myImageDefinition/versions/1.0.0"
```

The cloud profile configuration contains information about the update via `.countUpdateDomains[]` and failure domain via `.countFaultDomains[]` counts in the Azure regions you want to offer.

The `.machineTypes[]` list contain provider specific information to the machine types e.g. if the machine type support [Azure Accelerated Networking](https://docs.microsoft.com/en-us/azure/virtual-network/create-vm-accelerated-networking-cli), see `.machineTypes[].acceleratedNetworking`.

Additionally, it contains the real machine image identifiers in the Azure environment. You can provide either URN for Azure Market Place images or id of [Shared Image Gallery](https://docs.microsoft.com/en-us/azure/virtual-machines/linux/shared-image-galleries) images.
When Shared Image Gallery is used, you have to ensure that the image is available in the desired regions and the end-user subscriptions have access to the image or to the whole gallery.
You have to map every version that you specify in `.spec.machineImages[].versions` here such that the Azure extension knows the machine image identifiers for every version you want to offer.
Furthermore, you can specify for each image version via `.machineImages[].versions[].acceleratedNetworking` if Azure Accelerated Networking is supported.

## Example `CloudProfile` manifest

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
    - version: 1.16.1
    - version: 1.16.0
      expirationDate: "2020-04-05T01:02:03Z"
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
        acceleratedNetworking: true
      - version: 2135.6.0
        urn: "CoreOS:CoreOS:Stable:2135.6.0"
```
