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

package validation

import (
	"fmt"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	natGatewayMinTimeoutInMinutes int32 = 4
	natGatewayMaxTimeoutInMinutes int32 = 120
)

// ValidateInfrastructureConfig validates a InfrastructureConfig object.
func ValidateInfrastructureConfig(infra *apisazure.InfrastructureConfig, nodesCIDR, podsCIDR, servicesCIDR *string, hasVmoAlphaAnnotation bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	var (
		nodes    cidrvalidation.CIDR
		pods     cidrvalidation.CIDR
		services cidrvalidation.CIDR
	)

	if nodesCIDR != nil {
		nodes = cidrvalidation.NewCIDR(*nodesCIDR, nil)
	}
	if podsCIDR != nil {
		pods = cidrvalidation.NewCIDR(*podsCIDR, nil)
	}
	if servicesCIDR != nil {
		services = cidrvalidation.NewCIDR(*servicesCIDR, nil)
	}

	// Currently, we will not allow deployments into existing resource groups or VNets although this functionality
	// is already implemented, because the Azure cloud provider is not cleaning up self-created resources properly.
	// This resources would be orphaned when the cluster will be deleted. We block these cases thereby that the Azure shoot
	// validation here will fail for those cases.
	// TODO: remove the following block and uncomment below blocks once deployment into existing resource groups works properly.
	if infra.ResourceGroup != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("resourceGroup"), infra.ResourceGroup, "specifying an existing resource group is not supported yet"))
	}

	if infra.Zoned && hasVmoAlphaAnnotation {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("zoned"), infra.Zoned, fmt.Sprintf("specifying a zoned cluster and having the %q annotation is not allowed", azure.ShootVmoUsageAnnotation)))
	}

	networksPath := fldPath.Child("networks")

	// Validate workers subnet cidr
	workerCIDR := cidrvalidation.NewCIDR(infra.Networks.Workers, networksPath.Child("workers"))
	allErrs = append(allErrs, cidrvalidation.ValidateCIDRParse(workerCIDR)...)
	allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(networksPath.Child("workers"), infra.Networks.Workers)...)
	if nodes != nil {
		allErrs = append(allErrs, nodes.ValidateSubset(workerCIDR)...)
	}

	allErrs = append(allErrs, validateVnetConfig(infra.Networks.VNet, infra.ResourceGroup, workerCIDR, nodes, pods, services, networksPath.Child("vnet"))...)
	allErrs = append(allErrs, validateNatGatewayConfig(infra.Networks.NatGateway, infra.Zoned, hasVmoAlphaAnnotation, networksPath.Child("natGateway"))...)

	if infra.Identity != nil && (infra.Identity.Name == "" || infra.Identity.ResourceGroup == "") {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("identity"), infra.Identity, "specifying an identity requires the name of the identity and the resource group which hosts the identity"))
	}

	return allErrs
}

func validateNatGatewayConfig(natGatewayConfig *apisazure.NatGatewayConfig, zoned bool, hasVmoAlphaAnnotation bool, natGatewayPath *field.Path) field.ErrorList {
	var allErrs = field.ErrorList{}

	if natGatewayConfig == nil {
		return nil
	}

	if !natGatewayConfig.Enabled {
		if natGatewayConfig.Zone != nil || natGatewayConfig.IdleConnectionTimeoutMinutes != nil || natGatewayConfig.IPAddresses != nil {
			return append(allErrs, field.Invalid(natGatewayPath, natGatewayConfig, "NatGateway is disabled but additional NatGateway config is passed"))
		}
		return nil
	}

	// NatGateway cannot be offered for Shoot clusters with a primary AvailabilitySet.
	// The NatGateway is not compatible with the Basic SKU Loadbalancers which are
	// required to use for Shoot clusters with AvailabilitySet.
	if !zoned && !hasVmoAlphaAnnotation {
		return append(allErrs, field.Forbidden(natGatewayPath, "NatGateway is currently only supported for zonal and VMO clusters"))
	}

	if natGatewayConfig.IdleConnectionTimeoutMinutes != nil && (*natGatewayConfig.IdleConnectionTimeoutMinutes < natGatewayMinTimeoutInMinutes || *natGatewayConfig.IdleConnectionTimeoutMinutes > natGatewayMaxTimeoutInMinutes) {
		allErrs = append(allErrs, field.Invalid(natGatewayPath.Child("idleConnectionTimeoutMinutes"), *natGatewayConfig.IdleConnectionTimeoutMinutes, "idleConnectionTimeoutMinutes values must range between 4 and 120"))
	}

	if natGatewayConfig.Zone == nil {
		if len(natGatewayConfig.IPAddresses) > 0 {
			allErrs = append(allErrs, field.Invalid(natGatewayPath.Child("zone"), *natGatewayConfig, "Public IPs can only be selected for zonal NatGateways"))
		}
		return allErrs
	}
	allErrs = append(allErrs, validateNatGatewayIPReference(natGatewayConfig.IPAddresses, *natGatewayConfig.Zone, natGatewayPath.Child("ipAddresses"))...)

	return allErrs
}

func validateNatGatewayIPReference(publicIPReferences []apisazure.PublicIPReference, zone int32, fldPath *field.Path) field.ErrorList {
	var allErrs = field.ErrorList{}
	for i, publicIPRef := range publicIPReferences {
		if publicIPRef.Zone != zone {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("zone"), publicIPRef.Zone, fmt.Sprintf("Public IP can't be used as it is not in the same zone as the NatGateway (zone %d)", zone)))
		}
		if publicIPRef.Name == "" {
			allErrs = append(allErrs, field.Required(fldPath.Index(i).Child("name"), "Name for NatGateway public ip resource is required"))
		}
		if publicIPRef.ResourceGroup == "" {
			allErrs = append(allErrs, field.Required(fldPath.Index(i).Child("resourceGroup"), "ResourceGroup for NatGateway public ip resouce is required"))
		}
	}
	return allErrs
}

func validateVnetConfig(vnetConfig apisazure.VNet, resourceGroupConfig *apisazure.ResourceGroup, workers, nodes, pods, services cidrvalidation.CIDR, vnetConfigPath *field.Path) field.ErrorList {
	var allErrs = field.ErrorList{}

	// Validate that just vnet name or vnet resource group is specified.
	if (vnetConfig.Name != nil && vnetConfig.ResourceGroup == nil) || (vnetConfig.Name == nil && vnetConfig.ResourceGroup != nil) {
		return append(allErrs, field.Invalid(vnetConfigPath, vnetConfig, "a vnet cidr or vnet name and resource group need to be specified"))
	}

	if isExternalVnetUsed(&vnetConfig) {
		if vnetConfig.CIDR != nil {
			allErrs = append(allErrs, field.Invalid(vnetConfigPath.Child("cidr"), vnetConfig, "specifying a cidr for an existing vnet is not possible"))
		}

		if resourceGroupConfig != nil && *vnetConfig.ResourceGroup == resourceGroupConfig.Name {
			allErrs = append(allErrs, field.Invalid(vnetConfigPath.Child("resourceGroup"), *vnetConfig.ResourceGroup, "the vnet resource group must not be the same as the cluster resource group"))
		}
		return allErrs
	}

	// Validate no cidr config is specified at all.
	if isDefaultVnetConfig(&vnetConfig) {
		allErrs = append(allErrs, workers.ValidateSubset(nodes)...)
		allErrs = append(allErrs, workers.ValidateNotOverlap(pods, services)...)
		return allErrs
	}

	vnetCIDR := cidrvalidation.NewCIDR(*vnetConfig.CIDR, vnetConfigPath.Child("cidr"))
	allErrs = append(allErrs, vnetCIDR.ValidateParse()...)
	allErrs = append(allErrs, vnetCIDR.ValidateSubset(nodes)...)
	allErrs = append(allErrs, vnetCIDR.ValidateSubset(workers)...)
	allErrs = append(allErrs, vnetCIDR.ValidateNotOverlap(pods, services)...)
	allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(vnetConfigPath.Child("cidr"), *vnetConfig.CIDR)...)

	return allErrs
}

// ValidateInfrastructureConfigUpdate validates a InfrastructureConfig object.
func ValidateInfrastructureConfigUpdate(oldConfig, newConfig *apisazure.InfrastructureConfig, providerPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.ResourceGroup, oldConfig.ResourceGroup, providerPath.Child("resourceGroup"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.Networks.Workers, oldConfig.Networks.Workers, providerPath.Child("networks").Child("workers"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(oldConfig.Zoned, newConfig.Zoned, providerPath.Child("zoned"))...)

	allErrs = append(allErrs, validateVnetConfigUpdate(&oldConfig.Networks, &newConfig.Networks, providerPath.Child("networks"))...)

	return allErrs
}

// ValidateVmoConfigUpdate validates the VMO configuration on update.
func ValidateVmoConfigUpdate(oldShootHasAlphaVmoAnnotation, newShootHasAlphaVmoAnnotation bool, metaDataPath *field.Path) field.ErrorList {
	var allErrs = field.ErrorList{}

	// Check if old shoot has not the vmo alpha annotation and forbid to add it.
	if !oldShootHasAlphaVmoAnnotation && newShootHasAlphaVmoAnnotation {
		allErrs = append(allErrs, field.Forbidden(metaDataPath.Child("annotations"), fmt.Sprintf("not allowed to add annotation %q to an already existing shoot cluster", azure.ShootVmoUsageAnnotation)))
	}

	// Check if old shoot has the vmo alpha annotaion and forbid to remove it.
	if oldShootHasAlphaVmoAnnotation && !newShootHasAlphaVmoAnnotation {
		allErrs = append(allErrs, field.Forbidden(metaDataPath.Child("annotations"), fmt.Sprintf("not allowed to remove annotation %q to an already existing shoot cluster", azure.ShootVmoUsageAnnotation)))
	}

	return allErrs
}

func validateVnetConfigUpdate(oldNeworkConfig, newNetworkConfig *apisazure.NetworkConfig, networkConfigPath *field.Path) field.ErrorList {
	var allErrs = field.ErrorList{}

	if isExternalVnetUsed(&oldNeworkConfig.VNet) || isDefaultVnetConfig(&oldNeworkConfig.VNet) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newNetworkConfig.VNet.Name, oldNeworkConfig.VNet.Name, networkConfigPath.Child("vnet", "name"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newNetworkConfig.VNet.ResourceGroup, oldNeworkConfig.VNet.ResourceGroup, networkConfigPath.Child("vnet", "resourceGroup"))...)
		return allErrs
	}

	if oldNeworkConfig.VNet.CIDR != nil && newNetworkConfig.VNet.CIDR == nil {
		return append(allErrs, field.Invalid(networkConfigPath.Child("vnet", "cidr"), newNetworkConfig.VNet.CIDR, "vnet cidr need to be specified"))
	}

	return allErrs
}

func isExternalVnetUsed(vnetConfig *apisazure.VNet) bool {
	if vnetConfig == nil {
		return false
	}
	if vnetConfig.Name != nil && vnetConfig.ResourceGroup != nil {
		return true
	}
	return false
}

func isDefaultVnetConfig(vnetConfig *apisazure.VNet) bool {
	if vnetConfig.CIDR == nil && vnetConfig.Name == nil && vnetConfig.ResourceGroup == nil {
		return true
	}
	return false
}
