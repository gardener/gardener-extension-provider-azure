// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	azurevalidation "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/validation"
)

type secret struct{}

// NewSecretValidator returns a new instance of an infrastructure secret validator.
func NewSecretValidator() extensionswebhook.Validator {
	return &secret{}
}

// Validate checks whether the given new secret contains valid Azure credentials.
func (s *secret) Validate(_ context.Context, newObj, oldObj client.Object) error {
	var oldSecret *corev1.Secret
	secret, ok := newObj.(*corev1.Secret)
	if !ok {
		return fmt.Errorf("wrong object type %T", newObj)
	}

	if oldObj != nil {
		oldSecret, ok = oldObj.(*corev1.Secret)
		if !ok {
			return fmt.Errorf("wrong object type %T for old object", oldObj)
		}

		if equality.Semantic.DeepEqual(secret.Data, oldSecret.Data) {
			return nil
		}
	}

	secretPath := field.NewPath("secret")
	if errs := azurevalidation.ValidateCloudProviderSecret(secret, oldSecret, secretPath); len(errs) > 0 {
		return errs.ToAggregate()
	}

	return nil
}
