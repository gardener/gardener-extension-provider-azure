// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v9"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
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

var _ = Describe("ensurePublicIPSKU", func() {
	It("should return 'Standard' when both sku and natSku are nil", func() {
		result := ensurePublicIPSKU(nil, nil)
		Expect(result).ToNot(BeNil())
		Expect(*result).To(Equal(string(armnetwork.PublicIPAddressSKUNameStandard)))
	})

	It("should return 'Standard' when sku is nil and natSku is 'Standard'", func() {
		natSku := string(armnetwork.NatGatewaySKUNameStandard)
		result := ensurePublicIPSKU(nil, &natSku)
		Expect(result).ToNot(BeNil())
		Expect(*result).To(Equal(string(armnetwork.PublicIPAddressSKUNameStandard)))
	})

	It("should return 'StandardV2' when sku is nil and natSku is 'StandardV2'", func() {
		natSku := string(armnetwork.NatGatewaySKUNameStandardV2)
		result := ensurePublicIPSKU(nil, &natSku)
		Expect(result).ToNot(BeNil())
		Expect(*result).To(Equal(string(armnetwork.PublicIPAddressSKUNameStandardV2)))
	})

	It("should return explicit sku when sku is set, even if natSku differs", func() {
		sku := string(armnetwork.PublicIPAddressSKUNameStandardV2)
		natSku := string(armnetwork.NatGatewaySKUNameStandard)
		result := ensurePublicIPSKU(&sku, &natSku)
		Expect(result).ToNot(BeNil())
		Expect(*result).To(Equal(string(armnetwork.PublicIPAddressSKUNameStandardV2)))
	})
})

var _ = Describe("InfrastructureAdapter defaultZone with StandardV2", func() {
	var (
		infra   *extensionsv1alpha1.Infrastructure
		profile *azure.CloudProfileConfig
		cluster *extensionscontroller.Cluster
	)

	BeforeEach(func() {
		infra = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "shoot--foo--bar",
			},
			Spec: extensionsv1alpha1.InfrastructureSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: "azure",
				},
				Region: "westeurope",
			},
		}
		profile = &azure.CloudProfileConfig{}
		cluster = &extensionscontroller.Cluster{
			Shoot: &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
		}
	})

	It("should set StandardV2 SKU on the NAT gateway and no zone", func() {
		workers := "10.250.0.0/19"
		config := &azure.InfrastructureConfig{
			Networks: azure.NetworkConfig{
				Workers: &workers,
				VNet:    azure.VNet{CIDR: ptr.To("10.250.0.0/16")},
				NatGateway: &azure.NatGatewayConfig{
					Enabled: true,
					SKU:     ptr.To("StandardV2"),
				},
			},
			Zoned: true,
		}

		adapter, err := NewInfrastructureAdapter(infra, config, nil, profile, cluster)
		Expect(err).ToNot(HaveOccurred())

		zones := adapter.Zones()
		Expect(zones).To(HaveLen(1))
		Expect(zones[0].NatGateway).ToNot(BeNil())
		Expect(*zones[0].NatGateway.SKU).To(Equal("StandardV2"))
		Expect(zones[0].NatGateway.Zone).To(BeNil())
	})

	It("should set StandardV2 SKU on managed public IPs", func() {
		workers := "10.250.0.0/19"
		config := &azure.InfrastructureConfig{
			Networks: azure.NetworkConfig{
				Workers: &workers,
				VNet:    azure.VNet{CIDR: ptr.To("10.250.0.0/16")},
				NatGateway: &azure.NatGatewayConfig{
					Enabled: true,
					SKU:     ptr.To("StandardV2"),
				},
			},
			Zoned: true,
		}

		adapter, err := NewInfrastructureAdapter(infra, config, nil, profile, cluster)
		Expect(err).ToNot(HaveOccurred())

		zones := adapter.Zones()
		Expect(zones[0].NatGateway.PublicIPList).To(HaveLen(1))
		Expect(*zones[0].NatGateway.PublicIPList[0].SKU).To(Equal(string(armnetwork.PublicIPAddressSKUNameStandardV2)))
		Expect(zones[0].NatGateway.PublicIPList[0].Managed).To(BeTrue())
	})

	It("should set Standard SKU when NAT gateway SKU is nil (default)", func() {
		workers := "10.250.0.0/19"
		config := &azure.InfrastructureConfig{
			Networks: azure.NetworkConfig{
				Workers: &workers,
				VNet:    azure.VNet{CIDR: ptr.To("10.250.0.0/16")},
				NatGateway: &azure.NatGatewayConfig{
					Enabled: true,
					Zone:    ptr.To[int32](1),
				},
			},
			Zoned: true,
		}

		adapter, err := NewInfrastructureAdapter(infra, config, nil, profile, cluster)
		Expect(err).ToNot(HaveOccurred())

		zones := adapter.Zones()
		Expect(zones).To(HaveLen(1))
		Expect(zones[0].NatGateway).ToNot(BeNil())
		Expect(*zones[0].NatGateway.SKU).To(Equal("Standard"))
		Expect(zones[0].NatGateway.Zone).To(Equal(ptr.To("1")))
	})

	It("should propagate StandardV2 SKU to user-provided public IPs", func() {
		workers := "10.250.0.0/19"
		config := &azure.InfrastructureConfig{
			Networks: azure.NetworkConfig{
				Workers: &workers,
				VNet:    azure.VNet{CIDR: ptr.To("10.250.0.0/16")},
				NatGateway: &azure.NatGatewayConfig{
					Enabled: true,
					SKU:     ptr.To("StandardV2"),
					IPAddresses: []azure.PublicIPReference{
						{
							Name:          "my-ip",
							ResourceGroup: "my-rg",
							SKU:           ptr.To("StandardV2"),
						},
					},
				},
			},
			Zoned: true,
		}

		adapter, err := NewInfrastructureAdapter(infra, config, nil, profile, cluster)
		Expect(err).ToNot(HaveOccurred())

		zones := adapter.Zones()
		Expect(zones[0].NatGateway.PublicIPList).To(HaveLen(1))
		Expect(*zones[0].NatGateway.PublicIPList[0].SKU).To(Equal(string(armnetwork.PublicIPAddressSKUNameStandardV2)))
		Expect(zones[0].NatGateway.PublicIPList[0].Managed).To(BeFalse())
		Expect(zones[0].NatGateway.PublicIPList[0].Name).To(Equal("my-ip"))
	})
})
