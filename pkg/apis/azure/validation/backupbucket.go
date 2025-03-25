// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
)

// ValidateBackupBucketConfig validates a BackupBucketConfig object.
func ValidateBackupBucketConfig(backupBucketConfig *apisazure.BackupBucketConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if backupBucketConfig == nil {
		return nil
	}

	allErrs = append(allErrs, validateImmutability(backupBucketConfig.Immutability, fldPath.Child("immutability"))...)
	allErrs = append(allErrs, validateKeyRotation(backupBucketConfig.RotationConfig, fldPath.Child("rotationConfig"))...)

	return allErrs
}
func validateKeyRotation(cfg *apisazure.RotationConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if cfg == nil {
		return allErrs
	}
	if cfg.RotationPeriodDays < 1 {
		return append(allErrs, field.Required(fldPath.Child("rotationPeriodDays"), "rotationPeriod must be configured if key rotation is enabled"))
	}

	if cfg.RotationPeriodDays < 2 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("rotationPeriodDays"), cfg.RotationPeriodDays, "must be equal or greater than 2 days"))
	}

	if cfg.ExpirationPeriodDays != nil {
		if ptr.Deref(cfg.ExpirationPeriodDays, 0) <= cfg.RotationPeriodDays {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("expirationPeriodDays"), cfg.ExpirationPeriodDays, "must be greater than the rotation period"))
		}
	}

	return allErrs
}

func validateImmutability(immutabilityCfg *apisazure.ImmutableConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if immutabilityCfg == nil {
		return allErrs
	}

	if immutabilityCfg.RetentionType != apisazure.BucketLevelImmutability {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("retentionType"), immutabilityCfg.RetentionType, "must be 'bucket'"))
	}

	// Azure Blob Storage immutability period can only be set in days
	if immutabilityCfg.RetentionPeriod.Duration < 24*time.Hour {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("retentionPeriod"), immutabilityCfg.RetentionPeriod.Duration.String(), "must be greater than 24h"))
	}

	if immutabilityCfg.RetentionPeriod.Duration%(24*time.Hour) != 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("retentionPeriod"), immutabilityCfg.RetentionPeriod.Duration.String(), "must be a positive integer multiple of 24h"))
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
