// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"strings"

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

// Inventory is responsible for managing a list of all infrastructure created objects.
type Inventory struct {
	shared.Whiteboard
}

// NewSimpleInventory returns a new instance of Inventory.
func NewSimpleInventory(wb shared.Whiteboard) *Inventory {
	return &Inventory{wb}
}

// Insert inserts the id to the inventory.
func (i *Inventory) Insert(id string) error {
	resourceID, err := arm.ParseResourceID(id)
	if err != nil {
		return err
	}

	i.GetChild(ChildKeyInventory).SetObject(id, resourceID)
	return nil
}

// Get gets the item from the inventory.
func (i *Inventory) Get(id string) *arm.ResourceID {
	if i.GetChild(ChildKeyInventory).HasObject(id) {
		return i.GetChild(ChildKeyInventory).GetObject(id).(*arm.ResourceID)
	}
	return nil
}

// Delete deletes the item with ID==id from the inventory and any children it may have. That means that it deletes any ID prefixed by id,
// since azure IDs are hierarchical.
func (i *Inventory) Delete(id string) {
	// since azure IDs are hierarchical, we remove from our inventory all items that are prefixed by the Id we want to remove.
	for _, key := range i.GetChild(ChildKeyInventory).ObjectKeys() {
		if strings.HasPrefix(key, id) {
			i.GetChild(ChildKeyInventory).DeleteObject(key)
		}
	}
}

// ByKind returns a list of all the IDs of stored objects of a particular kind.
func (i *Inventory) ByKind(kind AzureResourceKind) []string {
	res := make([]string, 0)
	for _, key := range i.GetChild(ChildKeyInventory).ObjectKeys() {
		if i.GetChild(ChildKeyInventory).HasObject(key) {
			resource := i.GetChild(ChildKeyInventory).GetObject(key).(*arm.ResourceID)
			if resource.ResourceType.String() == kind.String() {
				res = append(res, resource.Name)
			}
		}
	}

	return res
}

// ToList returns a list of v1alpha1 API objects that correspond to the current inventory list.
func (i *Inventory) ToList() []v1alpha1.AzureResource {
	var res []v1alpha1.AzureResource
	for _, key := range i.GetChild(ChildKeyInventory).ObjectKeys() {
		if i.GetChild(ChildKeyInventory).HasObject(key) {
			resource := i.GetChild(ChildKeyInventory).GetObject(key).(*arm.ResourceID)
			res = append(res, v1alpha1.AzureResource{
				Kind: resource.ResourceType.String(),
				ID:   key,
			})
		}
	}
	return res
}
