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

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	mockazureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client/mock"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-05-01/network"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		var azureGroupClient *mockazureclient.MockGroup

		BeforeEach(func() {
			azureGroupClient = mockazureclient.NewMockGroup(ctrl)
			infraConfig.ResourceGroup = nil
		})

		It("should return true as resource group is not managed", func() {
			infraConfig.ResourceGroup = &api.ResourceGroup{Name: "test"}
			resourceGroupAvailable, err := IsShootResourceGroupAvailable(ctx, azureClientFactory, infra, infraConfig)

			Expect(err).NotTo(HaveOccurred())
			Expect(resourceGroupAvailable).To(BeTrue())
		})

		It("should return true as resource group exists", func() {
			azureClientFactory.EXPECT().Group(ctx, infra.Spec.SecretRef).Return(azureGroupClient, nil)
			azureGroupClient.EXPECT().Get(ctx, clusterName).Return(&resources.Group{Name: &clusterName}, nil)

			resourceGroupAvailable, err := IsShootResourceGroupAvailable(ctx, azureClientFactory, infra, infraConfig)

			Expect(err).NotTo(HaveOccurred())
			Expect(resourceGroupAvailable).To(BeTrue())
		})

		It("should return false as resource group does not exists", func() {
			azureClientFactory.EXPECT().Group(ctx, infra.Spec.SecretRef).Return(azureGroupClient, nil)
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
			subnetList        []network.Subnet
			vnetName          string
			vnetResourceGroup string
		)

		BeforeEach(func() {
			azureSubnetClient = mockazureclient.NewMockSubnet(ctrl)
			baseSubnetName = fmt.Sprintf("%s-nodes", clusterName)
			subnetList = []network.Subnet{}
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
			subnetList = append(subnetList, network.Subnet{Name: &baseSubnetName})

			azureClientFactory.EXPECT().Subnet(ctx, infra.Spec.SecretRef).Return(azureSubnetClient, nil)
			azureSubnetClient.EXPECT().List(ctx, vnetResourceGroup, vnetName).Return(subnetList, nil)
			azureSubnetClient.EXPECT().Delete(ctx, vnetResourceGroup, vnetName, baseSubnetName)

			err := DeleteNodeSubnetIfExists(ctx, azureClientFactory, infra, infraConfig)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete name matching subnet in foreign virtual network", func() {
			subnetName := fmt.Sprintf("%s-z2", baseSubnetName)
			subnetList = append(subnetList, network.Subnet{Name: &subnetName})

			azureClientFactory.EXPECT().Subnet(ctx, infra.Spec.SecretRef).Return(azureSubnetClient, nil)
			azureSubnetClient.EXPECT().List(ctx, vnetResourceGroup, vnetName).Return(subnetList, nil)
			azureSubnetClient.EXPECT().Delete(ctx, vnetResourceGroup, vnetName, subnetName)

			err := DeleteNodeSubnetIfExists(ctx, azureClientFactory, infra, infraConfig)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete no subnet in foreign virtual network as none is name matching", func() {
			subnetName := "test-abc"
			subnetList = append(subnetList, network.Subnet{Name: &subnetName})

			azureClientFactory.EXPECT().Subnet(ctx, infra.Spec.SecretRef).Return(azureSubnetClient, nil)
			azureSubnetClient.EXPECT().List(ctx, vnetResourceGroup, vnetName).Return(subnetList, nil)

			err := DeleteNodeSubnetIfExists(ctx, azureClientFactory, infra, infraConfig)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
