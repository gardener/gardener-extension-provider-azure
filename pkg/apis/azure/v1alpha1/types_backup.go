// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RetentionType defines the level at which immutability properties are obtained by objects
type RetentionType string

const (
	// BucketLevelImmutability sets the immutability at the bucket level
	BucketLevelImmutability RetentionType = "bucket"
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
	Immutability *ImmutableConfig `json:"immutability,omitempty"`
	// RotationConfig controls the behavior for the rotation of storage account keys.
	// +optional
	RotationConfig *RotationConfig `json:"rotationConfig,omitempty"`
}

// ImmutableConfig represents the immutability configuration for a backup bucket.
type ImmutableConfig struct {
	// RetentionType specifies the type of retention for the backup bucket.
	// Currently allowed values are:
	// - BucketLevelImmutability: The retention policy applies to the entire bucket.
	RetentionType RetentionType `json:"retentionType"`

	// RetentionPeriod specifies the immutability retention period for the backup bucket.
	RetentionPeriod metav1.Duration `json:"retentionPeriod"`

	// Locked indicates whether the immutable retention policy is locked for the backup bucket.
	// If set to true, the retention policy cannot be removed or the retention period reduced, enforcing immutability.
	Locked bool `json:"locked"`
}

// RotationConfig controls the behavior for the rotation of storage account keys.
type RotationConfig struct {
	// RotationPeriod is the period after the creation of the currently used key, that a key rotation will be triggered.
	RotationPeriodDays int32 `json:"rotationPeriodDays"`
	// ExpirationPeriod sets the policy on the storage account to expire stale storage account keys. Can only be configured if `rotationPeriod` is configured.
	ExpirationPeriodDays *int32 `json:"expirationPeriodDays,omitempty"`
}
