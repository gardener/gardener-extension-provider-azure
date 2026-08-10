// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

const (
	// ChildKeyIDs is the prefix key for all ids.
	ChildKeyIDs = "ids"
	// ChildKeyInventory is the prefix key for for the inventory struct.
	ChildKeyInventory = "inventory"
	// CreatedResourcesExistKey is a marker for the Terraform migration case. If the TF state is not empty
	// we inject this marker into the state to block the deletion without having first a successful reconciliation.
	CreatedResourcesExistKey = "resources_exist"

	// KeyManagedIdentityClientId is a key for the MI's client ID.
	KeyManagedIdentityClientId = "managed_identity_client_id"
	// KeyManagedIdentityId is a key for the MI's identity ID.
	KeyManagedIdentityId = "managed_identity_id"

	// ChildKeyMigration is the prefix key for data stored during migrations.
	ChildKeyMigration = "migration"

	// TagManagedByGardener is the tag used to mark resources managed by Gardener.
	TagManagedByGardener = "managed-by-gardener"
	// TagShootName is the tag used to mark the shoot name on resources managed by Gardener.
	TagShootName = "gardener-shoot-name"

	// ChildKeyBYO is the prefix key used to persist BYO-subnet discovery data in the whiteboard so
	// the delete flow can act deterministically (e.g. remove observability tags) without having to
	// re-read the subnet.
	ChildKeyBYO = "byo"
	// KeyBYONSGName is the name of the network security group discovered on the BYO subnet.
	KeyBYONSGName = "nsgName"
	// KeyBYONSGResourceGroup is the resource group hosting the discovered BYO NSG.
	KeyBYONSGResourceGroup = "nsgResourceGroup"
	// KeyBYONSGID is the ARM resource ID of the discovered BYO NSG.
	KeyBYONSGID = "nsgID"
	// KeyBYORTName is the name of the route table discovered on the BYO subnet (empty if the subnet
	// has no route table attached, which is only accepted when the shoot uses an overlay CNI).
	KeyBYORTName = "rtName"
	// KeyBYORTResourceGroup is the resource group hosting the discovered BYO route table.
	KeyBYORTResourceGroup = "rtResourceGroup"
	// KeyBYORTID is the ARM resource ID of the discovered BYO route table.
	KeyBYORTID = "rtID"
	// KeyBYOSubnetCIDR is the address prefix of the discovered BYO subnet.
	KeyBYOSubnetCIDR = "subnetCIDR"
	// KeyBYOVNetTagged marks whether the observability tag has been successfully applied to the
	// BYO VNet by this shoot. Used to drive best-effort tag removal on shoot deletion.
	KeyBYOVNetTagged = "vnetTagged"
	// KeyBYONSGTagged marks whether the observability tag has been successfully applied to the
	// BYO NSG by this shoot.
	KeyBYONSGTagged = "nsgTagged"
	// KeyBYORTTagged marks whether the observability tag has been successfully applied to the
	// BYO route table by this shoot.
	KeyBYORTTagged = "rtTagged"

	// TagKeyClusterPrefix is the prefix used for the observability tag applied to BYO resources:
	// full form is `kubernetes.io/cluster/<shootTechnicalName>`.
	TagKeyClusterPrefix = "kubernetes.io/cluster/"
	// TagValueShared is the value used for the shoot-cluster observability tag on BYO resources.
	TagValueShared = "shared"
)
