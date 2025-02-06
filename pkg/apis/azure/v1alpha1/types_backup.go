// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupBucketConfig is the provider-specific configuration for backup buckets/entries
type BackupBucketConfig struct {
	metav1.TypeMeta `json:",inline"`
	// CloudConfiguration contains config that controls which cloud to connect to.
	// +optional
	CloudConfiguration *CloudConfiguration `json:"cloudConfiguration,omitempty"`
	// Immutability defines the immutability config for the backup bucket.
	// +optional
	Immutability *ImmutableConfig `json:"immutability"`
}

// ImmutableConfig represents the immutability configuration for a backup bucket.
type ImmutableConfig struct {
	// RetentionType specifies the type of retention for the backup bucket.
	// Currently allowed values are:
	// - "bucket": The retention policy applies to the entire bucket.
	RetentionType string `json:"retentionType"`

	// RetentionPeriod specifies the immutability retention period for the backup bucket.
	RetentionPeriod metav1.Duration `json:"retentionPeriod"`

	// Locked indicates whether the immutable retention policy is locked for the backup bucket.
	// If set to true, the retention policy cannot be removed or the retention period reduced, enforcing immutability.
	Locked bool `json:"locked"`
}
