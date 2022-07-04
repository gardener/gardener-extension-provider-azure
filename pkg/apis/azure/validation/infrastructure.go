// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validation

import (
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

const (
	natGatewayMinTimeoutInMinutes int32 = 4
	natGatewayMaxTimeoutInMinutes int32 = 120
)

// ValidateInfrastructureConfigAgainstCloudProfile validates the InfrastructureConfig against the CloudProfile.
func ValidateInfrastructureConfigAgainstCloudProfile(oldInfra, infra *apisazure.InfrastructureConfig, shootRegion string, cloudProfile *gardencorev1beta1.CloudProfile, fld *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if helper.IsUsingSingleSubnetLayout(infra) {
		return allErrs
	}

	for _, region := range cloudProfile.Spec.Regions {
		if region.Name == shootRegion {
			allErrs = append(allErrs, validateInfrastructureConfigZones(oldInfra, infra, region.Zones, fld.Child("networks").Child("zones"))...)
			break
		}
	}

	return allErrs
}

func validateInfrastructureConfigZones(oldInfra, infra *apisazure.InfrastructureConfig, zones []gardencorev1beta1.AvailabilityZone, fld *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	azureZones := sets.NewString()
	for _, zone := range zones {
		azureZones.Insert(zone.Name)
	}

	for i, zone := range infra.Networks.Zones {
		// Search if the current zones were alrady present and valid in the old infrastructureConfig and if so, skip further checks.
		// This safeguards from breaking existing shoots in case a zone has been removed from the CloudProfile.
		if oldInfra != nil && len(oldInfra.Networks.Zones) > 0 {
			matchFound := false
			for _, oldZone := range oldInfra.Networks.Zones {
				if oldZone.Name == zone.Name {
					matchFound = true
					break
				}
			}
			if matchFound {
				continue
			}
		}

		if !azureZones.Has(helper.InfrastructureZoneToString(zone.Name)) {
			allErrs = append(allErrs, field.NotSupported(fld.Index(i).Child("name"), zone.Name, azureZones.UnsortedList()))
		}
	}

	return allErrs
}

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

	allErrs = append(allErrs, validateNetworkConfig(infra, nodes, pods, services, hasVmoAlphaAnnotation, fldPath)...)

	if infra.Identity != nil && (infra.Identity.Name == "" || infra.Identity.ResourceGroup == "") {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("identity"), infra.Identity, "specifying an identity requires the name of the identity and the resource group which hosts the identity"))
	}

	return allErrs
}

func validateNetworkConfig(
	infra *apisazure.InfrastructureConfig,
	nodes cidrvalidation.CIDR,
	pods cidrvalidation.CIDR,
	services cidrvalidation.CIDR,
	hasVmoAlphaAnnotation bool,
	fldPath *field.Path,
) field.ErrorList {

	var (
		config       = infra.Networks
		networksPath = fldPath.Child("networks")
		allErrs      = field.ErrorList{}
		workersPath  = networksPath.Child("workers")
		zonesPath    = networksPath.Child("zones")
		vNetPath     = networksPath.Child("vnet")
		workerCIDR   cidrvalidation.CIDR
	)

	// forbid not setting at least one of {workers, zones}
	if config.Workers == nil && len(config.Zones) == 0 {
		allErrs = append(allErrs, field.Forbidden(workersPath, "either workers or zones must be specified"))
		return allErrs
	}

	// forbid setting both of {workers, zones}
	if config.Workers != nil && len(config.Zones) > 0 {
		allErrs = append(allErrs, field.Forbidden(workersPath, "workers and zones cannot be specified in parallel"))
		return allErrs
	}

	if config.Workers != nil {
		workerCIDR = cidrvalidation.NewCIDR(*config.Workers, workersPath)
	}

	allErrs = append(allErrs, validateVnetConfig(&config, infra.ResourceGroup, workerCIDR, nodes, pods, services, zonesPath, vNetPath)...)

	// handle single subnet layout validation.
	if helper.IsUsingSingleSubnetLayout(infra) {
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRParse(workerCIDR)...)
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(workersPath, *config.Workers)...)

		if nodes != nil {
			allErrs = append(allErrs, nodes.ValidateSubset(workerCIDR)...)
		}

		allErrs = append(allErrs, validateNatGatewayConfig(config.NatGateway, infra.Zoned, hasVmoAlphaAnnotation, networksPath.Child("natGateway"))...)
		return allErrs
	}

	// handle multiple subnet layout validation.
	if !infra.Zoned {
		allErrs = append(allErrs, field.Forbidden(zonesPath, "cannot specify zones in an non-zonal cluster"))
	}

	if config.NatGateway != nil {
		allErrs = append(allErrs, field.Forbidden(workersPath, "natGateway cannot be specified when workers field is missing"))
	}
	if len(config.ServiceEndpoints) > 0 {
		allErrs = append(allErrs, field.Forbidden(workersPath, "serviceEndpoints cannot be specified when workers field is missing"))
	}

	allErrs = append(allErrs, validateZones(config.Zones, nodes, pods, services, zonesPath)...)

	return allErrs
}

func validateVnetConfig(networkConfig *apisazure.NetworkConfig, resourceGroupConfig *apisazure.ResourceGroup, workers, nodes, pods, services cidrvalidation.CIDR, zonesPath, vNetPath *field.Path) field.ErrorList {
	var (
		allErrs    = field.ErrorList{}
		vnetConfig = networkConfig.VNet
	)

	// Validate that just vnet name or vnet resource group is specified.
	if (networkConfig.VNet.Name != nil && vnetConfig.ResourceGroup == nil) || (vnetConfig.Name == nil && vnetConfig.ResourceGroup != nil) {
		return append(allErrs, field.Invalid(vNetPath, vnetConfig, "a vnet cidr or vnet name and resource group need to be specified"))
	}

	if isExternalVnetUsed(&networkConfig.VNet) {
		if networkConfig.VNet.CIDR != nil {
			allErrs = append(allErrs, field.Invalid(vNetPath.Child("cidr"), vnetConfig, "specifying a cidr for an existing vnet is not possible"))
		}

		if resourceGroupConfig != nil && *networkConfig.VNet.ResourceGroup == resourceGroupConfig.Name {
			allErrs = append(allErrs, field.Invalid(vNetPath.Child("resourceGroup"), *vnetConfig.ResourceGroup, "the vnet resource group must not be the same as the cluster resource group"))
		}

		if networkConfig.VNet.DDosProtectionPlanID != nil {
			allErrs = append(allErrs, field.Forbidden(vNetPath.Child("ddosProtectionPlanID"), "cannot assign a ddos protection plan to a vnet not managed by Gardener"))
		}
		return allErrs
	}

	if isDefaultVnetConfig(&networkConfig.VNet) {
		if workers == nil {
			allErrs = append(allErrs, field.Forbidden(vNetPath.Child("cidr"), "a vnet cidr or vnet reference must be specified when the workers field is not set"))
			return allErrs
		}

		allErrs = append(allErrs, workers.ValidateSubset(nodes)...)
		allErrs = append(allErrs, workers.ValidateNotOverlap(pods, services)...)
		return allErrs
	}

	vnetCIDR := cidrvalidation.NewCIDR(*networkConfig.VNet.CIDR, vNetPath.Child("cidr"))
	allErrs = append(allErrs, vnetCIDR.ValidateParse()...)
	allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(vNetPath.Child("cidr"), *vnetConfig.CIDR)...)
	allErrs = append(allErrs, vnetCIDR.ValidateSubset(nodes)...)
	allErrs = append(allErrs, vnetCIDR.ValidateNotOverlap(pods, services)...)
	if workers != nil {
		allErrs = append(allErrs, vnetCIDR.ValidateSubset(workers)...)
	}
	for index, zone := range networkConfig.Zones {
		zoneCIDR := cidrvalidation.NewCIDR(zone.CIDR, zonesPath.Index(index).Child("cidr"))
		allErrs = append(allErrs, vnetCIDR.ValidateSubset(zoneCIDR)...)
	}

	return allErrs
}

func validateZones(zones []apisazure.Zone, nodes, pods, services cidrvalidation.CIDR, fld *field.Path) field.ErrorList {
	var (
		allErrs   = field.ErrorList{}
		zoneNames = sets.NewInt32()
		zoneCIDRs []cidrvalidation.CIDR
	)

	for index, zone := range zones {
		zonePath := fld.Index(index)
		if zoneNames.Has(zone.Name) {
			allErrs = append(allErrs, field.Invalid(zonePath, zone.Name, "the same zone cannot be specified multiple times"))
		}
		zoneNames.Insert(zone.Name)

		// construct the zone CIDR slice
		zoneCIDR := cidrvalidation.NewCIDR(zone.CIDR, zonePath.Child("cidr"))
		zoneCIDRs = append(zoneCIDRs, zoneCIDR)
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(zoneCIDR.GetFieldPath(), zoneCIDR.GetCIDR())...)

		// NAT validation
		allErrs = append(allErrs, validateZonedNatGatewayConfig(zone.NatGateway, zonePath.Child("natGateway"))...)
	}

	allErrs = append(allErrs, cidrvalidation.ValidateCIDRParse(zoneCIDRs...)...)
	if nodes != nil {
		allErrs = append(allErrs, nodes.ValidateSubset(zoneCIDRs...)...)
	}

	allErrs = append(allErrs, cidrvalidation.ValidateCIDROverlap(zoneCIDRs, false)...)
	if pods != nil {
		allErrs = append(allErrs, pods.ValidateNotOverlap(zoneCIDRs...)...)
	}
	if services != nil {
		allErrs = append(allErrs, services.ValidateNotOverlap(zoneCIDRs...)...)
	}
	return allErrs
}

func validateNatGatewayConfig(natGatewayConfig *apisazure.NatGatewayConfig, zoned bool, hasVmoAlphaAnnotation bool, natGatewayPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

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
	allErrs := field.ErrorList{}
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

func validateZonedNatGatewayConfig(natGatewayConfig *apisazure.ZonedNatGatewayConfig, natGatewayPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if natGatewayConfig == nil {
		return nil
	}

	if !natGatewayConfig.Enabled {
		if natGatewayConfig.IdleConnectionTimeoutMinutes != nil || natGatewayConfig.IPAddresses != nil {
			return append(allErrs, field.Invalid(natGatewayPath, natGatewayConfig, "NatGateway is disabled but additional NatGateway config is passed"))
		}
		return nil
	}

	allErrs = append(allErrs, validateZonedPublicIPReference(natGatewayConfig.IPAddresses, natGatewayPath.Child("ipAddresses"))...)
	return allErrs
}

func validateZonedPublicIPReference(publicIPReferences []apisazure.ZonedPublicIPReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, publicIPRef := range publicIPReferences {
		if publicIPRef.Name == "" {
			allErrs = append(allErrs, field.Required(fldPath.Index(i).Child("name"), "Name for NatGateway public ip resource is required"))
		}
		if publicIPRef.ResourceGroup == "" {
			allErrs = append(allErrs, field.Required(fldPath.Index(i).Child("resourceGroup"), "ResourceGroup for NatGateway public ip resouce is required"))
		}
	}
	return allErrs
}

// ValidateInfrastructureConfigUpdate validates a InfrastructureConfig object.
func ValidateInfrastructureConfigUpdate(oldConfig, newConfig *apisazure.InfrastructureConfig, providerPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.ResourceGroup, oldConfig.ResourceGroup, providerPath.Child("resourceGroup"))...)

	if oldConfig.Networks.Workers != nil && newConfig.Networks.Workers != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.Networks.Workers, oldConfig.Networks.Workers, providerPath.Child("networks").Child("workers"))...)
	}

	// validate state transitions for the network layouts
	switch {
	// if both new and old InfrastructureConfigs use multiple-subnet layout, validate the zones
	case !helper.IsUsingSingleSubnetLayout(oldConfig) && !helper.IsUsingSingleSubnetLayout(newConfig):
		allErrs = append(allErrs, validateZonesUpdate(oldConfig, newConfig, providerPath)...)
	// disallow transitioning from a multiple-subnet to a single-subnet layout
	case !helper.IsUsingSingleSubnetLayout(oldConfig) && helper.IsUsingSingleSubnetLayout(newConfig):
		allErrs = append(allErrs, field.Forbidden(providerPath.Child("networks").Child("worker"), "updating the infrastructure configuration from using dedicated subnets per zone to using single subnet is not allowed"))
	// validate transition from single-subnet to multiple-subnet layout
	case helper.IsUsingSingleSubnetLayout(oldConfig) && !helper.IsUsingSingleSubnetLayout(newConfig):
		allErrs = append(allErrs, validateSingleSubnetToMultipleSubnetTransition(oldConfig, newConfig, providerPath)...)
	default:
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.Networks.Workers, oldConfig.Networks.Workers, providerPath.Child("networks").Child("workers"))...)
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(oldConfig.Zoned, newConfig.Zoned, providerPath.Child("zoned"))...)
	allErrs = append(allErrs, validateVnetConfigUpdate(&oldConfig.Networks, &newConfig.Networks, providerPath.Child("networks"))...)

	return allErrs
}

// ValidateVmoConfigUpdate validates the VMO configuration on update.
func ValidateVmoConfigUpdate(oldShootHasAlphaVmoAnnotation, newShootHasAlphaVmoAnnotation bool, metaDataPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

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
	allErrs := field.ErrorList{}

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

func validateSingleSubnetToMultipleSubnetTransition(old, new *apisazure.InfrastructureConfig, fld *field.Path) field.ErrorList {
	var (
		allErrs           = field.ErrorList{}
		foundMatchingCIDR = false
	)

	for _, zone := range new.Networks.Zones {
		if zone.CIDR == *old.Networks.Workers {
			foundMatchingCIDR = true
			break
		}
	}

	if !foundMatchingCIDR {
		allErrs = append(allErrs, field.Forbidden(fld.Child("networks", "zones"), "when updating InfrastructureConfig to use dedicated subnets per zones, the CIDR for one of the zones must match that of the previous config.networks.workers"))
	}

	return allErrs
}

func validateZonesUpdate(old, new *apisazure.InfrastructureConfig, fld *field.Path) field.ErrorList {
	var (
		allErrs  = field.ErrorList{}
		oldZones = old.Networks.Zones
		newZones = new.Networks.Zones
	)

	if len(oldZones) > len(newZones) {
		allErrs = append(allErrs, field.Forbidden(fld.Child("networks", "zones"), "removing zones is not allowed"))
	}

	for i, newZone := range newZones {
		for _, oldZone := range oldZones {
			if newZone.Name == oldZone.Name {
				idxPath := fld.Child("networks", "zones").Index(i)
				allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Networks.Zones[i].CIDR, oldZone.CIDR, idxPath.Child("cidr"))...)
			}
		}
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
