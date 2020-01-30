// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"

	. "github.com/gardener/gardener/pkg/utils/validation/gomega"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var _ = Describe("InfrastructureConfig validation", func() {
	var (
		infrastructureConfig *apisazure.InfrastructureConfig
		nodes                string
		resourceGroup        = "shoot--test--foo"

		pods        = "100.96.0.0/11"
		services    = "100.64.0.0/13"
		vnetCIDR    = "10.0.0.0/8"
		invalidCIDR = "invalid-cidr"
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
	})

	Describe("#ValidateInfrastructureConfig", func() {
		It("should forbid specifying a resource group configuration", func() {
			infrastructureConfig.ResourceGroup = &apisazure.ResourceGroup{}

			errorList := ValidateInfrastructureConfig(infrastructureConfig, &resourceGroup, &nodes, &pods, &services)

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
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &resourceGroup, &nodes, &pods, &services)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.vnet"),
					"Detail": Equal("specifying an existing vnet name require a vnet name and vnet resource group"),
				}))
			})

			It("should forbid specifying a vnet resource group without name", func() {
				vnetGroup := "existing-vnet-rg"
				infrastructureConfig.Networks.VNet = apisazure.VNet{
					ResourceGroup: &vnetGroup,
				}
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &resourceGroup, &nodes, &pods, &services)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.vnet"),
					"Detail": Equal("specifying an existing vnet name require a vnet name and vnet resource group"),
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
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &resourceGroup, &nodes, &pods, &services)

				Expect(errorList).To(ConsistOfFields(Fields{
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
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &resourceGroup, &nodes, &pods, &services)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.vnet.resourceGroup"),
					"Detail": Equal("specifying an existing vnet is the cluster resource group is not supported"),
				}))
			})

			It("should pass if no vnet cidr is specified and default is applied", func() {
				nodes = "10.250.3.0/24"
				infrastructureConfig.Networks = apisazure.NetworkConfig{
					Workers: "10.250.3.0/24",
				}
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &resourceGroup, &nodes, &pods, &services)
				Expect(errorList).To(HaveLen(0))
			})
		})

		Context("CIDR", func() {
			It("should forbid invalid VNet CIDRs", func() {
				infrastructureConfig.Networks.VNet.CIDR = &invalidCIDR

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &resourceGroup, &nodes, &pods, &services)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.vnet.cidr"),
					"Detail": Equal("invalid CIDR address: invalid-cidr"),
				}))
			})

			It("should forbid invalid workers CIDR", func() {
				infrastructureConfig.Networks.Workers = invalidCIDR

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &resourceGroup, &nodes, &pods, &services)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.workers"),
					"Detail": Equal("invalid CIDR address: invalid-cidr"),
				}))
			})

			It("should forbid workers which are not in VNet and Nodes CIDR", func() {
				notOverlappingCIDR := "1.1.1.1/32"
				infrastructureConfig.Networks.Workers = notOverlappingCIDR

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &resourceGroup, &nodes, &pods, &services)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.workers"),
					"Detail": Equal(`must be a subset of "" ("10.250.0.0/16")`),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.workers"),
					"Detail": Equal(`must be a subset of "networks.vnet.cidr" ("10.0.0.0/8")`),
				}))
			})

			It("should forbid Pod CIDR to overlap with VNet CIDR", func() {
				podCIDR := "10.0.0.1/32"

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &resourceGroup, &nodes, &podCIDR, &services)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal(""),
					"Detail": Equal(`must not be a subset of "networks.vnet.cidr" ("10.0.0.0/8")`),
				}))
			})

			It("should forbid Services CIDR to overlap with VNet CIDR", func() {
				servicesCIDR := "10.0.0.1/32"

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &resourceGroup, &nodes, &pods, &servicesCIDR)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal(""),
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

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &resourceGroup, &nodeCIDR, &podCIDR, &serviceCIDR)

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
				Expect(ValidateInfrastructureConfig(infrastructureConfig, &resourceGroup, &nodes, &pods, &services)).To(BeEmpty())
			})

			It("should return errors because no name or resource group is given", func() {
				infrastructureConfig.Identity = &apisazure.IdentityConfig{
					Name: "test-identiy",
				}
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &resourceGroup, &nodes, &pods, &services)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("identity"),
				}))
			})
		})
	})

	Describe("#ValidateInfrastructureConfigUpdate", func() {
		It("should return no errors for an unchanged config", func() {
			Expect(ValidateInfrastructureConfigUpdate(infrastructureConfig, infrastructureConfig, &nodes, &pods, &services)).To(BeEmpty())
		})

		It("should forbid changing the resource group section", func() {
			newInfrastructureConfig := infrastructureConfig.DeepCopy()
			newInfrastructureConfig.ResourceGroup = &apisazure.ResourceGroup{}

			errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfrastructureConfig, &nodes, &pods, &services)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("resourceGroup"),
			}))))
		})

		It("should forbid changing the network section", func() {
			newInfrastructureConfig := infrastructureConfig.DeepCopy()
			newCIDR := "1.2.3.4/5"
			newInfrastructureConfig.Networks.VNet.CIDR = &newCIDR

			errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfrastructureConfig, &nodes, &pods, &services)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("networks"),
			}))))
		})
	})
})
