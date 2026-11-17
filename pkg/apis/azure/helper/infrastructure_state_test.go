// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
)

func TestInfrastructureStateFromRaw(t *testing.T) {
	const resourceGroupID = "/subscriptions/sub/resourceGroups/shoot"
	nativeState := &v1alpha1.InfrastructureState{
		TypeMeta: metav1.TypeMeta{APIVersion: v1alpha1.SchemeGroupVersion.String(), Kind: "InfrastructureState"},
		Data: map[string]string{
			"ids|Microsoft.Resources/resourceGroups": resourceGroupID,
			"resources_exist":                        "true",
		},
		ManagedItems: []v1alpha1.AzureResource{{Kind: "Microsoft.Resources/resourceGroups", ID: resourceGroupID}},
	}
	managedItems := []api.AzureResource{{Kind: "Microsoft.Resources/resourceGroups", ID: resourceGroupID}}
	for _, tc := range []struct {
		name         string
		raw          *runtime.RawExtension
		data         map[string]string
		managedItems []api.AzureResource
		wantErr      bool
	}{
		{name: "absent state", data: map[string]string{}, managedItems: []api.AzureResource{}},
		{name: "empty raw extension", raw: &runtime.RawExtension{}, data: map[string]string{}, managedItems: []api.AzureResource{}},
		{name: "empty raw bytes", raw: &runtime.RawExtension{Raw: []byte{}}, data: map[string]string{}, managedItems: []api.AzureResource{}},
		{
			name: "raw state preserves data and inventory and ignores unknown fields",
			raw:  &runtime.RawExtension{Raw: []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1","kind":"InfrastructureState","data":{"ids|Microsoft.Resources/resourceGroups":"/subscriptions/sub/resourceGroups/shoot","resources_exist":"true"},"managedItems":[{"kind":"Microsoft.Resources/resourceGroups","id":"/subscriptions/sub/resourceGroups/shoot"}],"futureField":true}`)},
			data: nativeState.Data, managedItems: managedItems,
		},
		{name: "object-backed state preserves data and inventory", raw: &runtime.RawExtension{Object: nativeState}, data: nativeState.Data, managedItems: managedItems},
		{
			name: "raw bytes take precedence over object",
			raw: &runtime.RawExtension{
				Raw:    []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1","kind":"InfrastructureState","data":{"resources_exist":"true"}}`),
				Object: nativeState,
			},
			data: map[string]string{"resources_exist": "true"}, managedItems: []api.AzureResource{},
		},
		{name: "native state without data or inventory", raw: &runtime.RawExtension{Raw: []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1","kind":"InfrastructureState"}`)}, data: map[string]string{}, managedItems: []api.AzureResource{}},
		{name: "invalid JSON", raw: &runtime.RawExtension{Raw: []byte(`invalid`)}, wantErr: true},
		{name: "object cannot be marshalled", raw: &runtime.RawExtension{Object: &runtime.Unknown{Raw: []byte(`invalid`)}}, wantErr: true},
		{name: "missing kind", raw: &runtime.RawExtension{Raw: []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1","data":{"resources_exist":"true"}}`)}, wantErr: true},
		{name: "missing API version", raw: &runtime.RawExtension{Raw: []byte(`{"kind":"InfrastructureState","data":{"resources_exist":"true"}}`)}, wantErr: true},
		{name: "legacy Terraform state", raw: &runtime.RawExtension{Raw: []byte(`{"terraformState":{"data":"{}","encoding":"none"},"savedProviderStatus":{}}`)}, wantErr: true},
		{name: "unsupported API version", raw: &runtime.RawExtension{Raw: []byte(`{"apiVersion":"example.com/v1","kind":"InfrastructureState"}`)}, wantErr: true},
		{name: "different registered kind", raw: &runtime.RawExtension{Raw: []byte(`{"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1","kind":"InfrastructureConfig"}`)}, wantErr: true},
		{
			name: "different registered object kind",
			raw: &runtime.RawExtension{Object: &v1alpha1.InfrastructureConfig{
				TypeMeta: metav1.TypeMeta{APIVersion: v1alpha1.SchemeGroupVersion.String(), Kind: "InfrastructureConfig"},
			}},
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			state, err := helper.InfrastructureStateFromRaw(tc.raw)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(state).To(BeNil())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(state).To(Equal(&api.InfrastructureState{Data: tc.data, ManagedItems: tc.managedItems}))
			}
		})
	}
}
