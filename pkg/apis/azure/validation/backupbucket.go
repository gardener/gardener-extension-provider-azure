// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
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

// ValidateBackupBucketConfigUpdate validates updates to the BackupBucketConfig.
func ValidateBackupBucketConfigUpdate(oldConfig, newConfig *apisazure.BackupBucketConfig, fldPath *field.Path) field.ErrorList {
	var (
		allErrs          = field.ErrorList{}
		immutabilityPath = fldPath.Child("immutability")
	)

	if oldConfig.Immutability == nil || !oldConfig.Immutability.Locked {
		return allErrs
	}

	if newConfig == nil || newConfig.Immutability == nil || *newConfig.Immutability == (apisazure.ImmutableConfig{}) {
		return append(allErrs, field.Invalid(immutabilityPath, newConfig, "immutability cannot be disabled once it is locked"))
	}

	if !newConfig.Immutability.Locked {
		allErrs = append(allErrs, field.Forbidden(immutabilityPath.Child("locked"), "immutable retention policy lock cannot be unlocked once it is locked"))
	} else if newConfig.Immutability.RetentionPeriod.Duration < oldConfig.Immutability.RetentionPeriod.Duration {
		allErrs = append(allErrs, field.Forbidden(
			immutabilityPath.Child("retentionPeriod"),
			fmt.Sprintf("reducing the retention period from %v to %v is prohibited when the immutable retention policy is locked",
				oldConfig.Immutability.RetentionPeriod.Duration,
				newConfig.Immutability.RetentionPeriod.Duration,
			),
		))
	}

	return allErrs
}
