// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validation_test

import (
	"fmt"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"

	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

var _ = Describe("InfrastructureConfig validation", func() {
	var (
		infrastructureConfig  *apisazure.InfrastructureConfig
		nodes                 string
		resourceGroup         = "shoot--test--foo"
		hasVmoAlphaAnnotation bool

		pods        = "100.96.0.0/11"
		services    = "100.64.0.0/13"
		vnetCIDR    = "10.0.0.0/8"
		invalidCIDR = "invalid-cidr"

		providerPath *field.Path
	)

	BeforeEach(func() {
		nodes = "10.250.0.0/16"
		infrastructureConfig = &apisazure.InfrastructureConfig{
			Networks: apisazure.NetworkConfig{
				Workers: "10.250.3.0/24",
				VNet: apisazure.VNet{
					CIDR: &vnetCIDR,
				},
			},
		}
		hasVmoAlphaAnnotation = false
	})

	Describe("#ValidateInfrastructureConfig", func() {
		It("should forbid specifying a resource group configuration", func() {
			infrastructureConfig.ResourceGroup = &apisazure.ResourceGroup{}

			errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)

			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("resourceGroup"),
			}))
		})

		Context("vnet", func() {
			It("should forbid specifying a vnet name without resource group", func() {
				vnetName := "existing-vnet"
				infrastructureConfig.Networks.VNet = apisazure.VNet{
					Name: &vnetName,
				}
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(ConsistOfFields(
					Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("networks.vnet"),
						"Detail": Equal("a vnet cidr or vnet name and resource group need to be specified"),
					}))
			})

			It("should forbid specifying a vnet resource group without name", func() {
				vnetGroup := "existing-vnet-rg"
				infrastructureConfig.Networks.VNet = apisazure.VNet{
					ResourceGroup: &vnetGroup,
				}
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(ConsistOfFields(
					Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("networks.vnet"),
						"Detail": Equal("a vnet cidr or vnet name and resource group need to be specified"),
					}))
			})

			It("should forbid specifying existing vnet plus a vnet cidr", func() {
				name := "existing-vnet"
				vnetGroup := "existing-vnet-rg"
				infrastructureConfig.Networks.VNet = apisazure.VNet{
					Name:          &name,
					ResourceGroup: &vnetGroup,
					CIDR:          &vnetCIDR,
				}
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(ConsistOfFields(
					Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("networks.vnet.cidr"),
						"Detail": Equal("specifying a cidr for an existing vnet is not possible"),
					}))
			})

			It("should forbid specifying existing vnet in same resource group", func() {
				name := "existing-vnet"
				infrastructureConfig.Networks.VNet = apisazure.VNet{
					Name:          &name,
					ResourceGroup: &resourceGroup,
				}
				infrastructureConfig.ResourceGroup = &apisazure.ResourceGroup{
					Name: resourceGroup,
				}
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(ConsistOfFields(
					Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("resourceGroup"),
					},
					Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("networks.vnet.resourceGroup"),
						"Detail": Equal("the vnet resource group must not be the same as the cluster resource group"),
					}))
			})

			It("should pass if no vnet cidr is specified and default is applied", func() {
				nodes = "10.250.3.0/24"
				infrastructureConfig.ResourceGroup = nil
				infrastructureConfig.Networks = apisazure.NetworkConfig{
					Workers: "10.250.3.0/24",
				}
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)
				Expect(errorList).To(HaveLen(0))
			})
		})

		Context("Zonal", func() {
			It(fmt.Sprintf("should forbid specifying the %q annotation for a zonal cluster", azure.ShootVmoUsageAnnotation), func() {
				infrastructureConfig.Zoned = true
				hasVmoAlphaAnnotation = true

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("zoned"),
				}))
			})
		})

		Context("CIDR", func() {
			It("should forbid invalid VNet CIDRs", func() {
				infrastructureConfig.Networks.VNet.CIDR = &invalidCIDR

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.vnet.cidr"),
					"Detail": Equal("invalid CIDR address: invalid-cidr"),
				}))
			})

			It("should forbid invalid workers CIDR", func() {
				infrastructureConfig.Networks.Workers = invalidCIDR

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.workers"),
					"Detail": Equal("invalid CIDR address: invalid-cidr"),
				}))
			})

			It("should forbid empty workers CIDR", func() {
				infrastructureConfig.Networks.Workers = ""

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(ConsistOfFields(
					Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("networks.workers"),
						"Detail": Equal("invalid CIDR address: "),
					}))
			})

			It("should forbid workers which are not in VNet and Nodes CIDR", func() {
				notOverlappingCIDR := "1.1.1.1/32"
				infrastructureConfig.Networks.Workers = notOverlappingCIDR

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.workers"),
					"Detail": Equal(`must be a subset of "<nil>" ("10.250.0.0/16")`),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.workers"),
					"Detail": Equal(`must be a subset of "networks.vnet.cidr" ("10.0.0.0/8")`),
				}))
			})

			It("should forbid Pod CIDR to overlap with VNet CIDR", func() {
				podCIDR := "10.0.0.1/32"

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &podCIDR, &services, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Detail": Equal(`must not be a subset of "networks.vnet.cidr" ("10.0.0.0/8")`),
				}))
			})

			It("should forbid Services CIDR to overlap with VNet CIDR", func() {
				servicesCIDR := "10.0.0.1/32"

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &servicesCIDR, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Detail": Equal(`must not be a subset of "networks.vnet.cidr" ("10.0.0.0/8")`),
				}))
			})

			It("should forbid non canonical CIDRs", func() {
				vpcCIDR := "10.0.0.3/8"
				nodeCIDR := "10.250.0.3/16"
				podCIDR := "100.96.0.4/11"
				serviceCIDR := "100.64.0.5/13"
				workers := "10.250.3.8/24"

				infrastructureConfig.Networks.Workers = workers
				infrastructureConfig.Networks.VNet = apisazure.VNet{CIDR: &vpcCIDR}

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodeCIDR, &podCIDR, &serviceCIDR, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(HaveLen(2))
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.vnet.cidr"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.workers"),
					"Detail": Equal("must be valid canonical CIDR"),
				}))
			})
		})

		Context("Identity", func() {
			It("should return no errors for using an identity", func() {
				infrastructureConfig.Identity = &apisazure.IdentityConfig{
					Name:          "test-identiy",
					ResourceGroup: "identity-resource-group",
				}
				Expect(ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)).To(BeEmpty())
			})

			It("should return errors because no name or resource group is given", func() {
				infrastructureConfig.Identity = &apisazure.IdentityConfig{
					Name: "test-identiy",
				}
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("identity"),
				}))
			})
		})

		Context("NatGateway", func() {
			BeforeEach(func() {
				infrastructureConfig.Zoned = true
				infrastructureConfig.Networks.NatGateway = &apisazure.NatGatewayConfig{Enabled: true}
			})

			It("should pass as there is no NatGateway config is provided", func() {
				infrastructureConfig.Networks.NatGateway = nil
				Expect(ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)).To(BeEmpty())
			})

			It("should pass as the NatGateway is disabled", func() {
				infrastructureConfig.Networks.NatGateway.Enabled = false
				Expect(ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)).To(BeEmpty())
			})

			It("should fail as NatGatway is disabled but additional config for the NatGateway is supplied", func() {
				infrastructureConfig.Networks.NatGateway.Enabled = false
				infrastructureConfig.Networks.NatGateway.Zone = pointer.Int32Ptr(2)

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.natGateway"),
					"Detail": Equal("NatGateway is disabled but additional NatGateway config is passed"),
				}))
			})

			It("should fail as NatGatway is enabled but the cluster is not zonal", func() {
				infrastructureConfig.Zoned = false
				infrastructureConfig.Networks.NatGateway.Enabled = true

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("networks.natGateway"),
					"Detail": Equal("NatGateway is currently only supported for zonal and VMO clusters"),
				}))
			})

			It("should pass as the NatGateway has a zone", func() {
				infrastructureConfig.Networks.NatGateway.Zone = pointer.Int32Ptr(2)
				Expect(ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)).To(BeEmpty())
			})

			Context("User provided public IP", func() {
				BeforeEach(func() {
					infrastructureConfig.Networks.NatGateway.Zone = pointer.Int32Ptr(1)
					infrastructureConfig.Networks.NatGateway.IPAddresses = []apisazure.PublicIPReference{{
						Name:          "public-ip-name",
						ResourceGroup: "public-ip-resource-group",
						Zone:          1,
					}}
				})

				It("should fail as NatGateway has no zone but an external public ip", func() {
					infrastructureConfig.Networks.NatGateway.Zone = nil
					errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)
					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("networks.natGateway.zone"),
						"Detail": Equal("Public IPs can only be selected for zonal NatGateways"),
					}))
				})

				It("should fail as resource is in a different zone as the NatGateway", func() {
					infrastructureConfig.Networks.NatGateway.IPAddresses[0].Zone = 2
					errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)
					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("networks.natGateway.ipAddresses[0].zone"),
						"Detail": Equal("Public IP can't be used as it is not in the same zone as the NatGateway (zone 1)"),
					}))
				})

				It("should fail as name is empty", func() {
					infrastructureConfig.Networks.NatGateway.IPAddresses[0].Name = ""
					errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)
					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("networks.natGateway.ipAddresses[0].name"),
						"Detail": Equal("Name for NatGateway public ip resource is required"),
					}))
				})

				It("should fail as resource group is empty", func() {
					infrastructureConfig.Networks.NatGateway.IPAddresses[0].ResourceGroup = ""
					errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)
					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("networks.natGateway.ipAddresses[0].resourceGroup"),
						"Detail": Equal("ResourceGroup for NatGateway public ip resouce is required"),
					}))
				})
			})

			Context("IdleConnectionTimeoutMinutes", func() {
				It("should return an error when specifying lower than minimum values", func() {
					var timeoutValue int32 = 0
					infrastructureConfig.Zoned = true
					infrastructureConfig.Networks.NatGateway = &apisazure.NatGatewayConfig{Enabled: true, IdleConnectionTimeoutMinutes: &timeoutValue}
					errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)
					Expect(errorList).To(HaveLen(1))
					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("networks.natGateway.idleConnectionTimeoutMinutes"),
						"Detail": Equal("idleConnectionTimeoutMinutes values must range between 4 and 120"),
					}))
				})

				It("should return an error when specifying greater than maximum values", func() {
					var timeoutValue int32 = 121
					infrastructureConfig.Zoned = true
					infrastructureConfig.Networks.NatGateway = &apisazure.NatGatewayConfig{Enabled: true, IdleConnectionTimeoutMinutes: &timeoutValue}
					errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)
					Expect(errorList).To(HaveLen(1))
					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("networks.natGateway.idleConnectionTimeoutMinutes"),
						"Detail": Equal("idleConnectionTimeoutMinutes values must range between 4 and 120"),
					}))
				})

				It("should not return an error for valid values", func() {
					var timeoutValue int32 = 120
					infrastructureConfig.Zoned = true
					infrastructureConfig.Networks.NatGateway = &apisazure.NatGatewayConfig{Enabled: true, IdleConnectionTimeoutMinutes: &timeoutValue}
					Expect(ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, hasVmoAlphaAnnotation, providerPath)).To(BeEmpty())
				})
			})
		})
	})

	Describe("#ValidateInfrastructureConfigUpdate", func() {
		var newInfrastructureConfig *apisazure.InfrastructureConfig

		BeforeEach(func() {
			newInfrastructureConfig = infrastructureConfig.DeepCopy()
		})

		It("should return no errors for an unchanged config", func() {
			Expect(ValidateInfrastructureConfigUpdate(infrastructureConfig, infrastructureConfig, providerPath)).To(BeEmpty())
		})

		It("should forbid changing the resource group section", func() {
			newInfrastructureConfig.ResourceGroup = &apisazure.ResourceGroup{}

			errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfrastructureConfig, providerPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("resourceGroup"),
			}))))
		})

		Context("vnet config update", func() {
			It("should allow to resize the vnet cidr", func() {
				newInfrastructureConfig := infrastructureConfig.DeepCopy()
				newInfrastructureConfig.Networks.VNet.CIDR = pointer.StringPtr("10.250.3.0/22")

				errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfrastructureConfig, providerPath)
				Expect(errorList).Should(HaveLen(0))
			})

			It("should forbid to modify the external vnet config", func() {
				infrastructureConfig.Networks.VNet.Name = pointer.StringPtr("external-vnet-name")
				infrastructureConfig.Networks.VNet.ResourceGroup = pointer.StringPtr("external-vnet-rg")

				newInfrastructureConfig := infrastructureConfig.DeepCopy()
				newInfrastructureConfig.Networks.VNet.Name = pointer.StringPtr("modified")
				newInfrastructureConfig.Networks.VNet.ResourceGroup = pointer.StringPtr("modified")

				errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfrastructureConfig, providerPath)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.vnet.name"),
					"Detail": Equal("field is immutable"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.vnet.resourceGroup"),
					"Detail": Equal("field is immutable"),
				}))
			})

			It("should forbid to add external vnet config", func() {
				infrastructureConfig.Networks.VNet = apisazure.VNet{}

				newInfrastructureConfig := infrastructureConfig.DeepCopy()
				newInfrastructureConfig.Networks.VNet.Name = pointer.StringPtr("modified")
				newInfrastructureConfig.Networks.VNet.ResourceGroup = pointer.StringPtr("modified")

				errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfrastructureConfig, providerPath)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.vnet.name"),
					"Detail": Equal("field is immutable"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.vnet.resourceGroup"),
					"Detail": Equal("field is immutable"),
				}))
			})

		})

		DescribeTable("Zoned",
			func(isOldZoned, isNewZoned, expectError bool) {
				newInfrastructureConfig := infrastructureConfig.DeepCopy()
				if isOldZoned {
					infrastructureConfig.Zoned = true
				}
				if isNewZoned {
					newInfrastructureConfig.Zoned = true
				}
				errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfrastructureConfig, providerPath)
				if !expectError {
					Expect(errorList).To(HaveLen(0))
					return
				}
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("zoned"),
				}))))
			},
			Entry("should pass as old and new cluster are zoned", true, true, false),
			Entry("should pass as old and new cluster are non-zoned", false, false, false),
			Entry("should forbid moving a zoned cluster to a non-zoned cluster", false, true, true),
			Entry("should forbid moving a non-zoned cluster to a zoned cluster", true, false, true),
		)
	})

	DescribeTable("#ValidateVmoConfigUpdate",
		func(newHasVmoAlphaAnnotation, oldHasVmoAlphaAnnotation, expectErrors bool) {
			var (
				path      *field.Path
				errorList = ValidateVmoConfigUpdate(newHasVmoAlphaAnnotation, oldHasVmoAlphaAnnotation, path)
			)
			if !expectErrors {
				Expect(errorList).To(HaveLen(0))
				return
			}
			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("annotations"),
			}))))
		},
		Entry("should pass as old and new cluster have vmo alpha annotation", true, true, false),
		Entry("should pass as old and new cluster don't have vmo alpha annotation", false, false, false),
		Entry("should forbid removing the vmo alpha annotation for an already existing cluster", true, false, true),
		Entry("should forbid adding the vmo alpha annotation to an already existing cluster", false, true, true),
	)
})
