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

package validation

import (
	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"

	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateInfrastructureConfig validates a InfrastructureConfig object.
func ValidateInfrastructureConfig(infra *apisazure.InfrastructureConfig, resourceGroupName, nodesCIDR, podsCIDR, servicesCIDR *string) field.ErrorList {
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
		allErrs = append(allErrs, field.Invalid(field.NewPath("resourceGroup"), infra.ResourceGroup, "specifying an existing resource group is not supported yet"))
	}

	networksPath := field.NewPath("networks")
	if len(infra.Networks.Workers) == 0 {
		allErrs = append(allErrs, field.Required(networksPath.Child("workers"), "must specify the network range for the worker network"))
	}

	workerCIDR := cidrvalidation.NewCIDR(infra.Networks.Workers, networksPath.Child("workers"))

	allErrs = append(allErrs, cidrvalidation.ValidateCIDRParse(workerCIDR)...)
	allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(networksPath.Child("workers"), infra.Networks.Workers)...)

	if (infra.Networks.VNet.Name != nil && infra.Networks.VNet.ResourceGroup == nil) || (infra.Networks.VNet.Name == nil && infra.Networks.VNet.ResourceGroup != nil) {
		allErrs = append(allErrs, field.Invalid(networksPath.Child("vnet"), infra.Networks.VNet, "specifying an existing vnet name require a vnet name and vnet resource group"))
	} else if infra.Networks.VNet.Name != nil && infra.Networks.VNet.ResourceGroup != nil {
		if infra.Networks.VNet.CIDR != nil {
			allErrs = append(allErrs, field.Invalid(networksPath.Child("vnet", "cidr"), *infra.Networks.VNet.ResourceGroup, "specifying a cidr for an existing vnet is not possible"))
		}
		if *infra.Networks.VNet.ResourceGroup == *resourceGroupName {
			allErrs = append(allErrs, field.Invalid(networksPath.Child("vnet", "resourceGroup"), *infra.Networks.VNet.ResourceGroup, "specifying an existing vnet is the cluster resource group is not supported"))
		}
	} else {
		cidrPath := networksPath.Child("vnet", "cidr")
		if infra.Networks.VNet.CIDR == nil {
			// Use worker/subnet cidr as cidr for the vnet.
			allErrs = append(allErrs, workerCIDR.ValidateSubset(nodes)...)
			allErrs = append(allErrs, workerCIDR.ValidateNotSubset(pods, services)...)
		} else {
			vpcCIDR := cidrvalidation.NewCIDR(*(infra.Networks.VNet.CIDR), cidrPath)
			allErrs = append(allErrs, vpcCIDR.ValidateParse()...)
			allErrs = append(allErrs, vpcCIDR.ValidateSubset(nodes)...)
			allErrs = append(allErrs, vpcCIDR.ValidateSubset(workerCIDR)...)
			allErrs = append(allErrs, vpcCIDR.ValidateNotSubset(pods, services)...)
			allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(cidrPath, *infra.Networks.VNet.CIDR)...)
		}
	}

	if infra.Identity != nil && (infra.Identity.Name == "" || infra.Identity.ResourceGroup == "") {
		allErrs = append(allErrs, field.Invalid(field.NewPath("identity"), infra.Identity, "specifying an identity requires the name of the identity and the resource group which hosts the identity"))
	}

	if nodes != nil {
		allErrs = append(allErrs, nodes.ValidateSubset(workerCIDR)...)
	}

	return allErrs
}

// ValidateInfrastructureConfigUpdate validates a InfrastructureConfig object.
func ValidateInfrastructureConfigUpdate(oldConfig, newConfig *apisazure.InfrastructureConfig, nodesCIDR, podsCIDR, servicesCIDR *string) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.ResourceGroup, oldConfig.ResourceGroup, field.NewPath("resourceGroup"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.Networks, oldConfig.Networks, field.NewPath("networks"))...)

	return allErrs
}
