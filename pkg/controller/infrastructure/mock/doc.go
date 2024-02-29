// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
//go:generate mockgen -package infrastructure -destination=mocks.go github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure Reconciler

package infrastructure
