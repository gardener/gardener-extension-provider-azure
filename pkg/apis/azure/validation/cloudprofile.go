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
	"fmt"
	"strings"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateCloudProfileConfig validates a CloudProfileConfig object.
func ValidateCloudProfileConfig(cloudProfile *apisazure.CloudProfileConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateDomainCount(cloudProfile.CountFaultDomains, fldPath.Child("countFaultDomains"))...)
	allErrs = append(allErrs, validateDomainCount(cloudProfile.CountUpdateDomains, fldPath.Child("countUpdateDomains"))...)

	machineImagesPath := fldPath.Child("machineImages")
	if len(cloudProfile.MachineImages) == 0 {
		allErrs = append(allErrs, field.Required(machineImagesPath, "must provide at least one machine image"))
	}
	for i, machineImage := range cloudProfile.MachineImages {
		idxPath := machineImagesPath.Index(i)

		if len(machineImage.Name) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), "must provide a name"))
		}

		if len(machineImage.Versions) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("versions"), fmt.Sprintf("must provide at least one version for machine image %q", machineImage.Name)))
		}
		for j, version := range machineImage.Versions {
			jdxPath := idxPath.Child("versions").Index(j)

			if len(version.Version) == 0 {
				allErrs = append(allErrs, field.Required(jdxPath.Child("version"), "must provide a version"))
			}
			if (version.URN == nil && version.ID == nil) || (version.URN != nil && version.ID != nil) {
				allErrs = append(allErrs, field.Required(jdxPath, "must provide either urn or id"))
			}
			if version.URN != nil {
				if len(*version.URN) == 0 {
					allErrs = append(allErrs, field.Required(jdxPath.Child("urn"), "urn cannot be empty when defined"))
				} else if len(strings.Split(*version.URN, ":")) != 4 {
					allErrs = append(allErrs, field.Invalid(jdxPath.Child("urn"), version.URN, "please use the format `Publisher:Offer:Sku:Version` for the urn"))
				}
			}
			if version.ID != nil && len(*version.ID) == 0 {
				allErrs = append(allErrs, field.Required(jdxPath.Child("id"), "id cannot be empty when defined"))
			}
		}
	}

	return allErrs
}

func validateDomainCount(domainCount []apisazure.DomainCount, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(domainCount) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one domain count"))
	}

	for i, count := range domainCount {
		idxPath := fldPath.Index(i)
		regionPath := idxPath.Child("region")
		countPath := idxPath.Child("count")

		if len(count.Region) == 0 {
			allErrs = append(allErrs, field.Required(regionPath, "must provide a region"))
		}
		if count.Count < 0 {
			allErrs = append(allErrs, field.Invalid(countPath, count.Count, "count must not be negative"))
		}
	}

	return allErrs
}
