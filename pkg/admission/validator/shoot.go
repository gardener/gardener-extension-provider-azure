// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"
	"reflect"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	azurevalidation "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"
)

var (
	metaDataPath    = field.NewPath("metadata")
	specPath        = field.NewPath("spec")
	nwPath          = specPath.Child("networking")
	providerPath    = specPath.Child("provider")
	infraConfigPath = providerPath.Child("infrastructureConfig")
	cpConfigPath    = providerPath.Child("controlPlaneConfig")
	workersPath     = providerPath.Child("workers")
)

// shoot validates shoots
type shoot struct {
	client         client.Client
	decoder        runtime.Decoder
	lenientDecoder runtime.Decoder
}

// NewShootValidator returns a new instance of a shoot validator.
func NewShootValidator(mgr manager.Manager) extensionswebhook.Validator {
	return &shoot{
		client:         mgr.GetClient(),
		decoder:        serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
		lenientDecoder: serializer.NewCodecFactory(mgr.GetScheme()).UniversalDecoder(),
	}
}

// Validate validates the given shoot object
func (s *shoot) Validate(ctx context.Context, newObj, oldObj client.Object) error {
	shoot, ok := newObj.(*core.Shoot)
	if !ok {
		return fmt.Errorf("wrong object type %T", newObj)
	}

	// skip validation if it's a workerless Shoot
	if gardencorehelper.IsWorkerless(shoot) {
		return nil
	}

	cloudProfile := &gardencorev1beta1.CloudProfile{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: shoot.Spec.CloudProfileName}, cloudProfile); err != nil {
		return err
	}

	if oldObj != nil {
		oldShoot, ok := oldObj.(*core.Shoot)
		if !ok {
			return fmt.Errorf("wrong object type %T for old object", oldObj)
		}
		return s.validateUpdate(oldShoot, shoot, cloudProfile)
	}

	return s.validateCreation(ctx, shoot, cloudProfile)
}

func (s *shoot) validateCreation(_ context.Context, shoot *core.Shoot, cloudProfile *gardencorev1beta1.CloudProfile) error {
	infraConfig, err := checkAndDecodeInfrastructureConfig(s.decoder, shoot.Spec.Provider.InfrastructureConfig, infraConfigPath)
	if err != nil {
		return err
	}

	var cpConfig *api.ControlPlaneConfig
	if shoot.Spec.Provider.ControlPlaneConfig != nil {
		cpConfig, err = decodeControlPlaneConfig(s.decoder, shoot.Spec.Provider.ControlPlaneConfig)
		if err != nil {
			return err
		}
	}

	return s.validateShoot(shoot, nil, infraConfig, cloudProfile, cpConfig).ToAggregate()
}

func (s *shoot) validateShoot(shoot *core.Shoot, oldInfraConfig, infraConfig *api.InfrastructureConfig, cloudProfile *gardencorev1beta1.CloudProfile, cpConfig *api.ControlPlaneConfig) field.ErrorList {
	allErrs := field.ErrorList{}

	// Network validation
	allErrs = append(allErrs, azurevalidation.ValidateNetworking(shoot.Spec.Networking, nwPath)...)

	if infraConfig != nil {
		// Cloudprofile validation
		allErrs = append(allErrs, azurevalidation.ValidateInfrastructureConfigAgainstCloudProfile(oldInfraConfig, infraConfig, shoot.Spec.Region, cloudProfile, infraConfigPath)...)
		// Provider validation
		allErrs = append(allErrs, azurevalidation.ValidateInfrastructureConfig(infraConfig, shoot.Spec.Networking, helper.HasShootVmoAlphaAnnotation(shoot.Annotations), infraConfigPath)...)
	}
	if cpConfig != nil {
		allErrs = append(allErrs, azurevalidation.ValidateControlPlaneConfig(cpConfig, shoot.Spec.Kubernetes.Version, cpConfigPath)...)
	}

	// Shoot workers
	allErrs = append(allErrs, azurevalidation.ValidateWorkers(shoot.Spec.Provider.Workers, infraConfig, workersPath)...)

	for i, worker := range shoot.Spec.Provider.Workers {
		workerFldPath := workersPath.Index(i)
		workerConfig, err := decodeWorkerConfig(s.decoder, worker.ProviderConfig)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(workerFldPath.Child("providerConfig"), err, "invalid providerConfig"))
		} else {
			allErrs = append(allErrs, azurevalidation.ValidateWorkerConfig(workerConfig, workerFldPath.Child("providerConfig"))...)
		}
	}

	return allErrs
}

func (s *shoot) validateUpdate(oldShoot, shoot *core.Shoot, cloudProfile *gardencorev1beta1.CloudProfile) error {
	// Decode the new infrastructure config.
	if shoot.Spec.Provider.InfrastructureConfig == nil {
		return field.Required(infraConfigPath, "InfrastructureConfig must be set for Azure shoots")
	}
	infraConfig, err := checkAndDecodeInfrastructureConfig(s.decoder, shoot.Spec.Provider.InfrastructureConfig, infraConfigPath)
	if err != nil {
		return err
	}

	// Decode the old infrastructure config.
	if oldShoot.Spec.Provider.InfrastructureConfig == nil {
		return field.InternalError(infraConfigPath, fmt.Errorf("InfrastructureConfig is not available on old shoot"))
	}
	oldInfraConfig, err := checkAndDecodeInfrastructureConfig(s.lenientDecoder, oldShoot.Spec.Provider.InfrastructureConfig, infraConfigPath)
	if err != nil {
		return err
	}

	// Decode the new controlplane config
	var cpConfig *api.ControlPlaneConfig
	if shoot.Spec.Provider.ControlPlaneConfig != nil {
		cpConfig, err = decodeControlPlaneConfig(s.decoder, shoot.Spec.Provider.ControlPlaneConfig)
		if err != nil {
			return err
		}
	}

	var allErrs = field.ErrorList{}
	if !reflect.DeepEqual(oldInfraConfig, infraConfig) {
		allErrs = append(allErrs, azurevalidation.ValidateInfrastructureConfigUpdate(oldInfraConfig, infraConfig, metaDataPath)...)
	}

	allErrs = append(allErrs, azurevalidation.ValidateVmoConfigUpdate(helper.HasShootVmoAlphaAnnotation(oldShoot.Annotations), helper.HasShootVmoAlphaAnnotation(shoot.Annotations), metaDataPath)...)
	allErrs = append(allErrs, azurevalidation.ValidateWorkersUpdate(oldShoot.Spec.Provider.Workers, shoot.Spec.Provider.Workers, workersPath)...)

	allErrs = append(allErrs, s.validateShoot(shoot, oldInfraConfig, infraConfig, cloudProfile, cpConfig)...)

	return allErrs.ToAggregate()
}
