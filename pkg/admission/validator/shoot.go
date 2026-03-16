// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"
	"reflect"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorehelper "github.com/gardener/gardener/pkg/api/core/helper"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	azurevalidation "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

var (
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
	apiReader      client.Reader
	decoder        runtime.Decoder
	lenientDecoder runtime.Decoder
}

// NewShootValidator returns a new instance of a shoot validator.
func NewShootValidator(mgr manager.Manager) extensionswebhook.Validator {
	return &shoot{
		client:         mgr.GetClient(),
		apiReader:      mgr.GetAPIReader(),
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

	shootV1Beta1 := &gardencorev1beta1.Shoot{}
	err := gardencorev1beta1.Convert_core_Shoot_To_v1beta1_Shoot(shoot, shootV1Beta1, nil)
	if err != nil {
		return err
	}
	cloudProfile, err := gardener.GetCloudProfile(ctx, s.client, shootV1Beta1)
	if err != nil {
		return err
	}
	if cloudProfile == nil {
		return fmt.Errorf("cloudprofile could not be found")
	}

	if oldObj != nil {
		oldShoot, ok := oldObj.(*core.Shoot)
		if !ok {
			return fmt.Errorf("wrong object type %T for old object", oldObj)
		}
		return s.validateUpdate(ctx, shoot, oldShoot, &cloudProfile.Spec)
	}

	return s.validateCreation(ctx, shoot, &cloudProfile.Spec)
}

func (s *shoot) validateCreation(ctx context.Context, shoot *core.Shoot, cloudProfileSpec *gardencorev1beta1.CloudProfileSpec) error {
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

	return s.validateShoot(ctx, shoot, nil, infraConfig, cloudProfileSpec, cpConfig).ToAggregate()
}

func (s *shoot) validateShoot(ctx context.Context, shoot *core.Shoot, oldInfraConfig, infraConfig *api.InfrastructureConfig, cloudProfileSpec *gardencorev1beta1.CloudProfileSpec, cpConfig *api.ControlPlaneConfig) field.ErrorList {
	allErrs := field.ErrorList{}

	// Network validation
	allErrs = append(allErrs, azurevalidation.ValidateNetworking(shoot.Spec.Networking, nwPath)...)

	if infraConfig != nil {
		// Cloudprofile validation
		allErrs = append(allErrs, azurevalidation.ValidateInfrastructureConfigAgainstCloudProfile(oldInfraConfig, infraConfig, shoot.Spec.Region, cloudProfileSpec, infraConfigPath)...)
		// Provider validation
		allErrs = append(allErrs, azurevalidation.ValidateInfrastructureConfig(infraConfig, shoot, infraConfigPath)...)
	}
	if cpConfig != nil {
		allErrs = append(allErrs, azurevalidation.ValidateControlPlaneConfig(cpConfig, shoot.Spec.Kubernetes.Version, cpConfigPath)...)
	}

	// DNS validation
	allErrs = append(allErrs, s.validateDNS(ctx, shoot)...)

	// Shoot workers
	allErrs = append(allErrs, azurevalidation.ValidateWorkers(shoot.Spec.Provider.Workers, infraConfig, workersPath)...)

	for i, worker := range shoot.Spec.Provider.Workers {
		workerFldPath := workersPath.Index(i)
		workerConfig, err := decodeWorkerConfig(s.decoder, worker.ProviderConfig)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(workerFldPath.Child("providerConfig"), err, "invalid providerConfig"))
		} else {
			allErrs = append(allErrs, azurevalidation.ValidateWorkerConfig(workerConfig, worker.DataVolumes, workerFldPath.Child("providerConfig"))...)
		}
	}

	return allErrs
}

func (s *shoot) validateUpdate(ctx context.Context, shoot, oldShoot *core.Shoot, cloudProfileSpec *gardencorev1beta1.CloudProfileSpec) error {
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
		allErrs = append(allErrs, azurevalidation.ValidateInfrastructureConfigUpdate(oldInfraConfig, infraConfig, oldShoot, infraConfigPath)...)
	}

	allErrs = append(allErrs, azurevalidation.ValidateWorkersUpdate(oldShoot.Spec.Provider.Workers, shoot.Spec.Provider.Workers, workersPath)...)

	allErrs = append(allErrs, s.validateShoot(ctx, shoot, oldInfraConfig, infraConfig, cloudProfileSpec, cpConfig)...)

	return allErrs.ToAggregate()
}

// validateDNS validates all azure-dns provider entries in the Shoot spec.
func (s *shoot) validateDNS(ctx context.Context, shoot *core.Shoot) field.ErrorList {
	allErrs := field.ErrorList{}

	if shoot.Spec.DNS == nil {
		return allErrs
	}

	providersPath := specPath.Child("dns").Child("providers")

	for i, p := range shoot.Spec.DNS.Providers {
		if p.Type == nil || *p.Type != azure.DNSType {
			continue
		}

		// skip non-primary providers
		if p.Primary == nil || !*p.Primary {
			continue
		}

		providerFldPath := providersPath.Index(i)

		// TODO(vpnachev): Enable this validation once the extension does not support github.com/gardener/gardener < v1.135.0
		// if p.CredentialsRef == nil {
		// 	allErrs = append(allErrs, field.Required(providerFldPath.Child("credentialsRef"), "must be set"))
		// }

		if p.CredentialsRef != nil {
			credentialsFldPath := providerFldPath.Child("credentialsRef")

			credentials, err := kubernetes.GetCredentialsByCrossVersionObjectReference(ctx, s.apiReader, *p.CredentialsRef, shoot.GetNamespace())
			if err != nil {
				if apierrors.IsNotFound(err) {
					allErrs = append(allErrs, field.NotFound(credentialsFldPath, p.CredentialsRef.String()))
				} else {
					allErrs = append(allErrs, field.InternalError(credentialsFldPath, err))
				}
				continue
			}

			switch creds := credentials.(type) {
			case *securityv1alpha1.WorkloadIdentity:
				if err := ValidateWorkloadIdentity(creds, nil); err != nil {
					allErrs = append(allErrs, field.Invalid(credentialsFldPath, p.CredentialsRef.String(), err.Error()))
				}
			case *corev1.Secret:
				if errList := azurevalidation.ValidateDNSProviderSecret(creds, field.NewPath("secret")); len(errList) != 0 {
					allErrs = append(allErrs, field.Invalid(credentialsFldPath, p.CredentialsRef.String(), errList.ToAggregate().Error()))
				}
			default:
				allErrs = append(allErrs, field.Invalid(credentialsFldPath, p.CredentialsRef.String(), "supported credentials types are Secret and WorkloadIdentity"))
			}
		} else { // TODO(vpnachev): Remove the else block once the extension does not support github.com/gardener/gardener < v1.135.0
			secretNameFldPath := providerFldPath.Child("secretName")
			if p.SecretName == nil || *p.SecretName == "" {
				allErrs = append(allErrs, field.Required(secretNameFldPath,
					fmt.Sprintf("secretName must be specified for %v provider", azure.DNSType)))
				continue
			}

			secret := &corev1.Secret{}
			key := client.ObjectKey{Namespace: shoot.Namespace, Name: *p.SecretName}
			if err := s.apiReader.Get(ctx, key, secret); err != nil {
				if apierrors.IsNotFound(err) {
					allErrs = append(allErrs, field.Invalid(secretNameFldPath,
						*p.SecretName, "referenced secret not found"))
				} else {
					allErrs = append(allErrs, field.InternalError(secretNameFldPath, err))
				}
				continue
			}

			if errList := azurevalidation.ValidateDNSProviderSecret(secret, field.NewPath("secret")); len(errList) != 0 {
				allErrs = append(allErrs, field.Invalid(secretNameFldPath, *p.SecretName, errList.ToAggregate().Error()))
			}
		}
	}

	return allErrs
}
