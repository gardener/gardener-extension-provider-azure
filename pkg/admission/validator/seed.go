package validator

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	azurevalidation "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
)

// seedValidator validates create and update operations on Seed resources,
// validating the immutability settings passed through the backup configuration.
type seedValidator struct {
	client         client.Client
	decoder        runtime.Decoder
	lenientDecoder runtime.Decoder
}

// NewSeedValidator returns a new Validator for Seed resources,
// ensuring backup configuration immutability according to policy.
func NewSeedValidator(mgr manager.Manager) extensionswebhook.Validator {
	return &seedValidator{
		client:         mgr.GetClient(),
		decoder:        serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
		lenientDecoder: serializer.NewCodecFactory(mgr.GetScheme()).UniversalDecoder(),
	}
}

// Validate validates the Seed resource during create or update operations.
// It validates the seed resource during creation by only allowing
// appropriate values for the `spec.backup.immutability`.
// It also validates the seed resource during updates by disallowing
// disabling immutable settings, reducing retention periods, or changing retention types.
func (s *seedValidator) Validate(ctx context.Context, newObj, oldObj client.Object) error {
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
	var (
		allErrs               = field.ErrorList{}
		providerConfigfldPath = field.NewPath("spec", "backup", "providerConfig")
	)

	if seed.Spec.Backup == nil || seed.Spec.Backup.ProviderConfig == nil {
		return allErrs
	}

	backupBucketConfig, err := helper.BackupConfigFromProviderConfig(seed.Spec.Backup.ProviderConfig)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(providerConfigfldPath, seed.Spec.Backup.ProviderConfig, fmt.Errorf("failed to decode new provider config: %v", err).Error()))
		return allErrs
	}

	allErrs = append(allErrs, azurevalidation.ValidateBackupBucketConfig(&backupBucketConfig, providerConfigfldPath)...)

	return allErrs
}

func (s *seedValidator) validateUpdate(oldSeed, newSeed *core.Seed) field.ErrorList {
	var (
		allErrs               = field.ErrorList{}
		providerConfigfldPath = field.NewPath("spec", "backup", "providerConfig")
	)

	// create validations need to run if the old seed did not have backups/immutable backups
	if oldSeed.Spec.Backup == nil || oldSeed.Spec.Backup.ProviderConfig == nil {
		return s.validateCreate(newSeed)
	}

	oldBackupBucketConfig, err := helper.BackupConfigFromProviderConfig(oldSeed.Spec.Backup.ProviderConfig)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(providerConfigfldPath, oldSeed.Spec.Backup.ProviderConfig, fmt.Errorf("failed to decode old provider config: %v", err).Error()))
		return allErrs
	}

	var newBackupBucketConfig azure.BackupBucketConfig
	if newSeed != nil && newSeed.Spec.Backup != nil && newSeed.Spec.Backup.ProviderConfig != nil {
		newBackupBucketConfig, err = helper.BackupConfigFromProviderConfig(newSeed.Spec.Backup.ProviderConfig)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(providerConfigfldPath, newSeed.Spec.Backup.ProviderConfig, fmt.Errorf("failed to decode new provider config: %v", err).Error()))
			return allErrs
		}
	}

	allErrs = append(allErrs, azurevalidation.ValidateBackupBucketConfig(&newBackupBucketConfig, providerConfigfldPath)...)
	allErrs = append(allErrs, s.validateImmutabilityUpdate(&oldBackupBucketConfig, &newBackupBucketConfig, providerConfigfldPath)...)

	return allErrs
}

// validateImmutability validates immutability constraints.
func (s *seedValidator) validateImmutabilityUpdate(oldConfig, newConfig *azure.BackupBucketConfig, fldPath *field.Path) field.ErrorList {
	var (
		allErrs          = field.ErrorList{}
		immutabilityPath = fldPath.Child("immutability")
	)

	if oldConfig.Immutability == nil || !oldConfig.Immutability.Locked {
		return allErrs
	}

	if newConfig == nil || newConfig.Immutability == nil || *newConfig.Immutability == (azure.ImmutableConfig{}) {
		allErrs = append(allErrs, field.Invalid(immutabilityPath, newConfig, "immutability cannot be disabled once it is locked"))
		return allErrs
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
