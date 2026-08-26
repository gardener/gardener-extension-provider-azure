// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0
//go:generate mockgen -package vmss -destination=mocks.go github.com/gardener/gardener-extension-provider-azure/pkg/azure/client Vmss

package vmss
