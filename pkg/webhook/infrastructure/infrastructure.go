// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type infraMutator struct {
	mutators []extensionswebhook.Mutator
}

// NewInfraMutator returns a new Infrastructure infraMutator that calls the
// Mutate func of all passed mutators
func NewInfraMutator(mutators []extensionswebhook.Mutator) extensionswebhook.Mutator {
	return &infraMutator{
		mutators: mutators,
	}
}

// Mutate mutates the given object using the mutateFunc
func (m *infraMutator) Mutate(ctx context.Context, new, old client.Object) error {
	for _, mutator := range m.mutators {
		err := mutator.Mutate(ctx, new, old)
		if err != nil {
			return err
		}
	}
	return nil
}
