//  Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package infraflow_test

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow/shared"
)

var _ = Describe("Inventory", func() {
	var (
		wb           shared.Whiteboard
		subscription = "subscription"
		rgName       = "rg"
		vnetName     = "vnet"
		subnet1Name  = "subnet1"
		subnet2Name  = "subnet2"
		rgId         string
		vnetId       string
		subnet1Id    string
		subnet2Id    string
		inventory    *infraflow.Inventory
	)
	BeforeEach(func() {
		wb = shared.NewWhiteboard()
		inventory = infraflow.NewSimpleInventory(wb)
		rgId = infraflow.ResourceGroupIdFromTemplate(subscription, "rg")
		vnetId = infraflow.GetIdFromTemplate(infraflow.TemplateVirtualNetwork, subscription, rgName, vnetName)
		subnet1Id = infraflow.GetIdFromTemplateWithParent(infraflow.TemplateSubnet, subscription, rgName, vnetName, subnet1Name)
		subnet2Id = infraflow.GetIdFromTemplateWithParent(infraflow.TemplateSubnet, subscription, rgName, vnetName, subnet2Name)
	})

	Describe("Insert", func() {
		It("should correctly insert all ids to inventory", func() {
			Expect(inventory.Insert(rgId)).NotTo(HaveOccurred())
			Expect(inventory.Insert(vnetId)).NotTo(HaveOccurred())
			Expect(inventory.Insert(subnet1Id)).NotTo(HaveOccurred())
			Expect(inventory.Insert(subnet2Id)).NotTo(HaveOccurred())

			board := wb.GetChild(infraflow.ChildKeyInventory)
			Expect(board.GetObject(rgId)).NotTo(BeNil())
			Expect(board.GetObject(vnetId)).NotTo(BeNil())
			Expect(board.GetObject(subnet1Id)).NotTo(BeNil())
			Expect(board.GetObject(subnet2Id)).NotTo(BeNil())

			obj := board.GetObject(subnet2Id).(*arm.ResourceID)
			Expect(obj.Name).To(Equal(subnet2Name))
			Expect(obj.ResourceType.String()).To(Equal(infraflow.KindSubnet.String()))
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			Expect(inventory.Insert(rgId)).NotTo(HaveOccurred())
			Expect(inventory.Insert(vnetId)).NotTo(HaveOccurred())
			Expect(inventory.Insert(subnet1Id)).NotTo(HaveOccurred())
			Expect(inventory.Insert(subnet2Id)).NotTo(HaveOccurred())
		})

		It("Should correctly delete the subnetID", func() {
			inventory.Delete(subnet1Id)
			Expect(inventory.Get(subnet1Id)).To(BeNil())
			Expect(inventory.Get(subnet2Id)).NotTo(BeNil())
		})

		It("Should correctly delete the virtual network", func() {
			inventory.Delete(vnetId)

			Expect(inventory.Get(subnet1Id)).To(BeNil())
			Expect(inventory.Get(subnet2Id)).To(BeNil())
			Expect(inventory.Get(vnetId)).To(BeNil())
			Expect(inventory.Get(rgId)).NotTo(BeNil())
		})

		It("Should correctly delete the resource group", func() {
			inventory.Delete(rgId)

			Expect(inventory.Get(subnet1Id)).To(BeNil())
			Expect(inventory.Get(subnet2Id)).To(BeNil())
			Expect(inventory.Get(vnetId)).To(BeNil())
			Expect(inventory.Get(rgId)).To(BeNil())
		})
	})
})
