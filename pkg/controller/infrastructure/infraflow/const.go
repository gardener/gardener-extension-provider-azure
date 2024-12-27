// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
	// ChildKeyComplete is a key to indicate whether a task is complete.
	ChildKeyComplete = "complete"

	// IgnoredByGardenerTag is the tag used to mark resources managed by Gardener.
	// It's used for an edge case where public IPs that are customer managed + migrated
	// from terraform + reside in our RG + have the shoot prefix name.
	IgnoredByGardenerTag = "managedByGardener"
)
