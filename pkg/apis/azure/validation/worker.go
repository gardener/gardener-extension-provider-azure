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

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"

	apiazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
)

// ValidateWorkerConfig validates a WorkerConfig object.
func ValidateWorkerConfig(workerConfig *apiazure.WorkerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if workerConfig != nil {
		allErrs = append(allErrs, validateNodeTemplate(workerConfig.NodeTemplate, fldPath)...)
	}

	return allErrs
}

func validateNodeTemplate(nodeTemplate *extensionsv1alpha1.NodeTemplate, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if nodeTemplate == nil {
		return nil
	}
	for _, capacityAttribute := range []corev1.ResourceName{corev1.ResourceCPU, "gpu", corev1.ResourceMemory} {
		value, ok := nodeTemplate.Capacity[capacityAttribute]
		if !ok {
			allErrs = append(allErrs, field.Required(fldPath.Child("nodeTemplate").Child("capacity"), fmt.Sprintf("%s is a mandatory field", capacityAttribute)))
			continue
		}
		allErrs = append(allErrs, validateResourceQuantityValue(capacityAttribute, value, fldPath.Child("nodeTemplate").Child("capacity").Child(string(capacityAttribute)))...)
	}

	return allErrs
}

func validateResourceQuantityValue(key corev1.ResourceName, value resource.Quantity, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if value.Cmp(resource.Quantity{}) < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, value.String(), fmt.Sprintf("%s value must not be negative", key)))
	}

	return allErrs
}
