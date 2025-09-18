// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"net/url"
	"regexp"
	"unicode/utf8"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// There is no enum for Azure Service Endpoints, so we use a regex to validate the names.
var (
	resourceGroupNameRegex       = `^[A-Za-z0-9_().-]{1,89}[A-Za-z0-9_()-]$`
	vnetNameRegex                = `^[A-Za-z0-9][\w.-]*[\w]$`
	genericAzureNameRegex        = `^[A-Za-z0-9][\w-]*$`
	serviceEndpointsRegex        = `^Microsoft\.[A-Za-z0-9.]+$`
	storageURIRegex              = `^https://[a-z0-9-]{3,24}\.blob\.core[^\s]+/[^\s]+$`
	urnRegex                     = `^[\w-]+:[\w-]+:[\w.-]+:[\w.-]+$`
	sharedGalleryImageIDRegex    = `^/SharedGalleries/[\w-]+/Images/[\w-]+/Versions/[\w.-]+$`
	communityGalleryImageIDRegex = `^/CommunityGalleries/[\w-]+/Images/[\w-]+/Versions/[\w.-]+$`

	validateServiceEndpoint           = combineValidationFuncs(regex(serviceEndpointsRegex), minLength(9), maxLength(120))
	validateResourceGroup             = combineValidationFuncs(regex(resourceGroupNameRegex), maxLength(90))
	validateVnetName                  = combineValidationFuncs(regex(vnetNameRegex), minLength(2), maxLength(64))
	validateGenericName               = combineValidationFuncs(regex(genericAzureNameRegex), minLength(3), maxLength(120))
	validatePublicIP                  = combineValidationFuncs(regex(genericAzureNameRegex), maxLength(80))
	storageURIValidation              = combineValidationFuncs(urlFilter, regex(storageURIRegex), notEmpty)
	urnValidation                     = combineValidationFuncs(regex(urnRegex), notEmpty, maxLength(256))
	sharedGalleryImageIDValidation    = combineValidationFuncs(regex(sharedGalleryImageIDRegex), notEmpty, maxLength(512))
	communityGalleryImageIDValidation = combineValidationFuncs(regex(communityGalleryImageIDRegex), notEmpty, maxLength(512))
)

type validateFunc[T any] func(T, *field.Path) field.ErrorList

// combineValidationFuncs validates a value against a list of filters.
func combineValidationFuncs[T any](filters ...validateFunc[T]) validateFunc[T] {
	return func(t T, fld *field.Path) field.ErrorList {
		var allErrs field.ErrorList
		for _, f := range filters {
			allErrs = append(allErrs, f(t, fld)...)
		}
		return allErrs
	}
}

// regex returns a filterFunc that validates a string against a regular expression.
func regex(regex string) validateFunc[string] {
	compiled := regexp.MustCompile(regex)
	return func(name string, fld *field.Path) field.ErrorList {
		var allErrs field.ErrorList
		if name == "" {
			return allErrs // Allow empty strings to pass through
		}
		if !compiled.MatchString(name) {
			allErrs = append(allErrs, field.Invalid(fld, name, fmt.Sprintf("does not match expected regex %s", compiled.String())))
		}
		return allErrs
	}
}

func minLength(min int) validateFunc[string] {
	return func(name string, fld *field.Path) field.ErrorList {
		var allErrs field.ErrorList
		if l := utf8.RuneCountInString(name); l < min {
			return field.ErrorList{field.Invalid(fld, name, fmt.Sprintf("must not be fewer than %d characters, got %d", min, l))}
		}
		return allErrs
	}
}

func notEmpty(name string, fld *field.Path) field.ErrorList {
	if utf8.RuneCountInString(name) == 0 {
		return field.ErrorList{field.Required(fld, name)}
	}
	return nil
}

func maxLength(max int) validateFunc[string] {
	return func(name string, fld *field.Path) field.ErrorList {
		var allErrs field.ErrorList
		if l := utf8.RuneCountInString(name); l > max {
			return field.ErrorList{field.Invalid(fld, name, fmt.Sprintf("must not be more than %d characters, got %d", max, l))}
		}
		return allErrs
	}
}

func urlFilter(u string, fld *field.Path) field.ErrorList {
	_, err := url.Parse(u)
	if err != nil {
		return field.ErrorList{field.Invalid(fld, u, fmt.Sprintf("must be a valid URL: %v", err))}
	}
	return nil
}

func validateResourceID(rid *arm.ResourceID, resourceType *string, fld *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	if rid == nil {
		return allErrs
	}

	if rid.SubscriptionID != "" {
		if _, err := uuid.Parse(rid.SubscriptionID); err != nil {
			allErrs = append(allErrs, field.Invalid(fld, rid.SubscriptionID, fmt.Sprintf("must be a valid Azure subscription ID: %v", err)))
		}
	}

	if rid.ResourceGroupName != "" {
		allErrs = append(allErrs, validateResourceGroup(rid.ResourceGroupName, fld.Child("resourceGroupName"))...)
	}
	if resourceType != nil {
		allErrs = append(allErrs, validateGenericName(*resourceType, fld.Child("resourceType"))...)
	}

	return allErrs
}
