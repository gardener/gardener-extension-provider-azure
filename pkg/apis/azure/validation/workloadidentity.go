// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
)

// ValidateWorkloadIdentityConfig checks whether the given workload identity configuration contains expected fields and values.
func ValidateWorkloadIdentityConfig(config *apisazure.WorkloadIdentityConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(config.ClientID) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("clientID"), "clientID is required"))
	}
	if len(config.TenantID) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("tenantID"), "tenantID is required"))
	}
	if len(config.SubscriptionID) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("subscriptionID"), "subscriptionID is required"))
	}

	// clientID, tenantID and subscriptionID must be valid GUIDs,
	// see https://docs.microsoft.com/en-us/rest/api/securitycenter/locations/get
	if !guidRegex.Match([]byte(config.ClientID)) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("clientID"), config.ClientID, "clientID should be a valid GUID"))
	}
	if !guidRegex.Match([]byte(config.TenantID)) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("tenantID"), config.TenantID, "tenantID should be a valid GUID"))
	}
	if !guidRegex.Match([]byte(config.SubscriptionID)) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("subscriptionID"), config.SubscriptionID, "subscriptionID should be a valid GUID"))
	}

	return allErrs
}

// ValidateWorkloadIdentityConfigUpdate validates updates on WorkloadIdentityConfig object.
func ValidateWorkloadIdentityConfigUpdate(oldConfig, newConfig *apisazure.WorkloadIdentityConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.SubscriptionID, oldConfig.SubscriptionID, fldPath.Child("subscriptionID"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newConfig.TenantID, oldConfig.TenantID, fldPath.Child("tenantID"))...)
	allErrs = append(allErrs, ValidateWorkloadIdentityConfig(newConfig, fldPath)...)

	return allErrs
}
