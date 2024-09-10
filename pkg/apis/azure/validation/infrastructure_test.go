// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
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
		networking  = core.Networking{}

		workers      = "10.250.3.0/24"
		providerPath *field.Path

		ddosProtectionPlanID string
	)

	BeforeEach(func() {
		nodes = "10.250.0.0/16"
		networking = core.Networking{
			Pods:     &pods,
			Services: &services,
			Nodes:    &nodes,
		}
		infrastructureConfig = &apisazure.InfrastructureConfig{
			Networks: apisazure.NetworkConfig{
				Workers: &workers,
				VNet: apisazure.VNet{
					CIDR: &vnetCIDR,
				},
			},
		}
		hasVmoAlphaAnnotation = false
		ddosProtectionPlanID = "/subscriptions/test/resourceGroups/test/providers/Microsoft.Network/ddosProtectionPlans/test-ddos-protection-plan"
	})

	Describe("#ValidateInfrastructureConfig", func() {
		It("should forbid specifying a resource group configuration", func() {
			infrastructureConfig.ResourceGroup = &apisazure.ResourceGroup{}

			errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)

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
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)

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
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(ConsistOfFields(
					Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("networks.vnet"),
						"Detail": Equal("a vnet cidr or vnet name and resource group need to be specified"),
					}))
			})

			It("should forbid specifying a vnet resource group with empty name or resource group", func() {
				infrastructureConfig.Networks.VNet = apisazure.VNet{
					Name:          ptr.To(""),
					ResourceGroup: ptr.To(""),
				}
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(ConsistOfFields(
					Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("networks.vnet.name"),
						"Detail": Equal("the vnet name must not be empty"),
					},
					Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("networks.vnet.resourceGroup"),
						"Detail": Equal("the vnet resource group must not be empty"),
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
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)

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
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)

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
				networking.Nodes = ptr.To("10.250.3.0/24")
				infrastructureConfig.ResourceGroup = nil
				infrastructureConfig.Networks = apisazure.NetworkConfig{
					Workers: &workers,
				}
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)
				Expect(errorList).To(HaveLen(0))
			})

			It("should allow to provide a ddos protection plan for a managed vnet", func() {
				networking.Nodes = ptr.To("10.250.3.0/24")
				infrastructureConfig.ResourceGroup = nil
				infrastructureConfig.Networks.VNet.DDosProtectionPlanID = &ddosProtectionPlanID
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)
				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid providing an ddos protection plan to an existing vnet", func() {
				name := "existing-vnet"
				infrastructureConfig.Networks.VNet = apisazure.VNet{
					Name:                 &name,
					ResourceGroup:        &resourceGroup,
					DDosProtectionPlanID: &ddosProtectionPlanID,
				}
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(ConsistOfFields(
					Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("networks.vnet.ddosProtectionPlanID"),
					}))
			})
		})

		Context("Zonal", func() {
			It(fmt.Sprintf("should forbid specifying the %q annotation for a zonal cluster", azure.ShootVmoUsageAnnotation), func() {
				infrastructureConfig.Zoned = true
				hasVmoAlphaAnnotation = true

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("zoned"),
				}))
			})
		})

		Context("CIDR", func() {
			It("should forbid invalid VNet CIDRs", func() {
				infrastructureConfig.Networks.VNet.CIDR = &invalidCIDR

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.vnet.cidr"),
					"Detail": Equal("invalid CIDR address: invalid-cidr"),
				}))
			})

			It("should forbid invalid workers CIDR", func() {
				infrastructureConfig.Networks.Workers = &invalidCIDR

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.workers"),
					"Detail": Equal("invalid CIDR address: invalid-cidr"),
				}))
			})

			It("should forbid empty workers CIDR", func() {
				emptyStr := ""
				infrastructureConfig.Networks.Workers = &emptyStr

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(ConsistOfFields(
					Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("networks.workers"),
						"Detail": Equal("invalid CIDR address: "),
					}))
			})

			It("should forbid nil workers CIDR", func() {
				infrastructureConfig.Networks.Workers = nil

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(ConsistOfFields(
					Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("networks.workers"),
						"Detail": Equal("either workers or zones must be specified"),
					}))
			})

			It("should forbid workers which are not in VNet and Nodes CIDR", func() {
				notOverlappingCIDR := "1.1.1.1/32"
				infrastructureConfig.Networks.Workers = &notOverlappingCIDR

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.workers"),
					"Detail": Equal(`must be a subset of "networking.nodes" ("10.250.0.0/16")`),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.workers"),
					"Detail": Equal(`must be a subset of "networks.vnet.cidr" ("10.0.0.0/8")`),
				}))
			})

			It("should forbid Pod CIDR to overlap with VNet CIDR", func() {
				networking.Pods = ptr.To("10.0.0.1/32")

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Detail": Equal(`must not overlap with "networks.vnet.cidr" ("10.0.0.0/8")`),
				}))
			})

			It("should forbid Services CIDR to overlap with VNet CIDR", func() {
				networking.Services = ptr.To("10.0.0.1/32")

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Detail": Equal(`must not overlap with "networks.vnet.cidr" ("10.0.0.0/8")`),
				}))
			})

			It("should forbid non canonical CIDRs", func() {
				vpcCIDR := "10.0.0.3/8"
				networking.Nodes = ptr.To("10.250.0.3/16")
				networking.Pods = ptr.To("100.96.0.4/11")
				networking.Services = ptr.To("100.64.0.5/13")
				workers := "10.250.3.8/24"

				infrastructureConfig.Networks.Workers = &workers
				infrastructureConfig.Networks.VNet = apisazure.VNet{CIDR: &vpcCIDR}

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)

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
				Expect(ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)).To(BeEmpty())
			})

			It("should return errors because no name or resource group is given", func() {
				infrastructureConfig.Identity = &apisazure.IdentityConfig{
					Name: "test-identiy",
				}
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)
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
				Expect(ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)).To(BeEmpty())
			})

			It("should pass as the NatGateway is disabled", func() {
				infrastructureConfig.Networks.NatGateway.Enabled = false
				Expect(ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)).To(BeEmpty())
			})

			It("should fail as NatGatway is disabled but additional config for the NatGateway is supplied", func() {
				infrastructureConfig.Networks.NatGateway.Enabled = false
				infrastructureConfig.Networks.NatGateway.Zone = ptr.To[int32](2)

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.natGateway"),
					"Detail": Equal("NatGateway is disabled but additional NatGateway config is passed"),
				}))
			})

			It("should fail as NatGatway is enabled but the cluster is not zonal", func() {
				infrastructureConfig.Zoned = false
				infrastructureConfig.Networks.NatGateway.Enabled = true

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("networks.natGateway"),
					"Detail": Equal("NatGateway is currently only supported for zonal and VMO clusters"),
				}))
			})

			It("should pass as the NatGateway has a zone", func() {
				infrastructureConfig.Networks.NatGateway.Zone = ptr.To[int32](2)
				Expect(ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)).To(BeEmpty())
			})

			Context("User provided public IP", func() {
				BeforeEach(func() {
					infrastructureConfig.Networks.NatGateway.Zone = ptr.To[int32](1)
					infrastructureConfig.Networks.NatGateway.IPAddresses = []apisazure.PublicIPReference{{
						Name:          "public-ip-name",
						ResourceGroup: "public-ip-resource-group",
						Zone:          1,
					}}
				})

				It("should fail as NatGateway has no zone but an external public ip", func() {
					infrastructureConfig.Networks.NatGateway.Zone = nil
					errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)
					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("networks.natGateway.zone"),
						"Detail": Equal("Public IPs can only be selected for zonal NatGateways"),
					}))
				})

				It("should fail as resource is in a different zone as the NatGateway", func() {
					infrastructureConfig.Networks.NatGateway.IPAddresses[0].Zone = 2
					errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)
					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("networks.natGateway.ipAddresses[0].zone"),
						"Detail": Equal("Public IP can't be used as it is not in the same zone as the NatGateway (zone 1)"),
					}))
				})

				It("should fail as name is empty", func() {
					infrastructureConfig.Networks.NatGateway.IPAddresses[0].Name = ""
					errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)
					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("networks.natGateway.ipAddresses[0].name"),
						"Detail": Equal("Name for NatGateway public ip resource is required"),
					}))
				})

				It("should fail as resource group is empty", func() {
					infrastructureConfig.Networks.NatGateway.IPAddresses[0].ResourceGroup = ""
					errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)
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
					errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)
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
					errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)
					Expect(errorList).To(HaveLen(1))
					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("networks.natGateway.idleConnectionTimeoutMinutes"),
						"Detail": Equal("idleConnectionTimeoutMinutes values must range between 4 and 120"),
					}))
				})

				It("should succeed for valid values", func() {
					var timeoutValue int32 = 120
					infrastructureConfig.Zoned = true
					infrastructureConfig.Networks.NatGateway = &apisazure.NatGatewayConfig{Enabled: true, IdleConnectionTimeoutMinutes: &timeoutValue}
					Expect(ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)).To(BeEmpty())
				})
			})
		})

		Context("Zones", func() {
			var (
				zoneName  int32 = 1
				zoneName1 int32 = 2
				zoneCIDR        = "10.250.0.0/24"
				zoneCIDR1       = "10.250.1.0/24"
			)

			BeforeEach(func() {
				infrastructureConfig = &apisazure.InfrastructureConfig{
					Zoned: true,
					Networks: apisazure.NetworkConfig{
						VNet: apisazure.VNet{
							CIDR: &vnetCIDR,
						},
						Zones: []apisazure.Zone{
							{
								Name: zoneName,
								CIDR: zoneCIDR,
							},
							{
								Name: zoneName1,
								CIDR: zoneCIDR1,
							},
						},
					},
				}
			})

			It("should succeed", func() {
				Expect(ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)).To(BeEmpty())
			})

			It("should succeed for nil service and pod CIDR", func() {
				networking.Pods = nil
				networking.Services = nil
				Expect(ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)).To(BeEmpty())
			})

			It("should succeed with NAT Gateway", func() {
				infrastructureConfig.Networks.Zones[0].NatGateway = &apisazure.ZonedNatGatewayConfig{
					Enabled: true,
				}
				infrastructureConfig.Networks.Zones[1].NatGateway = &apisazure.ZonedNatGatewayConfig{
					Enabled: true,
				}
				Expect(ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)).To(BeEmpty())
			})

			It("should succeed with NAT Gateway and  public IPs", func() {
				infrastructureConfig.Networks.Zones[0].NatGateway = &apisazure.ZonedNatGatewayConfig{
					Enabled: true,
					IPAddresses: []apisazure.ZonedPublicIPReference{
						{
							Name:          "public-ip-name",
							ResourceGroup: "public-ip-resource-group",
						},
					},
				}
				Expect(ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)).To(BeEmpty())
			})

			It("should forbid non canonical CIDRs", func() {
				infrastructureConfig.Networks.Zones[0].CIDR = "10.250.0.1/24"
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)
				Expect(errorList).NotTo(BeEmpty())
				Expect(errorList).To(HaveLen(1))
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("networks.zones[0].cidr"),
				}))
			})

			It("should forbid overlapping zone CIDRs", func() {
				infrastructureConfig.Networks.Zones[0].CIDR = zoneCIDR1
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)
				Expect(errorList).NotTo(BeEmpty())
				Expect(errorList).To(HaveLen(1))
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.zones[1].cidr"),
					"Detail": ContainSubstring("must not overlap"),
				}))
			})

			It("should forbid not specifying VNet when using Zones", func() {
				infrastructureConfig.Networks.VNet = apisazure.VNet{}
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)
				Expect(errorList).NotTo(BeEmpty())
				Expect(errorList).To(HaveLen(1))
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("networks.vnet.cidr"),
					"Detail": Equal("a vnet cidr or vnet reference must be specified when the workers field is not set"),
				}))
			})

			It("should forbid zone CIDRs which are not in Vnet and Nodes CIDR", func() {
				vpcCIDR := "10.0.0.0/8"
				networking.Nodes = ptr.To("10.150.0.0/16")

				infrastructureConfig.Networks.VNet.CIDR = &vpcCIDR
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)

				Expect(errorList).NotTo(BeEmpty())
				Expect(errorList).To(HaveLen(2))
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.zones[0].cidr"),
					"Detail": ContainSubstring("subset of"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.zones[1].cidr"),
					"Detail": ContainSubstring("subset of"),
				}))
			})

			It("should forbid specifying zone multiple times", func() {
				infrastructureConfig.Networks.Zones[0].Name = zoneName1

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &networking, hasVmoAlphaAnnotation, providerPath)
				Expect(errorList).NotTo(BeEmpty())
				Expect(errorList).To(HaveLen(1))
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.zones[1]"),
					"Detail": Equal("the same zone cannot be specified multiple times"),
				}))
			})
		})
	})

	Describe("#ValidateInfrastructureConfigAgainstCloudProfile", func() {
		var (
			region          = "region"
			zoneName  int32 = 1
			zoneName1 int32 = 2
			cp        *v1beta1.CloudProfile
		)

		BeforeEach(func() {
			cp = &v1beta1.CloudProfile{
				Spec: v1beta1.CloudProfileSpec{
					Regions: []v1beta1.Region{
						{
							Name: region,
							Zones: []v1beta1.AvailabilityZone{
								{
									Name: "1",
								},
								{
									Name: "2",
								},
							},
						},
					},
				},
			}
			infrastructureConfig = &apisazure.InfrastructureConfig{
				Zoned: true,
				Networks: apisazure.NetworkConfig{
					VNet: apisazure.VNet{
						CIDR: &vnetCIDR,
					},
					Zones: []apisazure.Zone{
						{
							Name: zoneName,
						},
						{
							Name: zoneName1,
						},
					},
				},
			}
		})

		It("should deny zones not present in cloudprofile", func() {
			infrastructureConfig.Networks.Zones[0].Name = 5
			errorList := ValidateInfrastructureConfigAgainstCloudProfile(nil, infrastructureConfig, region, &cp.Spec, providerPath)
			Expect(errorList).NotTo(BeEmpty())
			Expect(errorList).To(HaveLen(1))
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("networks.zones[0].name"),
			}))
		})
		It("should allow zones removed from cloudprofile", func() {
			cp.Spec.Regions[0].Zones = cp.Spec.Regions[0].Zones[1:]
			errorList := ValidateInfrastructureConfigAgainstCloudProfile(infrastructureConfig, infrastructureConfig, region, &cp.Spec, providerPath)
			Expect(errorList).To(BeEmpty())
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
				newInfrastructureConfig.Networks.VNet.CIDR = ptr.To("10.0.0.0/7")

				errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfrastructureConfig, providerPath)
				Expect(errorList).Should(BeEmpty())
			})

			It("should forbid shrinking vnet cidr", func() {
				newInfrastructureConfig := infrastructureConfig.DeepCopy()
				newInfrastructureConfig.Networks.VNet.CIDR = ptr.To("10.0.0.0/9")

				errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfrastructureConfig, providerPath)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.vnet.cidr"),
					"Detail": Equal("VNet CIDR blocks can only be expanded"),
				}))
			})

			It("should forbid to modify the external vnet config", func() {
				infrastructureConfig.Networks.VNet.Name = ptr.To("external-vnet-name")
				infrastructureConfig.Networks.VNet.ResourceGroup = ptr.To("external-vnet-rg")

				newInfrastructureConfig := infrastructureConfig.DeepCopy()
				newInfrastructureConfig.Networks.VNet.Name = ptr.To("modified")
				newInfrastructureConfig.Networks.VNet.ResourceGroup = ptr.To("modified")

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
				newInfrastructureConfig.Networks.VNet.Name = ptr.To("modified")
				newInfrastructureConfig.Networks.VNet.ResourceGroup = ptr.To("modified")

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

		Context("Infrastructure Zones", func() {
			BeforeEach(func() {
				infrastructureConfig.Zoned = true
			})

			It("transition should succeed", func() {
				newCIDR := "10.250.250.0/24"
				newInfra := &apisazure.InfrastructureConfig{
					Zoned: true,
					Networks: apisazure.NetworkConfig{
						VNet: apisazure.VNet{
							CIDR: &vnetCIDR,
						},
						Zones: []apisazure.Zone{
							{
								Name: 1,
								CIDR: workers,
							},
							{
								Name: 2,
								CIDR: newCIDR,
							},
						},
					},
				}

				errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfra, providerPath)
				Expect(errorList).To(BeEmpty())
			})

			It("should deny transition from multi-subnet to single subnet", func() {
				newInfra := infrastructureConfig
				oldInfra := &apisazure.InfrastructureConfig{
					Zoned: true,
					Networks: apisazure.NetworkConfig{
						VNet: apisazure.VNet{
							CIDR: &vnetCIDR,
						},
						Zones: []apisazure.Zone{
							{
								Name: 1,
								CIDR: "1.1.1.1/20",
							},
							{
								Name: 2,
								CIDR: "2.2.2.2/20",
							},
						},
					},
				}

				errorList := ValidateInfrastructureConfigUpdate(oldInfra, newInfra, providerPath)
				Expect(errorList).NotTo(BeEmpty())
				Expect(errorList).To(HaveLen(1))
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("networks.worker"),
					"Detail": Equal("updating the infrastructure configuration from using dedicated subnets per zone to using single subnet is not allowed"),
				}))
			})

			It("should deny transition to multi-subnet if the old workers CIDR is not used by any zone", func() {
				newCIDR := "10.250.250.0/24"
				newCIDR2 := "10.250.251.0/24"
				newInfra := &apisazure.InfrastructureConfig{
					Zoned: true,
					Networks: apisazure.NetworkConfig{
						VNet: apisazure.VNet{
							CIDR: &vnetCIDR,
						},
						Zones: []apisazure.Zone{
							{
								Name: 1,
								CIDR: newCIDR,
							},
							{
								Name: 2,
								CIDR: newCIDR2,
							},
						},
					},
				}

				errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfra, providerPath)
				Expect(errorList).NotTo(BeEmpty())
				Expect(errorList).To(HaveLen(1))
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("networks.zones"),
					"Detail": Equal("when updating InfrastructureConfig to use dedicated subnets per zones, the CIDR for one of the zones must match that of the previous config.networks.workers"),
				}))
			})

			It("should deny changing zone CIDR", func() {
				zonedInfra := &apisazure.InfrastructureConfig{
					Zoned: true,
					Networks: apisazure.NetworkConfig{
						VNet: apisazure.VNet{
							CIDR: &vnetCIDR,
						},
						Zones: []apisazure.Zone{
							{
								Name: 1,
								CIDR: workers,
							},
							{
								Name: 2,
								CIDR: workers,
							},
						},
					},
				}

				newZonedInfra := zonedInfra.DeepCopy()
				newZonedInfra.Networks.Zones[0].CIDR = "10.0.0.0/24"

				errorList := ValidateInfrastructureConfigUpdate(zonedInfra, newZonedInfra, providerPath)
				Expect(errorList).NotTo(BeEmpty())
				Expect(errorList).To(HaveLen(1))
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("networks.zones[0].cidr"),
				}))
			})
		})
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
