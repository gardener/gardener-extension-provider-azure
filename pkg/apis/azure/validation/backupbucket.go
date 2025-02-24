// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"time"

	"k8s.io/apimachinery/pkg/util/validation/field"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
)

// ValidateBackupBucketConfig validates a BackupBucketConfig object.
func ValidateBackupBucketConfig(backupBucketConfig *apisazure.BackupBucketConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if backupBucketConfig == nil || backupBucketConfig.Immutability == nil {
		return allErrs
	}

	if backupBucketConfig.Immutability.RetentionType != apisazure.BucketLevelImmutability {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("immutability", "retentionType"), backupBucketConfig.Immutability.RetentionType, "must be 'bucket'"))
	}

	// Azure Blob Storage immutability period can only be set in days
	if backupBucketConfig.Immutability.RetentionPeriod.Duration < 24*time.Hour {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("immutability", "retentionPeriod"), backupBucketConfig.Immutability.RetentionPeriod.Duration.String(), "must be greater than 24h"))
	}

	if backupBucketConfig.Immutability.RetentionPeriod.Duration%(24*time.Hour) != 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("immutability", "retentionPeriod"), backupBucketConfig.Immutability.RetentionPeriod.Duration.String(), "must be a positive integer multiple of 24h"))
	}

	return allErrs
}
