// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	azurevalidation "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"
)

// seedValidator validates create and update operations on Seed resources,
// validating the immutability settings passed through the backup configuration.
type seedValidator struct{}

// NewSeedValidator returns a new Validator for Seed resources,
// ensuring backup configuration immutability according to policy.
func NewSeedValidator() extensionswebhook.Validator {
	return &seedValidator{}
}

// Validate validates the Seed resource during create or update operations.
// It validates the seed resource during creation by only allowing
// appropriate values for the `spec.backup.immutability`.
// It also validates the seed resource during updates by disallowing
// disabling immutable settings, reducing retention periods, or changing retention types.
func (s *seedValidator) Validate(_ context.Context, newObj, oldObj client.Object) error {
	newSeed, ok := newObj.(*core.Seed)
	if !ok {
		return fmt.Errorf("wrong object type %T for new object", newObj)
	}

	if oldObj != nil {
		oldSeed, ok := oldObj.(*core.Seed)
		if !ok {
			return fmt.Errorf("wrong object type %T for old object", oldObj)
		}

		return s.validateUpdate(oldSeed, newSeed).ToAggregate()
	}

	return s.validateCreate(newSeed).ToAggregate()
}

// validateCreate validates the Seed object before creation.
// It validates the `spec.backup.immutability` configuration passed.
func (s *seedValidator) validateCreate(seed *core.Seed) field.ErrorList {
	allErrs := field.ErrorList{}

	if seed.Spec.Backup != nil {
		backupPath := field.NewPath("spec", "backup")
		allErrs = append(allErrs, azurevalidation.ValidateBackupBucketCredentialsRef(seed.Spec.Backup.CredentialsRef, backupPath.Child("credentialsRef"))...)

		if seed.Spec.Backup.ProviderConfig != nil {
			providerConfigPath := backupPath.Child("providerConfig")
			backupBucketConfig, err := helper.BackupConfigFromProviderConfig(seed.Spec.Backup.ProviderConfig)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(providerConfigPath, seed.Spec.Backup.ProviderConfig, fmt.Errorf("failed to decode provider config: %w", err).Error()))
			} else {
				allErrs = append(allErrs, azurevalidation.ValidateBackupBucketConfig(&backupBucketConfig, providerConfigPath)...)
			}
		}
	}

	return allErrs
}

func (s *seedValidator) validateUpdate(oldSeed, newSeed *core.Seed) field.ErrorList {
	// create validations need to run if the old seed did not have backups/immutable backups
	if oldSeed.Spec.Backup == nil || oldSeed.Spec.Backup.ProviderConfig == nil {
		return s.validateCreate(newSeed)
	}

	var (
		allErrs            = field.ErrorList{}
		backupPath         = field.NewPath("spec", "backup")
		providerConfigPath = backupPath.Child("providerConfig")
	)

	oldBackupBucketConfig, err := helper.BackupConfigFromProviderConfig(oldSeed.Spec.Backup.ProviderConfig)
	if err != nil {
		return append(allErrs, field.InternalError(providerConfigPath, fmt.Errorf("failed to decode old provider config: %w", err)))
	}

	var newBackupBucketConfig azure.BackupBucketConfig
	if newSeed != nil && newSeed.Spec.Backup != nil && newSeed.Spec.Backup.ProviderConfig != nil {
		newBackupBucketConfig, err = helper.BackupConfigFromProviderConfig(newSeed.Spec.Backup.ProviderConfig)
		if err != nil {
			return append(allErrs, field.InternalError(providerConfigPath, fmt.Errorf("failed to decode new provider config: %w", err)))
		}
	}

	allErrs = append(allErrs, azurevalidation.ValidateBackupBucketConfig(&newBackupBucketConfig, providerConfigPath)...)
	allErrs = append(allErrs, azurevalidation.ValidateBackupBucketConfigUpdate(&oldBackupBucketConfig, &newBackupBucketConfig, providerConfigPath)...)
	allErrs = append(allErrs, azurevalidation.ValidateBackupBucketCredentialsRef(newSeed.Spec.Backup.CredentialsRef, backupPath.Child("credentialsRef"))...)

	return allErrs
}
