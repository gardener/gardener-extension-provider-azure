// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

// Package v1 provides stub types for testdata.
package v1

// TypeMeta describes an individual object in an API response or request.
type TypeMeta struct {
	Kind       string
	APIVersion string
}
