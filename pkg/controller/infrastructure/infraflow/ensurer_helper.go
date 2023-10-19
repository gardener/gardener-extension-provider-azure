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

import (
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow/shared"
)

// GetObject returns the object and attempts to cast it to the specified type.
func GetObject[T any](wb shared.Whiteboard, key string) T {
	if ok := wb.HasObject(key); !ok {
		return *new(T)
	}
	o := wb.GetObject(key)
	return o.(T)
}

// Filter filters the given array based on the provided functions.
func Filter[T any](arr []T, fs ...func(T) bool) []T {
	var res []T
	for _, t := range arr {
		func() {
			for _, f := range fs {
				if !f(t) {
					return
				}
			}
			res = append(res, t)
		}()
	}
	return res
}

// ToMap converts an array into a map. The key is provided by applying "f" to the array objects and the value are the objects.
func ToMap[T comparable, Y comparable](arr []T, f func(T) Y) map[Y]T {
	res := map[Y]T{}
	for _, t := range arr {
		key := f(t)
		// if key is with default value e.g. ""
		if key == *(new(Y)) {
			continue
		}
		res[key] = t
	}

	return res
}

// Join merges maps by appending m2 to m1.
func Join[K comparable, V any](m1, m2 map[K]V) map[K]V {
	if m2 == nil {
		return m1
	}
	if m1 == nil {
		m1 = make(map[K]V)
	}

	for k, v := range m2 {
		m1[k] = v
	}
	return m1
}

// SimpleInventory is responsible for managing a list of all infrastructure created objects.
type SimpleInventory struct {
	sync.Mutex
	inventory map[string]*arm.ResourceID
}

// NewSimpleInventory returns a new instance of SimpleInventory.
func NewSimpleInventory() *SimpleInventory {
	return &SimpleInventory{
		inventory: map[string]*arm.ResourceID{},
	}
}

// Insert inserts the id to the inventory.
func (i *SimpleInventory) Insert(id string) error {
	i.Lock()
	defer i.Unlock()
	resourceID, err := arm.ParseResourceID(id)
	if err != nil {
		return err
	}

	i.inventory[id] = resourceID
	return nil
}

// Delete deletes the item with ID==id from the inventory and any children it may have. That means that it deletes any ID prefixed by id,
// since azure IDs are hierarchical.
func (i *SimpleInventory) Delete(id string) {
	i.Lock()
	defer i.Unlock()
	delete(i.inventory, id)

	// since azure IDs are hierarchical, we remove from our inventory all items that are prefixed by the Id we want to remove.
	for k := range i.inventory {
		if strings.HasPrefix(k, id) {
			delete(i.inventory, id)
		}
	}
}

// ByKind returns a list of all the IDs of stored objects of a particular kind.
func (i *SimpleInventory) ByKind(kind AzureResourceKind) []string {
	i.Lock()
	defer i.Unlock()
	res := make([]string, 0)
	for _, v := range i.inventory {
		if v.ResourceType.String() == kind.String() {
			res = append(res, v.Name)

		}
	}

	return res
}

// ToList returns a list of v1alpha1 API objects that correspond to the current inventory list.
func (i *SimpleInventory) ToList() []v1alpha1.AzureResource {
	i.Lock()
	defer i.Unlock()

	var res []v1alpha1.AzureResource
	for k, v := range i.inventory {
		res = append(res, v1alpha1.AzureResource{
			Kind: v.ResourceType.String(),
			ID:   k,
		})
	}
	return res
}
