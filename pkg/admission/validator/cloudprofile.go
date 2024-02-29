// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	azurevalidation "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"
)

// NewCloudProfileValidator returns a new instance of a cloud profile validator.
func NewCloudProfileValidator(mgr manager.Manager) extensionswebhook.Validator {
	return &cloudProfile{
		decoder: serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
	}
}

type cloudProfile struct {
	decoder runtime.Decoder
}

// Validate validates the given CloudProfile objects.
func (cp *cloudProfile) Validate(_ context.Context, newObj, _ client.Object) error {
	cloudProfile, ok := newObj.(*core.CloudProfile)
	if !ok {
		return fmt.Errorf("wrong object type %T", newObj)
	}

	providerConfigPath := field.NewPath("spec").Child("providerConfig")
	if cloudProfile.Spec.ProviderConfig == nil {
		return field.Required(providerConfigPath, "providerConfig must be set for Azure cloud profiles")
	}

	cpConfig, err := decodeCloudProfileConfig(cp.decoder, cloudProfile.Spec.ProviderConfig)
	if err != nil {
		return err
	}

	return azurevalidation.ValidateCloudProfileConfig(cpConfig, providerConfigPath).ToAggregate()
}
