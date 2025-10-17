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
)
