<p>Packages:</p>
<ul>
<li>
<a href="#azure.provider.extensions.gardener.cloud%2fv1alpha1">azure.provider.extensions.gardener.cloud/v1alpha1</a>
</li>
</ul>
<h2 id="azure.provider.extensions.gardener.cloud/v1alpha1">azure.provider.extensions.gardener.cloud/v1alpha1</h2>
<p>
<p>Package v1alpha1 contains the Azure provider API resources.</p>
</p>
Resource Types:
<ul><li>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.BackupBucketConfig">BackupBucketConfig</a>
</li><li>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.CloudProfileConfig">CloudProfileConfig</a>
</li><li>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.ControlPlaneConfig">ControlPlaneConfig</a>
</li><li>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.InfrastructureConfig">InfrastructureConfig</a>
</li><li>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.WorkerConfig">WorkerConfig</a>
</li><li>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.WorkerStatus">WorkerStatus</a>
</li><li>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.WorkloadIdentityConfig">WorkloadIdentityConfig</a>
</li></ul>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.BackupBucketConfig">BackupBucketConfig
</h3>
<p>
<p>BackupBucketConfig is the provider-specific configuration for backup buckets/entries</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>
azure.provider.extensions.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>BackupBucketConfig</code></td>
</tr>
<tr>
<td>
<code>cloudConfiguration</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.CloudConfiguration">
CloudConfiguration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CloudConfiguration contains config that controls which cloud to connect to.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.CloudProfileConfig">CloudProfileConfig
</h3>
<p>
<p>CloudProfileConfig contains provider-specific configuration that is embedded into Gardener&rsquo;s <code>CloudProfile</code>
resource.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>
azure.provider.extensions.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>CloudProfileConfig</code></td>
</tr>
<tr>
<td>
<code>countUpdateDomains</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.DomainCount">
[]DomainCount
</a>
</em>
</td>
<td>
<p>CountUpdateDomains is list of update domain counts for each region.</p>
</td>
</tr>
<tr>
<td>
<code>countFaultDomains</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.DomainCount">
[]DomainCount
</a>
</em>
</td>
<td>
<p>CountFaultDomains is list of fault domain counts for each region.</p>
</td>
</tr>
<tr>
<td>
<code>machineImages</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.MachineImages">
[]MachineImages
</a>
</em>
</td>
<td>
<p>MachineImages is the list of machine images that are understood by the controller. It maps
logical names and versions to provider-specific identifiers.</p>
</td>
</tr>
<tr>
<td>
<code>machineTypes</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.MachineType">
[]MachineType
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineTypes is a list of machine types complete with provider specific information.</p>
</td>
</tr>
<tr>
<td>
<code>cloudConfiguration</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.CloudConfiguration">
CloudConfiguration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CloudConfiguration contains config that controls which cloud to connect to.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.ControlPlaneConfig">ControlPlaneConfig
</h3>
<p>
<p>ControlPlaneConfig contains configuration settings for the control plane.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>
azure.provider.extensions.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>ControlPlaneConfig</code></td>
</tr>
<tr>
<td>
<code>cloudControllerManager</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.CloudControllerManagerConfig">
CloudControllerManagerConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CloudControllerManager contains configuration settings for the cloud-controller-manager.</p>
</td>
</tr>
<tr>
<td>
<code>storage</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.Storage">
Storage
</a>
</em>
</td>
<td>
<p>Storage contains configuration for storage in the cluster.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.InfrastructureConfig">InfrastructureConfig
</h3>
<p>
<p>InfrastructureConfig infrastructure configuration resource</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>
azure.provider.extensions.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>InfrastructureConfig</code></td>
</tr>
<tr>
<td>
<code>resourceGroup</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.ResourceGroup">
ResourceGroup
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ResourceGroup is azure resource group.</p>
</td>
</tr>
<tr>
<td>
<code>networks</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.NetworkConfig">
NetworkConfig
</a>
</em>
</td>
<td>
<p>Networks is the network configuration (VNet, subnets, etc.).</p>
</td>
</tr>
<tr>
<td>
<code>identity</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.IdentityConfig">
IdentityConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Identity contains configuration for the assigned managed identity.</p>
</td>
</tr>
<tr>
<td>
<code>zoned</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zoned indicates whether the cluster uses availability zones.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.WorkerConfig">WorkerConfig
</h3>
<p>
<p>WorkerConfig contains configuration settings for the worker nodes.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>
azure.provider.extensions.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>WorkerConfig</code></td>
</tr>
<tr>
<td>
<code>nodeTemplate</code></br>
<em>
github.com/gardener/gardener/pkg/apis/extensions/v1alpha1.NodeTemplate
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeTemplate contains resource information of the machine which is used by Cluster Autoscaler to generate nodeTemplate during scaling a nodeGroup from zero.</p>
</td>
</tr>
<tr>
<td>
<code>diagnosticsProfile</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.DiagnosticsProfile">
DiagnosticsProfile
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DiagnosticsProfile specifies boot diagnostic options.</p>
</td>
</tr>
<tr>
<td>
<code>dataVolumes</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.DataVolume">
[]DataVolume
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DataVolumes contains configuration for the additional disks attached to VMs.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.WorkerStatus">WorkerStatus
</h3>
<p>
<p>WorkerStatus contains information about created worker resources.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>
azure.provider.extensions.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>WorkerStatus</code></td>
</tr>
<tr>
<td>
<code>machineImages</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.MachineImage">
[]MachineImage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineImages is a list of machine images that have been used in this worker. Usually, the extension controller
gets the mapping from name/version to the provider-specific machine image data in its componentconfig. However, if
a version that is still in use gets removed from this componentconfig it cannot reconcile anymore existing <code>Worker</code>
resources that are still using this version. Hence, it stores the used versions in the provider status to ensure
reconciliation is possible.</p>
</td>
</tr>
<tr>
<td>
<code>vmoDependencies</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.VmoDependency">
[]VmoDependency
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>VmoDependencies is a list of external VirtualMachineScaleSet Orchestration Mode VM (VMO) dependencies.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.WorkloadIdentityConfig">WorkloadIdentityConfig
</h3>
<p>
<p>WorkloadIdentityConfig contains configuration settings for workload identity.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>
azure.provider.extensions.gardener.cloud/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>WorkloadIdentityConfig</code></td>
</tr>
<tr>
<td>
<code>clientID</code></br>
<em>
string
</em>
</td>
<td>
<p>ClientID is the ID of the Azure client.</p>
</td>
</tr>
<tr>
<td>
<code>tenantID</code></br>
<em>
string
</em>
</td>
<td>
<p>TenantID is the ID of the Azure tenant.</p>
</td>
</tr>
<tr>
<td>
<code>subscriptionID</code></br>
<em>
string
</em>
</td>
<td>
<p>SubscriptionID is the ID of the subscription.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.AvailabilitySet">AvailabilitySet
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.InfrastructureStatus">InfrastructureStatus</a>)
</p>
<p>
<p>AvailabilitySet contains information about the azure availability set</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>purpose</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.Purpose">
Purpose
</a>
</em>
</td>
<td>
<p>Purpose is the purpose of the availability set</p>
</td>
</tr>
<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<p>ID is the id of the availability set</p>
</td>
</tr>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the availability set</p>
</td>
</tr>
<tr>
<td>
<code>countFaultDomains</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>CountFaultDomains is the count of fault domains.</p>
</td>
</tr>
<tr>
<td>
<code>countUpdateDomains</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>CountUpdateDomains is the count of update domains.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.AzureResource">AzureResource
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.InfrastructureState">InfrastructureState</a>)
</p>
<p>
<p>AzureResource represents metadata information about created infrastructure resources.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>kind</code></br>
<em>
string
</em>
</td>
<td>
<p>Kind is the type of resource.</p>
</td>
</tr>
<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<p>ID is the ID of the resource.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.CloudConfiguration">CloudConfiguration
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.BackupBucketConfig">BackupBucketConfig</a>, 
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.CloudProfileConfig">CloudProfileConfig</a>)
</p>
<p>
<p>CloudConfiguration contains detailed config for the cloud to connect to. Currently we only support selection of well-
known Azure-instances by name, but this could be extended in future to support private clouds.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the cloud to connect to, e.g. &ldquo;AzurePublic&rdquo; or &ldquo;AzureChina&rdquo;.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.CloudControllerManagerConfig">CloudControllerManagerConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.ControlPlaneConfig">ControlPlaneConfig</a>)
</p>
<p>
<p>CloudControllerManagerConfig contains configuration settings for the cloud-controller-manager.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>featureGates</code></br>
<em>
map[string]bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>FeatureGates contains information about enabled feature gates.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.DataVolume">DataVolume
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.WorkerConfig">WorkerConfig</a>)
</p>
<p>
<p>DataVolume contains configuration for data volumes attached to VMs.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the data volume this configuration applies to.</p>
</td>
</tr>
<tr>
<td>
<code>imageRef</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.Image">
Image
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ImageRef defines the dataVolume source image.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.DiagnosticsProfile">DiagnosticsProfile
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.WorkerConfig">WorkerConfig</a>)
</p>
<p>
<p>DiagnosticsProfile specifies boot diagnostic options.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>enabled</code></br>
<em>
bool
</em>
</td>
<td>
<p>Enabled configures boot diagnostics to be stored or not.</p>
</td>
</tr>
<tr>
<td>
<code>storageURI</code></br>
<em>
string
</em>
</td>
<td>
<p>StorageURI is the URI of the storage account to use for storing console output and screenshot.
If not specified azure managed storage will be used.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.DomainCount">DomainCount
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.CloudProfileConfig">CloudProfileConfig</a>)
</p>
<p>
<p>DomainCount defines the region and the count for this domain count value.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>region</code></br>
<em>
string
</em>
</td>
<td>
<p>Region is a region.</p>
</td>
</tr>
<tr>
<td>
<code>count</code></br>
<em>
int32
</em>
</td>
<td>
<p>Count is the count value for the respective domain count.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.IdentityConfig">IdentityConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.InfrastructureConfig">InfrastructureConfig</a>)
</p>
<p>
<p>IdentityConfig contains configuration for the managed identity.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the identity.</p>
</td>
</tr>
<tr>
<td>
<code>resourceGroup</code></br>
<em>
string
</em>
</td>
<td>
<p>ResourceGroup is the resource group where the identity belongs to.</p>
</td>
</tr>
<tr>
<td>
<code>acrAccess</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>ACRAccess indicated if the identity should be used by the Shoot worker nodes to pull from an Azure Container Registry.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.IdentityStatus">IdentityStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.InfrastructureStatus">InfrastructureStatus</a>)
</p>
<p>
<p>IdentityStatus contains the status information of the created managed identity.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<p>ID is the Azure resource if of the identity.</p>
</td>
</tr>
<tr>
<td>
<code>clientID</code></br>
<em>
string
</em>
</td>
<td>
<p>ClientID is the client id of the identity.</p>
</td>
</tr>
<tr>
<td>
<code>acrAccess</code></br>
<em>
bool
</em>
</td>
<td>
<p>ACRAccess specifies if the identity should be used by the Shoot worker nodes to pull from an Azure Container Registry.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.Image">Image
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.DataVolume">DataVolume</a>, 
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.MachineImage">MachineImage</a>)
</p>
<p>
<p>Image identifies the azure image.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>urn</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>URN is the uniform resource name of the image, it has the format &lsquo;publisher:offer:sku:version&rsquo;.</p>
</td>
</tr>
<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ID is the VM image ID.</p>
</td>
</tr>
<tr>
<td>
<code>communityGalleryImageID</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>CommunityGalleryImageID is the Community Image Gallery image id.</p>
</td>
</tr>
<tr>
<td>
<code>sharedGalleryImageID</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SharedGalleryImageID is the Shared Image Gallery image id.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.InfrastructureState">InfrastructureState
</h3>
<p>
<p>InfrastructureState contains state information of the infrastructure resource.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>data</code></br>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Data is map to store things.</p>
</td>
</tr>
<tr>
<td>
<code>managedItems</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.AzureResource">
[]AzureResource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ManagedItems is a list of resources that were created during the infrastructure reconciliation.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.InfrastructureStatus">InfrastructureStatus
</h3>
<p>
<p>InfrastructureStatus contains information about created infrastructure resources.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>networks</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.NetworkStatus">
NetworkStatus
</a>
</em>
</td>
<td>
<p>Networks is the status of the networks of the infrastructure.</p>
</td>
</tr>
<tr>
<td>
<code>resourceGroup</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.ResourceGroup">
ResourceGroup
</a>
</em>
</td>
<td>
<p>ResourceGroup is azure resource group</p>
</td>
</tr>
<tr>
<td>
<code>availabilitySets</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.AvailabilitySet">
[]AvailabilitySet
</a>
</em>
</td>
<td>
<p>AvailabilitySets is a list of created availability sets
Deprecated: Will be removed in future versions.</p>
</td>
</tr>
<tr>
<td>
<code>migratingToVMO</code></br>
<em>
bool
</em>
</td>
<td>
<p>MigratingToVMO indicates whether the infrastructure controller has prepared the migration from Availability set.
Deprecated: Will be removed in future versions.</p>
</td>
</tr>
<tr>
<td>
<code>routeTables</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.RouteTable">
[]RouteTable
</a>
</em>
</td>
<td>
<p>RouteTables is a list of created route tables</p>
</td>
</tr>
<tr>
<td>
<code>securityGroups</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.SecurityGroup">
[]SecurityGroup
</a>
</em>
</td>
<td>
<p>SecurityGroups is a list of created security groups</p>
</td>
</tr>
<tr>
<td>
<code>identity</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.IdentityStatus">
IdentityStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Identity is the status of the managed identity.</p>
</td>
</tr>
<tr>
<td>
<code>zoned</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zoned indicates whether the cluster uses zones</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.MachineImage">MachineImage
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.WorkerStatus">WorkerStatus</a>)
</p>
<p>
<p>MachineImage is a mapping from logical names and versions to provider-specific machine image data.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the logical name of the machine image.</p>
</td>
</tr>
<tr>
<td>
<code>version</code></br>
<em>
string
</em>
</td>
<td>
<p>Version is the logical version of the machine image.</p>
</td>
</tr>
<tr>
<td>
<code>acceleratedNetworking</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>AcceleratedNetworking is an indicator if the image supports Azure accelerated networking.</p>
</td>
</tr>
<tr>
<td>
<code>architecture</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Architecture is the CPU architecture of the machine image.</p>
</td>
</tr>
<tr>
<td>
<code>skipMarketplaceAgreement</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>SkipMarketplaceAgreement skips the marketplace agreement check when enabled.</p>
</td>
</tr>
<tr>
<td>
<code>Image</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.Image">
Image
</a>
</em>
</td>
<td>
<p>
(Members of <code>Image</code> are embedded into this type.)
</p>
<p>Image identifies the azure image.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.MachineImageVersion">MachineImageVersion
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.MachineImages">MachineImages</a>)
</p>
<p>
<p>MachineImageVersion contains a version and a provider-specific identifier.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>version</code></br>
<em>
string
</em>
</td>
<td>
<p>Version is the version of the image.</p>
</td>
</tr>
<tr>
<td>
<code>urn</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>URN is the uniform resource name of the image, it has the format &lsquo;publisher:offer:sku:version&rsquo;.</p>
</td>
</tr>
<tr>
<td>
<code>skipMarketplaceAgreement</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>SkipMarketplaceAgreement skips the marketplace agreement check when enabled.</p>
</td>
</tr>
<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ID is the Shared Image Gallery image id.</p>
</td>
</tr>
<tr>
<td>
<code>communityGalleryImageID</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>CommunityGalleryImageID is the Community Image Gallery image id, it has the format &lsquo;/CommunityGalleries/myGallery/Images/myImage/Versions/myVersion&rsquo;</p>
</td>
</tr>
<tr>
<td>
<code>sharedGalleryImageID</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SharedGalleryImageID is the Shared Image Gallery image id, it has the format &lsquo;/SharedGalleries/sharedGalleryName/Images/sharedGalleryImageName/Versions/sharedGalleryImageVersionName&rsquo;</p>
</td>
</tr>
<tr>
<td>
<code>acceleratedNetworking</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>AcceleratedNetworking is an indicator if the image supports Azure accelerated networking.</p>
</td>
</tr>
<tr>
<td>
<code>architecture</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Architecture is the CPU architecture of the machine image.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.MachineImages">MachineImages
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.CloudProfileConfig">CloudProfileConfig</a>)
</p>
<p>
<p>MachineImages is a mapping from logical names and versions to provider-specific identifiers.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the logical name of the machine image.</p>
</td>
</tr>
<tr>
<td>
<code>versions</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.MachineImageVersion">
[]MachineImageVersion
</a>
</em>
</td>
<td>
<p>Versions contains versions and a provider-specific identifier.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.MachineType">MachineType
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.CloudProfileConfig">CloudProfileConfig</a>)
</p>
<p>
<p>MachineType contains provider specific information to a machine type.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the machine type.</p>
</td>
</tr>
<tr>
<td>
<code>acceleratedNetworking</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>AcceleratedNetworking is an indicator if the machine type supports Azure accelerated networking.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.NatGatewayConfig">NatGatewayConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.NetworkConfig">NetworkConfig</a>)
</p>
<p>
<p>NatGatewayConfig contains configuration for the NAT gateway and the attached resources.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>enabled</code></br>
<em>
bool
</em>
</td>
<td>
<p>Enabled is an indicator if NAT gateway should be deployed.</p>
</td>
</tr>
<tr>
<td>
<code>idleConnectionTimeoutMinutes</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>IdleConnectionTimeoutMinutes specifies the idle connection timeout limit for NAT gateway in minutes.</p>
</td>
</tr>
<tr>
<td>
<code>zone</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zone specifies the zone in which the NAT gateway should be deployed to.</p>
</td>
</tr>
<tr>
<td>
<code>ipAddresses</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.PublicIPReference">
[]PublicIPReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>IPAddresses is a list of ip addresses which should be assigned to the NAT gateway.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.NetworkConfig">NetworkConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.InfrastructureConfig">InfrastructureConfig</a>)
</p>
<p>
<p>NetworkConfig holds information about the Kubernetes and infrastructure networks.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>vnet</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.VNet">
VNet
</a>
</em>
</td>
<td>
<p>VNet indicates whether to use an existing VNet or create a new one.</p>
</td>
</tr>
<tr>
<td>
<code>workers</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Workers is the worker subnet range to create (used for the VMs).</p>
</td>
</tr>
<tr>
<td>
<code>natGateway</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.NatGatewayConfig">
NatGatewayConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NatGateway contains the configuration for the NatGateway.</p>
</td>
</tr>
<tr>
<td>
<code>serviceEndpoints</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ServiceEndpoints is a list of Azure ServiceEndpoints which should be associated with the worker subnet.</p>
</td>
</tr>
<tr>
<td>
<code>zones</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.Zone">
[]Zone
</a>
</em>
</td>
<td>
<p>Zones is a list of zones with their respective configuration.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.NetworkLayout">NetworkLayout
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.NetworkStatus">NetworkStatus</a>)
</p>
<p>
<p>NetworkLayout is the network layout type for the cluster.</p>
</p>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.NetworkStatus">NetworkStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.InfrastructureStatus">InfrastructureStatus</a>)
</p>
<p>
<p>NetworkStatus is the current status of the infrastructure networks.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>vnet</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.VNetStatus">
VNetStatus
</a>
</em>
</td>
<td>
<p>VNetStatus states the name of the infrastructure VNet.</p>
</td>
</tr>
<tr>
<td>
<code>subnets</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.Subnet">
[]Subnet
</a>
</em>
</td>
<td>
<p>Subnets are the subnets that have been created.</p>
</td>
</tr>
<tr>
<td>
<code>layout</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.NetworkLayout">
NetworkLayout
</a>
</em>
</td>
<td>
<p>Layout describes the network layout of the cluster.</p>
</td>
</tr>
<tr>
<td>
<code>outboundAccessType</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.OutboundAccessType">
OutboundAccessType
</a>
</em>
</td>
<td>
<p>OutboundAccessType is the type of outbound access configured for the shoot. It indicates how egress traffic flows outside the shoot.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.OutboundAccessType">OutboundAccessType
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.NetworkStatus">NetworkStatus</a>)
</p>
<p>
<p>OutboundAccessType is the type of outbound access configured for the shoot. It indicates how egress traffic flows outside the shoot.
See <a href="https://learn.microsoft.com/en-us/azure/load-balancer/load-balancer-outbound-connections#scenarios">https://learn.microsoft.com/en-us/azure/load-balancer/load-balancer-outbound-connections#scenarios</a></p>
</p>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.PublicIPReference">PublicIPReference
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.NatGatewayConfig">NatGatewayConfig</a>)
</p>
<p>
<p>PublicIPReference contains information about a public ip.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the public ip.</p>
</td>
</tr>
<tr>
<td>
<code>resourceGroup</code></br>
<em>
string
</em>
</td>
<td>
<p>ResourceGroup is the name of the resource group where the public ip is assigned to.</p>
</td>
</tr>
<tr>
<td>
<code>zone</code></br>
<em>
int32
</em>
</td>
<td>
<p>Zone is the zone in which the public ip is deployed to.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.Purpose">Purpose
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.AvailabilitySet">AvailabilitySet</a>, 
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.RouteTable">RouteTable</a>, 
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.SecurityGroup">SecurityGroup</a>, 
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.Subnet">Subnet</a>)
</p>
<p>
<p>Purpose is a purpose of a subnet.</p>
</p>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.ResourceGroup">ResourceGroup
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.InfrastructureConfig">InfrastructureConfig</a>, 
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.InfrastructureStatus">InfrastructureStatus</a>)
</p>
<p>
<p>ResourceGroup is azure resource group</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the resource group</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.RouteTable">RouteTable
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.InfrastructureStatus">InfrastructureStatus</a>)
</p>
<p>
<p>RouteTable is the azure route table</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>purpose</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.Purpose">
Purpose
</a>
</em>
</td>
<td>
<p>Purpose is the purpose of the route table</p>
</td>
</tr>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the route table</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.SecurityGroup">SecurityGroup
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.InfrastructureStatus">InfrastructureStatus</a>)
</p>
<p>
<p>SecurityGroup contains information about the security group</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>purpose</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.Purpose">
Purpose
</a>
</em>
</td>
<td>
<p>Purpose is the purpose of the security group</p>
</td>
</tr>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the security group</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.Storage">Storage
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.ControlPlaneConfig">ControlPlaneConfig</a>)
</p>
<p>
<p>Storage contains configuration for storage in the cluster.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>managedDefaultStorageClass</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>ManagedDefaultStorageClass controls if the &lsquo;default&rsquo; StorageClass would be marked as default. Set to false to
manually set the default to another class not managed by Gardener.
Defaults to true.</p>
</td>
</tr>
<tr>
<td>
<code>managedDefaultVolumeSnapshotClass</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>ManagedDefaultVolumeSnapshotClass controls if the &lsquo;default&rsquo; VolumeSnapshotClass would be marked as default.
Set to false to manually set the default to another class not managed by Gardener.
Defaults to true.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.Subnet">Subnet
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.NetworkStatus">NetworkStatus</a>)
</p>
<p>
<p>Subnet is a subnet that was created.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the subnet.</p>
</td>
</tr>
<tr>
<td>
<code>purpose</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.Purpose">
Purpose
</a>
</em>
</td>
<td>
<p>Purpose is the purpose for which the subnet was created.</p>
</td>
</tr>
<tr>
<td>
<code>zone</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zone is the name of the zone for which the subnet was created.</p>
</td>
</tr>
<tr>
<td>
<code>migrated</code></br>
<em>
bool
</em>
</td>
<td>
<p>Migrated is set when the network layout is migrated from NetworkLayoutSingleSubnet to NetworkLayoutMultipleSubnet.
Only the subnet that was used prior to the migration should have this attribute set.</p>
</td>
</tr>
<tr>
<td>
<code>natGatewayId</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>NatGatewayID is the ID of the NATGateway associated with the subnet.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.VNet">VNet
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.NetworkConfig">NetworkConfig</a>)
</p>
<p>
<p>VNet contains information about the VNet and some related resources.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Name is the name of an existing vNet which should be used.</p>
</td>
</tr>
<tr>
<td>
<code>resourceGroup</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ResourceGroup is the resource group where the existing vNet blongs to.</p>
</td>
</tr>
<tr>
<td>
<code>cidr</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>CIDR is the VNet CIDR</p>
</td>
</tr>
<tr>
<td>
<code>ddosProtectionPlanID</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>DDosProtectionPlanID is the id of a ddos protection plan assigned to the vnet.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.VNetStatus">VNetStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.NetworkStatus">NetworkStatus</a>)
</p>
<p>
<p>VNetStatus contains the VNet name.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the VNet name.</p>
</td>
</tr>
<tr>
<td>
<code>resourceGroup</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ResourceGroup is the resource group where the existing vNet belongs to.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.VmoDependency">VmoDependency
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.WorkerStatus">WorkerStatus</a>)
</p>
<p>
<p>VmoDependency is dependency reference for a workerpool to a VirtualMachineScaleSet Orchestration Mode VM (VMO).</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>poolName</code></br>
<em>
string
</em>
</td>
<td>
<p>PoolName is the name of the worker pool to which the VMO belong to.</p>
</td>
</tr>
<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<p>ID is the id of the VMO resource on Azure.</p>
</td>
</tr>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the VMO resource on Azure.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.Zone">Zone
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.NetworkConfig">NetworkConfig</a>)
</p>
<p>
<p>Zone describes the configuration for a subnet that is used for VMs on that region.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
int32
</em>
</td>
<td>
<p>Name is the name of the zone and should match with the name the infrastructure provider is using for the zone.</p>
</td>
</tr>
<tr>
<td>
<code>cidr</code></br>
<em>
string
</em>
</td>
<td>
<p>CIDR is the CIDR range used for the zone&rsquo;s subnet.</p>
</td>
</tr>
<tr>
<td>
<code>serviceEndpoints</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ServiceEndpoints is a list of Azure ServiceEndpoints which should be associated with the zone&rsquo;s subnet.</p>
</td>
</tr>
<tr>
<td>
<code>natGateway</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.ZonedNatGatewayConfig">
ZonedNatGatewayConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NatGateway contains the configuration for the NatGateway associated with this subnet.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.ZonedNatGatewayConfig">ZonedNatGatewayConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.Zone">Zone</a>)
</p>
<p>
<p>ZonedNatGatewayConfig contains configuration for NAT gateway and the attached resources.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>enabled</code></br>
<em>
bool
</em>
</td>
<td>
<p>Enabled is an indicator if NAT gateway should be deployed.</p>
</td>
</tr>
<tr>
<td>
<code>idleConnectionTimeoutMinutes</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>IdleConnectionTimeoutMinutes specifies the idle connection timeout limit for NAT gateway in minutes.</p>
</td>
</tr>
<tr>
<td>
<code>ipAddresses</code></br>
<em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.ZonedPublicIPReference">
[]ZonedPublicIPReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>IPAddresses is a list of ip addresses which should be assigned to the NAT gateway.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="azure.provider.extensions.gardener.cloud/v1alpha1.ZonedPublicIPReference">ZonedPublicIPReference
</h3>
<p>
(<em>Appears on:</em>
<a href="#azure.provider.extensions.gardener.cloud/v1alpha1.ZonedNatGatewayConfig">ZonedNatGatewayConfig</a>)
</p>
<p>
<p>ZonedPublicIPReference contains information about a public ip.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the public ip.</p>
</td>
</tr>
<tr>
<td>
<code>resourceGroup</code></br>
<em>
string
</em>
</td>
<td>
<p>ResourceGroup is the name of the resource group where the public ip is assigned to.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <a href="https://github.com/ahmetb/gen-crd-api-reference-docs">gen-crd-api-reference-docs</a>
</em></p>
