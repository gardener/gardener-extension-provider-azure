//  Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

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
)
