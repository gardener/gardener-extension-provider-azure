// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

// Package azure contains stub API types for apitypecheck testdata.
package azure

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// WidgetConfig is an example input config type with three fields.
type WidgetConfig struct {
	metav1.TypeMeta

	// Color is the color of the widget.
	Color string

	// Size is the size of the widget.
	Size int32

	// Tags are optional labels.
	// +optional
	Tags []string
}

// WidgetStatus is output-only; its fields do not need validation.
type WidgetStatus struct {
	metav1.TypeMeta

	// Phase describes the current phase.
	Phase string
}
