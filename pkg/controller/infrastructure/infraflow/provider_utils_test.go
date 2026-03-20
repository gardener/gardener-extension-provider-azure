// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v9"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ForceNewIp", func() {
	var (
		baseCurrent func() *armnetwork.PublicIPAddress
		baseTarget  func() *armnetwork.PublicIPAddress
	)

	BeforeEach(func() {
		baseCurrent = func() *armnetwork.PublicIPAddress {
			return &armnetwork.PublicIPAddress{
				Location: to.Ptr("westeurope"),
				Zones:    []*string{to.Ptr("1")},
				Properties: &armnetwork.PublicIPAddressPropertiesFormat{
					PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
				},
				SKU: &armnetwork.PublicIPAddressSKU{
					Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard),
				},
			}
		}
		baseTarget = func() *armnetwork.PublicIPAddress {
			return &armnetwork.PublicIPAddress{
				Location: to.Ptr("westeurope"),
				Zones:    []*string{to.Ptr("1")},
				Properties: &armnetwork.PublicIPAddressPropertiesFormat{
					PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
				},
				SKU: &armnetwork.PublicIPAddressSKU{
					Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard),
				},
			}
		}
	})

	It("should NOT force new when both SKUs are the same (Standard)", func() {
		forceNew, _, _ := ForceNewIp(baseCurrent(), baseTarget())
		Expect(forceNew).To(BeFalse())
	})

	It("should force new when SKUs differ (Standard -> StandardV2)", func() {
		current := baseCurrent()
		target := baseTarget()
		target.SKU.Name = to.Ptr(armnetwork.PublicIPAddressSKUNameStandardV2)

		forceNew, field, _ := ForceNewIp(current, target)
		Expect(forceNew).To(BeTrue())
		Expect(field).To(Equal("SKU.Name"))
	})

	It("should force new when current SKU is nil and target SKU is set", func() {
		current := baseCurrent()
		current.SKU = nil
		target := baseTarget()

		forceNew, field, _ := ForceNewIp(current, target)
		Expect(forceNew).To(BeTrue())
		Expect(field).To(Equal("SKU"))
	})

	It("should NOT force new when both SKUs are nil", func() {
		current := baseCurrent()
		current.SKU = nil
		target := baseTarget()
		target.SKU = nil

		forceNew, _, _ := ForceNewIp(current, target)
		Expect(forceNew).To(BeFalse())
	})

	It("should force new when current SKU is set and target SKU is nil", func() {
		current := baseCurrent()
		target := baseTarget()
		target.SKU = nil

		forceNew, field, _ := ForceNewIp(current, target)
		Expect(forceNew).To(BeTrue())
		Expect(field).To(Equal("SKU"))
	})
})

var _ = Describe("ForceNewNat", func() {
	var (
		baseCurrent func() *armnetwork.NatGateway
		baseTarget  func() *armnetwork.NatGateway
	)

	BeforeEach(func() {
		baseCurrent = func() *armnetwork.NatGateway {
			return &armnetwork.NatGateway{
				Location: to.Ptr("westeurope"),
				Zones:    []*string{to.Ptr("1")},
				SKU: &armnetwork.NatGatewaySKU{
					Name: to.Ptr(armnetwork.NatGatewaySKUNameStandard),
				},
			}
		}
		baseTarget = func() *armnetwork.NatGateway {
			return &armnetwork.NatGateway{
				Location: to.Ptr("westeurope"),
				Zones:    []*string{to.Ptr("1")},
				SKU: &armnetwork.NatGatewaySKU{
					Name: to.Ptr(armnetwork.NatGatewaySKUNameStandard),
				},
			}
		}
	})

	It("should NOT force new when both SKUs are the same", func() {
		forceNew, _, _ := ForceNewNat(baseCurrent(), baseTarget())
		Expect(forceNew).To(BeFalse())
	})

	It("should force new when SKUs differ (Standard -> StandardV2)", func() {
		current := baseCurrent()
		target := baseTarget()
		target.SKU.Name = to.Ptr(armnetwork.NatGatewaySKUNameStandardV2)

		forceNew, field, _ := ForceNewNat(current, target)
		Expect(forceNew).To(BeTrue())
		Expect(field).To(Equal("SKU.Name"))
	})

	It("should force new when current SKU is nil and target SKU is set", func() {
		current := baseCurrent()
		current.SKU = nil
		target := baseTarget()

		forceNew, field, _ := ForceNewNat(current, target)
		Expect(forceNew).To(BeTrue())
		Expect(field).To(Equal("SKU"))
	})

	It("should NOT force new when both SKUs are nil", func() {
		current := baseCurrent()
		current.SKU = nil
		target := baseTarget()
		target.SKU = nil

		forceNew, _, _ := ForceNewNat(current, target)
		Expect(forceNew).To(BeFalse())
	})
})

var _ = Describe("ensureNatGatewaySKU", func() {
	It("should return pointer to 'Standard' when input is nil", func() {
		result := ensureNatGatewaySKU(nil)
		Expect(result).ToNot(BeNil())
		Expect(*result).To(Equal(string(armnetwork.NatGatewaySKUNameStandard)))
	})

	It("should return pointer to 'Standard' when input is empty string", func() {
		input := ""
		result := ensureNatGatewaySKU(&input)
		Expect(result).ToNot(BeNil())
		Expect(*result).To(Equal(string(armnetwork.NatGatewaySKUNameStandard)))
	})

	It("should return pointer to 'Standard' when input is 'Standard'", func() {
		input := string(armnetwork.NatGatewaySKUNameStandard)
		result := ensureNatGatewaySKU(&input)
		Expect(result).ToNot(BeNil())
		Expect(*result).To(Equal(string(armnetwork.NatGatewaySKUNameStandard)))
	})

	It("should return pointer to 'StandardV2' when input is 'StandardV2'", func() {
		input := string(armnetwork.NatGatewaySKUNameStandardV2)
		result := ensureNatGatewaySKU(&input)
		Expect(result).ToNot(BeNil())
		Expect(*result).To(Equal(string(armnetwork.NatGatewaySKUNameStandardV2)))
	})
})

var _ = Describe("ensurePublicIpSKU", func() {
	It("should return 'Standard' when both sku and natSku are nil", func() {
		result := ensurePublicIpSKU(nil, nil)
		Expect(result).ToNot(BeNil())
		Expect(*result).To(Equal(string(armnetwork.PublicIPAddressSKUNameStandard)))
	})

	It("should return 'Standard' when sku is nil and natSku is 'Standard'", func() {
		natSku := string(armnetwork.NatGatewaySKUNameStandard)
		result := ensurePublicIpSKU(nil, &natSku)
		Expect(result).ToNot(BeNil())
		Expect(*result).To(Equal(string(armnetwork.PublicIPAddressSKUNameStandard)))
	})

	It("should return 'StandardV2' when sku is nil and natSku is 'StandardV2'", func() {
		natSku := string(armnetwork.NatGatewaySKUNameStandardV2)
		result := ensurePublicIpSKU(nil, &natSku)
		Expect(result).ToNot(BeNil())
		Expect(*result).To(Equal(string(armnetwork.PublicIPAddressSKUNameStandardV2)))
	})

	It("should return explicit sku when sku is set, even if natSku differs", func() {
		sku := string(armnetwork.PublicIPAddressSKUNameStandardV2)
		natSku := string(armnetwork.NatGatewaySKUNameStandard)
		result := ensurePublicIpSKU(&sku, &natSku)
		Expect(result).ToNot(BeNil())
		Expect(*result).To(Equal(string(armnetwork.PublicIPAddressSKUNameStandardV2)))
	})
})
