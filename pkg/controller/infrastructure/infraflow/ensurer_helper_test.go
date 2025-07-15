// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
