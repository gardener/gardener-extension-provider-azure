// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package infrastructure_test

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	mockazureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client/mock"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

const (
	infraName               = "azure-infra"
	cloudproviderSecretName = "cloudprovider"
)

var (
	azureClientFactory *mockazureclient.MockFactory
	clusterName        string
	ctrl               *gomock.Controller
	ctx                context.Context
	infra              *extensionsv1alpha1.Infrastructure
	infraConfig        *api.InfrastructureConfig
	infraNamespace     string
)

var _ = Describe("InfrastructureHelper", func() {
	BeforeEach(func() {
		ctx = context.TODO()
		ctrl = gomock.NewController(GinkgoT())
		azureClientFactory = mockazureclient.NewMockFactory(ctrl)

		clusterName = "shoot-test-az"
		infraNamespace = clusterName

		infraConfig = &api.InfrastructureConfig{}
		infra = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Name:      infraName,
				Namespace: infraNamespace,
			},
			Spec: extensionsv1alpha1.InfrastructureSpec{
				SecretRef: corev1.SecretReference{
					Name:      cloudproviderSecretName,
					Namespace: infraNamespace,
				},
			},
		}
	})

	Describe("#IsShootResourceGroupAvailable", func() {
		var azureGroupClient *mockazureclient.MockResourceGroup

		BeforeEach(func() {
			azureGroupClient = mockazureclient.NewMockResourceGroup(ctrl)
			infraConfig.ResourceGroup = nil
		})

		It("should return true as resource group is not managed", func() {
			infraConfig.ResourceGroup = &api.ResourceGroup{Name: "test"}
			resourceGroupAvailable, err := IsShootResourceGroupAvailable(ctx, azureClientFactory, infra, infraConfig)

			Expect(err).NotTo(HaveOccurred())
			Expect(resourceGroupAvailable).To(BeTrue())
		})

		It("should return true as resource group exists", func() {
			azureClientFactory.EXPECT().Group().Return(azureGroupClient, nil)
			azureGroupClient.EXPECT().Get(ctx, clusterName).Return(&armresources.ResourceGroup{Name: &clusterName}, nil)

			resourceGroupAvailable, err := IsShootResourceGroupAvailable(ctx, azureClientFactory, infra, infraConfig)

			Expect(err).NotTo(HaveOccurred())
			Expect(resourceGroupAvailable).To(BeTrue())
		})

		It("should return false as resource group does not exists", func() {
			azureClientFactory.EXPECT().Group().Return(azureGroupClient, nil)
			azureGroupClient.EXPECT().Get(ctx, clusterName).Return(nil, nil)

			resourceGroupAvailable, err := IsShootResourceGroupAvailable(ctx, azureClientFactory, infra, infraConfig)

			Expect(err).NotTo(HaveOccurred())
			Expect(resourceGroupAvailable).To(BeFalse())
		})
	})

	Describe("#DeleteNodeSubnetIfExists", func() {
		var (
			azureSubnetClient *mockazureclient.MockSubnet
			baseSubnetName    string
			subnetList        []*armnetwork.Subnet
			vnetName          string
			vnetResourceGroup string
		)

		BeforeEach(func() {
			azureSubnetClient = mockazureclient.NewMockSubnet(ctrl)
			baseSubnetName = fmt.Sprintf("%s-nodes", clusterName)
			subnetList = []*armnetwork.Subnet{}
			vnetName = "test-vnet"
			vnetResourceGroup = "test-vnet-rg"

			infraConfig.Networks = api.NetworkConfig{
				VNet: api.VNet{
					Name:          &vnetName,
					ResourceGroup: &vnetResourceGroup,
				},
			}
		})

		It("should abort as infra is not using a foreign a virtual network", func() {
			infraConfig.Networks.VNet = api.VNet{}

			err := DeleteNodeSubnetIfExists(ctx, azureClientFactory, infra, infraConfig)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete exact name matching subnet in foreign virtual network", func() {
			subnetList = append(subnetList, &armnetwork.Subnet{Name: &baseSubnetName})

			azureClientFactory.EXPECT().Subnet().Return(azureSubnetClient, nil)
			azureSubnetClient.EXPECT().List(ctx, vnetResourceGroup, vnetName).Return(subnetList, nil)
			azureSubnetClient.EXPECT().Delete(ctx, vnetResourceGroup, vnetName, baseSubnetName)

			err := DeleteNodeSubnetIfExists(ctx, azureClientFactory, infra, infraConfig)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete name matching subnet in foreign virtual network", func() {
			subnetName := fmt.Sprintf("%s-z2", baseSubnetName)
			subnetList = append(subnetList, &armnetwork.Subnet{Name: &subnetName})

			azureClientFactory.EXPECT().Subnet().Return(azureSubnetClient, nil)
			azureSubnetClient.EXPECT().List(ctx, vnetResourceGroup, vnetName).Return(subnetList, nil)
			azureSubnetClient.EXPECT().Delete(ctx, vnetResourceGroup, vnetName, subnetName)

			err := DeleteNodeSubnetIfExists(ctx, azureClientFactory, infra, infraConfig)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete no subnet in foreign virtual network as none is name matching", func() {
			subnetName := "test-abc"
			subnetList = append(subnetList, &armnetwork.Subnet{Name: &subnetName})

			azureClientFactory.EXPECT().Subnet().Return(azureSubnetClient, nil)
			azureSubnetClient.EXPECT().List(ctx, vnetResourceGroup, vnetName).Return(subnetList, nil)

			err := DeleteNodeSubnetIfExists(ctx, azureClientFactory, infra, infraConfig)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
