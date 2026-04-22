<p>Packages:</p>
<ul>
<li>
<a href="#azure.provider.extensions.gardener.cloud%2fv1alpha1">azure.provider.extensions.gardener.cloud/v1alpha1</a>
</li>
</ul>

<h2 id="azure.provider.extensions.gardener.cloud/v1alpha1">azure.provider.extensions.gardener.cloud/v1alpha1</h2>
<p>

</p>

<h3 id="azureresource">AzureResource
</h3>


<p>
(<em>Appears on:</em><a href="#infrastructurestate">InfrastructureState</a>)
</p>

<p>
AzureResource represents metadata information about created infrastructure resources.
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


<h3 id="backupbucketconfig">BackupBucketConfig
</h3>


<p>
BackupBucketConfig is the provider-specific configuration for backup buckets/entries
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
<code>cloudConfiguration</code></br>
<em>
<a href="#cloudconfiguration">CloudConfiguration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CloudConfiguration contains config that controls which cloud to connect to.</p>
</td>
</tr>
<tr>
<td>
<code>immutability</code></br>
<em>
<a href="#immutableconfig">ImmutableConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Immutability defines the immutability config for the backup bucket.</p>
</td>
</tr>
<tr>
<td>
<code>rotationConfig</code></br>
<em>
<a href="#rotationconfig">RotationConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RotationConfig controls the behavior for the rotation of storage account keys.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="capacityreservation">CapacityReservation
</h3>


<p>
(<em>Appears on:</em><a href="#workerconfig">WorkerConfig</a>)
</p>

<p>
CapacityReservation represents the configuration for capacity reservations on Azure.
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
<code>capacityReservationGroupID</code></br>
<em>
string
</em>
</td>
<td>
<p>CapacityReservationGroupID is the resource ID of the capacity reservation group to use.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="cloudconfiguration">CloudConfiguration
</h3>


<p>
(<em>Appears on:</em><a href="#backupbucketconfig">BackupBucketConfig</a>, <a href="#cloudprofileconfig">CloudProfileConfig</a>)
</p>

<p>
CloudConfiguration contains detailed config for the cloud to connect to. Currently we only support selection of well-
known Azure-instances by name, but this could be extended in future to support private clouds.
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
<p>Name is the name of the cloud to connect to, e.g. "AzurePublic" or "AzureChina".</p>
</td>
</tr>

</tbody>
</table>


<h3 id="cloudcontrollermanagerconfig">CloudControllerManagerConfig
</h3>


<p>
(<em>Appears on:</em><a href="#controlplaneconfig">ControlPlaneConfig</a>)
</p>

<p>
CloudControllerManagerConfig contains configuration settings for the cloud-controller-manager.
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
object (keys:string, values:boolean)
</em>
</td>
<td>
<em>(Optional)</em>
<p>FeatureGates contains information about enabled feature gates.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="cloudprofileconfig">CloudProfileConfig
</h3>


<p>
CloudProfileConfig contains provider-specific configuration that is embedded into Gardener's `CloudProfile`
resource.
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
<code>countUpdateDomains</code></br>
<em>
<a href="#domaincount">DomainCount</a> array
</em>
</td>
<td>
<p>CountUpdateDomains is list of update domain counts for each region.<br />Deprecated: VMSS does not allow specifying update domain count. With the deprecation of Availability Sets, only CountFaultDomains is required.</p>
</td>
</tr>
<tr>
<td>
<code>countFaultDomains</code></br>
<em>
<a href="#domaincount">DomainCount</a> array
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
<a href="#machineimages">MachineImages</a> array
</em>
</td>
<td>
<p>MachineImages is the list of machine images that are understood by the controller. It maps<br />logical names and versions to provider-specific identifiers.</p>
</td>
</tr>
<tr>
<td>
<code>machineTypes</code></br>
<em>
<a href="#machinetype">MachineType</a> array
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
<a href="#cloudconfiguration">CloudConfiguration</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CloudConfiguration contains config that controls which cloud to connect to.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="controlplaneconfig">ControlPlaneConfig
</h3>


<p>
ControlPlaneConfig contains configuration settings for the control plane.
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
<code>cloudControllerManager</code></br>
<em>
<a href="#cloudcontrollermanagerconfig">CloudControllerManagerConfig</a>
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
<a href="#storage">Storage</a>
</em>
</td>
<td>
<p>Storage contains configuration for storage in the cluster.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="datavolume">DataVolume
</h3>


<p>
(<em>Appears on:</em><a href="#workerconfig">WorkerConfig</a>)
</p>

<p>
DataVolume contains configuration for data volumes attached to VMs.
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
<a href="#image">Image</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ImageRef defines the dataVolume source image.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="diagnosticsprofile">DiagnosticsProfile
</h3>


<p>
(<em>Appears on:</em><a href="#workerconfig">WorkerConfig</a>)
</p>

<p>
DiagnosticsProfile specifies boot diagnostic options.
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
boolean
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
<p>StorageURI is the URI of the storage account to use for storing console output and screenshot.<br />If not specified azure managed storage will be used.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="domaincount">DomainCount
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofileconfig">CloudProfileConfig</a>)
</p>

<p>
DomainCount defines the region and the count for this domain count value.
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
integer
</em>
</td>
<td>
<p>Count is the count value for the respective domain count.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="identityconfig">IdentityConfig
</h3>


<p>
(<em>Appears on:</em><a href="#infrastructureconfig">InfrastructureConfig</a>)
</p>

<p>
IdentityConfig contains configuration for the managed identity.
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
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>ACRAccess indicated if the identity should be used by the Shoot worker nodes to pull from an Azure Container Registry.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="identitystatus">IdentityStatus
</h3>


<p>
(<em>Appears on:</em><a href="#infrastructurestatus">InfrastructureStatus</a>)
</p>

<p>
IdentityStatus contains the status information of the created managed identity.
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
boolean
</em>
</td>
<td>
<p>ACRAccess specifies if the identity should be used by the Shoot worker nodes to pull from an Azure Container Registry.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="image">Image
</h3>


<p>
(<em>Appears on:</em><a href="#datavolume">DataVolume</a>, <a href="#machineimage">MachineImage</a>, <a href="#machineimageflavor">MachineImageFlavor</a>, <a href="#machineimageversion">MachineImageVersion</a>)
</p>

<p>
Image identifies the azure image.
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
<p>URN is the uniform resource name of the image, it has the format 'publisher:offer:sku:version'.</p>
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
<p>CommunityGalleryImageID is the Community Image Gallery image id, it has the format '/CommunityGalleries/myGallery/Images/myImage/Versions/myVersion'</p>
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
<p>SharedGalleryImageID is the Shared Image Gallery image id, it has the format '/SharedGalleries/sharedGalleryName/Images/sharedGalleryImageName/Versions/sharedGalleryImageVersionName'</p>
</td>
</tr>

</tbody>
</table>


<h3 id="immutableconfig">ImmutableConfig
</h3>


<p>
(<em>Appears on:</em><a href="#backupbucketconfig">BackupBucketConfig</a>)
</p>

<p>
ImmutableConfig represents the immutability configuration for a backup bucket.
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
<code>retentionType</code></br>
<em>
<a href="#retentiontype">RetentionType</a>
</em>
</td>
<td>
<p>RetentionType specifies the type of retention for the backup bucket.<br />Currently allowed values are:<br />- BucketLevelImmutability: The retention policy applies to the entire bucket.</p>
</td>
</tr>
<tr>
<td>
<code>retentionPeriod</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#duration-v1-meta">Duration</a>
</em>
</td>
<td>
<p>RetentionPeriod specifies the immutability retention period for the backup bucket.</p>
</td>
</tr>
<tr>
<td>
<code>locked</code></br>
<em>
boolean
</em>
</td>
<td>
<p>Locked indicates whether the immutable retention policy is locked for the backup bucket.<br />If set to true, the retention policy cannot be removed or the retention period reduced, enforcing immutability.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="infrastructureconfig">InfrastructureConfig
</h3>


<p>
InfrastructureConfig infrastructure configuration resource
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
<code>resourceGroup</code></br>
<em>
<a href="#resourcegroup">ResourceGroup</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ResourceGroup is azure resource group.<br />Deprecated: This feature is no longer supported and will be removed in a future release.</p>
</td>
</tr>
<tr>
<td>
<code>networks</code></br>
<em>
<a href="#networkconfig">NetworkConfig</a>
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
<a href="#identityconfig">IdentityConfig</a>
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
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zoned indicates whether the cluster uses availability zones.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="infrastructurestate">InfrastructureState
</h3>


<p>
InfrastructureState contains state information of the infrastructure resource.
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
object (keys:string, values:string)
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
<a href="#azureresource">AzureResource</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>ManagedItems is a list of resources that were created during the infrastructure reconciliation.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="infrastructurestatus">InfrastructureStatus
</h3>


<p>
InfrastructureStatus contains information about created infrastructure resources.
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
<a href="#networkstatus">NetworkStatus</a>
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
<a href="#resourcegroup">ResourceGroup</a>
</em>
</td>
<td>
<p>ResourceGroup is azure resource group</p>
</td>
</tr>
<tr>
<td>
<code>routeTables</code></br>
<em>
<a href="#routetable">RouteTable</a> array
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
<a href="#securitygroup">SecurityGroup</a> array
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
<a href="#identitystatus">IdentityStatus</a>
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
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zoned indicates whether the cluster uses zones</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machineimage">MachineImage
</h3>


<p>
(<em>Appears on:</em><a href="#workerstatus">WorkerStatus</a>)
</p>

<p>
MachineImage is a mapping from logical names and versions to provider-specific machine image data.
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
boolean
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
<code>capabilities</code></br>
<em>
<a href="#capabilities">Capabilities</a>
</em>
</td>
<td>
<p>Capabilities of the machine image.</p>
</td>
</tr>
<tr>
<td>
<code>skipMarketplaceAgreement</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>SkipMarketplaceAgreement skips the marketplace agreement check when enabled.</p>
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
<p>URN is the uniform resource name of the image, it has the format 'publisher:offer:sku:version'.</p>
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
<p>CommunityGalleryImageID is the Community Image Gallery image id, it has the format '/CommunityGalleries/myGallery/Images/myImage/Versions/myVersion'</p>
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
<p>SharedGalleryImageID is the Shared Image Gallery image id, it has the format '/SharedGalleries/sharedGalleryName/Images/sharedGalleryImageName/Versions/sharedGalleryImageVersionName'</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machineimageflavor">MachineImageFlavor
</h3>


<p>
(<em>Appears on:</em><a href="#machineimageversion">MachineImageVersion</a>)
</p>

<p>
MachineImageFlavor is a flavor of the machine image version that supports a specific set of capabilities.
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
<code>skipMarketplaceAgreement</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>SkipMarketplaceAgreement skips the marketplace agreement check when enabled.</p>
</td>
</tr>
<tr>
<td>
<code>capabilities</code></br>
<em>
<a href="#capabilities">Capabilities</a>
</em>
</td>
<td>
<p>Capabilities is the set of capabilities that are supported by the image in this set.</p>
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
<p>URN is the uniform resource name of the image, it has the format 'publisher:offer:sku:version'.</p>
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
<p>CommunityGalleryImageID is the Community Image Gallery image id, it has the format '/CommunityGalleries/myGallery/Images/myImage/Versions/myVersion'</p>
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
<p>SharedGalleryImageID is the Shared Image Gallery image id, it has the format '/SharedGalleries/sharedGalleryName/Images/sharedGalleryImageName/Versions/sharedGalleryImageVersionName'</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machineimageversion">MachineImageVersion
</h3>


<p>
(<em>Appears on:</em><a href="#machineimages">MachineImages</a>)
</p>

<p>
MachineImageVersion contains a version and a provider-specific identifier.
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
<code>skipMarketplaceAgreement</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>SkipMarketplaceAgreement skips the marketplace agreement check when enabled.</p>
</td>
</tr>
<tr>
<td>
<code>acceleratedNetworking</code></br>
<em>
boolean
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
<code>capabilityFlavors</code></br>
<em>
<a href="#machineimageflavor">MachineImageFlavor</a> array
</em>
</td>
<td>
<p>CapabilityFlavors is a collection of all images for that version with capabilities.</p>
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
<p>URN is the uniform resource name of the image, it has the format 'publisher:offer:sku:version'.</p>
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
<p>CommunityGalleryImageID is the Community Image Gallery image id, it has the format '/CommunityGalleries/myGallery/Images/myImage/Versions/myVersion'</p>
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
<p>SharedGalleryImageID is the Shared Image Gallery image id, it has the format '/SharedGalleries/sharedGalleryName/Images/sharedGalleryImageName/Versions/sharedGalleryImageVersionName'</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machineimages">MachineImages
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofileconfig">CloudProfileConfig</a>)
</p>

<p>
MachineImages is a mapping from logical names and versions to provider-specific identifiers.
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
<a href="#machineimageversion">MachineImageVersion</a> array
</em>
</td>
<td>
<p>Versions contains versions and a provider-specific identifier.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="machinetype">MachineType
</h3>


<p>
(<em>Appears on:</em><a href="#cloudprofileconfig">CloudProfileConfig</a>)
</p>

<p>
MachineType contains provider specific information to a machine type.
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
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>AcceleratedNetworking is an indicator if the machine type supports Azure accelerated networking.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="natgatewayconfig">NatGatewayConfig
</h3>


<p>
(<em>Appears on:</em><a href="#networkconfig">NetworkConfig</a>)
</p>

<p>
NatGatewayConfig contains configuration for the NAT gateway and the attached resources.
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
boolean
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
integer
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
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zone specifies the zone in which the NAT gateway should be deployed to.</p>
</td>
</tr>
<tr>
<td>
<code>sku</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SKU specifies the SKU of the NAT gateway.<br />Supported values: "Standard", "StandardV2"<br />StandardV2 is zone-redundant and cannot be used with the Zone field.<br />IP addresses can be used with StandardV2, but they must also be zone-redundant (no zone specification).<br />If not specified, defaults to "Standard" for backward compatibility.</p>
</td>
</tr>
<tr>
<td>
<code>ipAddresses</code></br>
<em>
<a href="#publicipreference">PublicIPReference</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>IPAddresses is a list of ip addresses which should be assigned to the NAT gateway.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="networkconfig">NetworkConfig
</h3>


<p>
(<em>Appears on:</em><a href="#infrastructureconfig">InfrastructureConfig</a>)
</p>

<p>
NetworkConfig holds information about the Kubernetes and infrastructure networks.
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
<a href="#vnet">VNet</a>
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
<a href="#natgatewayconfig">NatGatewayConfig</a>
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
string array
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
<a href="#zone">Zone</a> array
</em>
</td>
<td>
<p>Zones is a list of zones with their respective configuration.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="networklayout">NetworkLayout
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#networkstatus">NetworkStatus</a>)
</p>

<p>
NetworkLayout is the network layout type for the cluster.
</p>


<h3 id="networkstatus">NetworkStatus
</h3>


<p>
(<em>Appears on:</em><a href="#infrastructurestatus">InfrastructureStatus</a>)
</p>

<p>
NetworkStatus is the current status of the infrastructure networks.
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
<a href="#vnetstatus">VNetStatus</a>
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
<a href="#subnet">Subnet</a> array
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
<a href="#networklayout">NetworkLayout</a>
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
<a href="#outboundaccesstype">OutboundAccessType</a>
</em>
</td>
<td>
<p>OutboundAccessType is the type of outbound access configured for the shoot. It indicates how egress traffic flows outside the shoot.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="outboundaccesstype">OutboundAccessType
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#networkstatus">NetworkStatus</a>)
</p>

<p>
OutboundAccessType is the type of outbound access configured for the shoot. It indicates how egress traffic flows outside the shoot.
See https://learn.microsoft.com/en-us/azure/load-balancer/load-balancer-outbound-connections#scenarios
</p>


<h3 id="publicipreference">PublicIPReference
</h3>


<p>
(<em>Appears on:</em><a href="#natgatewayconfig">NatGatewayConfig</a>)
</p>

<p>
PublicIPReference contains information about a public ip.
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
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zone is the zone in which the public ip is deployed to.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="purpose">Purpose
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#routetable">RouteTable</a>, <a href="#securitygroup">SecurityGroup</a>, <a href="#subnet">Subnet</a>)
</p>

<p>
Purpose is a purpose of a subnet.
</p>


<h3 id="resourcegroup">ResourceGroup
</h3>


<p>
(<em>Appears on:</em><a href="#infrastructureconfig">InfrastructureConfig</a>, <a href="#infrastructurestatus">InfrastructureStatus</a>)
</p>

<p>
ResourceGroup is azure resource group
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


<h3 id="retentiontype">RetentionType
</h3>
<p><em>Underlying type: string</em></p>


<p>
(<em>Appears on:</em><a href="#immutableconfig">ImmutableConfig</a>)
</p>

<p>
RetentionType defines the level at which immutability properties are obtained by objects
</p>


<h3 id="rotationconfig">RotationConfig
</h3>


<p>
(<em>Appears on:</em><a href="#backupbucketconfig">BackupBucketConfig</a>)
</p>

<p>
RotationConfig controls the behavior for the rotation of storage account keys.
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
<code>rotationPeriodDays</code></br>
<em>
integer
</em>
</td>
<td>
<p>RotationPeriod is the period after the creation of the currently used key, that a key rotation will be triggered.</p>
</td>
</tr>
<tr>
<td>
<code>expirationPeriodDays</code></br>
<em>
integer
</em>
</td>
<td>
<p>ExpirationPeriod sets the policy on the storage account to expire stale storage account keys. Can only be configured if `rotationPeriod` is configured.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="routetable">RouteTable
</h3>


<p>
(<em>Appears on:</em><a href="#infrastructurestatus">InfrastructureStatus</a>)
</p>

<p>
RouteTable is the azure route table
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
<a href="#purpose">Purpose</a>
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


<h3 id="securitygroup">SecurityGroup
</h3>


<p>
(<em>Appears on:</em><a href="#infrastructurestatus">InfrastructureStatus</a>)
</p>

<p>
SecurityGroup contains information about the security group
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
<a href="#purpose">Purpose</a>
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


<h3 id="storage">Storage
</h3>


<p>
(<em>Appears on:</em><a href="#controlplaneconfig">ControlPlaneConfig</a>)
</p>

<p>
Storage contains configuration for storage in the cluster.
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
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>ManagedDefaultStorageClass controls if the 'default' StorageClass would be marked as default. Set to false to<br />manually set the default to another class not managed by Gardener.<br />Defaults to true.</p>
</td>
</tr>
<tr>
<td>
<code>managedDefaultVolumeSnapshotClass</code></br>
<em>
boolean
</em>
</td>
<td>
<em>(Optional)</em>
<p>ManagedDefaultVolumeSnapshotClass controls if the 'default' VolumeSnapshotClass would be marked as default.<br />Set to false to manually set the default to another class not managed by Gardener.<br />Defaults to true.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="subnet">Subnet
</h3>


<p>
(<em>Appears on:</em><a href="#networkstatus">NetworkStatus</a>)
</p>

<p>
Subnet is a subnet that was created.
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
<a href="#purpose">Purpose</a>
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
boolean
</em>
</td>
<td>
<p>Migrated is set when the network layout is migrated from NetworkLayoutSingleSubnet to NetworkLayoutMultipleSubnet.<br />Only the subnet that was used prior to the migration should have this attribute set.</p>
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


<h3 id="vnet">VNet
</h3>


<p>
(<em>Appears on:</em><a href="#networkconfig">NetworkConfig</a>)
</p>

<p>
VNet contains information about the VNet and some related resources.
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


<h3 id="vnetstatus">VNetStatus
</h3>


<p>
(<em>Appears on:</em><a href="#networkstatus">NetworkStatus</a>)
</p>

<p>
VNetStatus contains the VNet name.
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


<h3 id="vmodependency">VmoDependency
</h3>


<p>
(<em>Appears on:</em><a href="#workerstatus">WorkerStatus</a>)
</p>

<p>
VmoDependency is dependency reference for a workerpool to a VirtualMachineScaleSet Orchestration Mode VM (VMO).
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


<h3 id="volume">Volume
</h3>


<p>
(<em>Appears on:</em><a href="#workerconfig">WorkerConfig</a>)
</p>

<p>
Volume contains configuration for the root disk of a VM.
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
<code>caching</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Caching specifies the caching type for the OS disk.<br />Valid values are 'None', 'ReadOnly', and 'ReadWrite'.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="workerconfig">WorkerConfig
</h3>


<p>
WorkerConfig contains configuration settings for the worker nodes.
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
<code>nodeTemplate</code></br>
<em>
<a href="#nodetemplate">NodeTemplate</a>
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
<a href="#diagnosticsprofile">DiagnosticsProfile</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>DiagnosticsProfile specifies boot diagnostic options.</p>
</td>
</tr>
<tr>
<td>
<code>volume</code></br>
<em>
<a href="#volume">Volume</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Volume contains configuration for the root (OS) disk of a VM.</p>
</td>
</tr>
<tr>
<td>
<code>dataVolumes</code></br>
<em>
<a href="#datavolume">DataVolume</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>DataVolumes contains configuration for the additional disks attached to VMs.</p>
</td>
</tr>
<tr>
<td>
<code>capacityReservation</code></br>
<em>
<a href="#capacityreservation">CapacityReservation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CapacityReservation represents the configuration for capacity reservations on Azure.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="workerstatus">WorkerStatus
</h3>


<p>
WorkerStatus contains information about created worker resources.
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
<code>machineImages</code></br>
<em>
<a href="#machineimage">MachineImage</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>MachineImages is a list of machine images that have been used in this worker. Usually, the extension controller<br />gets the mapping from name/version to the provider-specific machine image data in its componentconfig. However, if<br />a version that is still in use gets removed from this componentconfig it cannot reconcile anymore existing `Worker`<br />resources that are still using this version. Hence, it stores the used versions in the provider status to ensure<br />reconciliation is possible.</p>
</td>
</tr>
<tr>
<td>
<code>vmoDependencies</code></br>
<em>
<a href="#vmodependency">VmoDependency</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>VmoDependencies is a list of external VirtualMachineScaleSet Orchestration Mode VM (VMO) dependencies.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="workloadidentityconfig">WorkloadIdentityConfig
</h3>


<p>
WorkloadIdentityConfig contains configuration settings for workload identity.
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


<h3 id="zone">Zone
</h3>


<p>
(<em>Appears on:</em><a href="#networkconfig">NetworkConfig</a>)
</p>

<p>
Zone describes the configuration for a subnet that is used for VMs on that region.
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
integer
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
<p>CIDR is the CIDR range used for the zone's subnet.</p>
</td>
</tr>
<tr>
<td>
<code>serviceEndpoints</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>ServiceEndpoints is a list of Azure ServiceEndpoints which should be associated with the zone's subnet.</p>
</td>
</tr>
<tr>
<td>
<code>natGateway</code></br>
<em>
<a href="#zonednatgatewayconfig">ZonedNatGatewayConfig</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>NatGateway contains the configuration for the NatGateway associated with this subnet.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="zonednatgatewayconfig">ZonedNatGatewayConfig
</h3>


<p>
(<em>Appears on:</em><a href="#zone">Zone</a>)
</p>

<p>
ZonedNatGatewayConfig contains configuration for NAT gateway and the attached resources.
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
boolean
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
integer
</em>
</td>
<td>
<em>(Optional)</em>
<p>IdleConnectionTimeoutMinutes specifies the idle connection timeout limit for NAT gateway in minutes.</p>
</td>
</tr>
<tr>
<td>
<code>sku</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SKU specifies the SKU of the NAT gateway.<br />Supported values: "Standard"<br />StandardV2 NAT Gateway is zone-redundant and can only be configured at the network level<br />(spec.networks.natGateway), not per-zone.<br />If not specified, defaults to "Standard".</p>
</td>
</tr>
<tr>
<td>
<code>ipAddresses</code></br>
<em>
<a href="#zonedpublicipreference">ZonedPublicIPReference</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>IPAddresses is a list of ip addresses which should be assigned to the NAT gateway.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="zonedpublicipreference">ZonedPublicIPReference
</h3>


<p>
(<em>Appears on:</em><a href="#zonednatgatewayconfig">ZonedNatGatewayConfig</a>)
</p>

<p>
ZonedPublicIPReference contains information about a public ip.
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


