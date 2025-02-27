// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
)

var _ = Describe("ValidateBackupBucketConfig", func() {
	var fldPath *field.Path

	BeforeEach(func() {
		fldPath = field.NewPath("spec")
	})

	DescribeTable("validation cases",
		func(config *apisazure.BackupBucketConfig, wantErr bool, errMsg string) {
			errs := ValidateBackupBucketConfig(config, fldPath)
			if wantErr {
				Expect(errs).NotTo(BeEmpty())
				Expect(errs[0].Error()).To(ContainSubstring(errMsg))
			} else {
				Expect(errs).To(BeEmpty())
			}
		},
		Entry("valid config",
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 24 * time.Hour},
				},
			}, false, ""),
		Entry("missing retentionType",
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   "",
					RetentionPeriod: metav1.Duration{Duration: 1 * time.Hour},
				},
			}, true, "must be 'bucket'"),
		Entry("invalid retentionType",
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   "invalid",
					RetentionPeriod: metav1.Duration{Duration: 1 * time.Hour},
				},
			}, true, "must be 'bucket'"),
		Entry("non-positive retentionPeriod",
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 0},
				},
			}, true, "must be greater than 24h"),
		Entry("negative retentionPeriod",
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: -1 * time.Hour},
				},
			}, true, "must be greater than 24h"),
		Entry("empty retentionPeriod",
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{},
				},
			}, true, "must be greater than 24h"),
		Entry("retentionPeriod less than 24 hours",
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 23 * time.Hour},
				},
			}, true, "must be greater than 24h"),
		Entry("retentionPeriod is not a positive integer multiple of 24h",
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 27 * time.Hour},
				},
			}, true, "must be a positive integer multiple of 24h"),
	)
})

var _ = Describe("ValidateBackupBucketConfigUpdate", func() {
	var fldPath *field.Path

	BeforeEach(func() {
		fldPath = field.NewPath("spec")
	})

	DescribeTable("validation cases",
		func(oldConfig, newConfig *apisazure.BackupBucketConfig, wantErr bool, errMsg string) {
			errs := ValidateBackupBucketConfigUpdate(oldConfig, newConfig, fldPath)
			if wantErr {
				Expect(errs).NotTo(BeEmpty())
				Expect(errs[0].Error()).To(ContainSubstring(errMsg))
			} else {
				Expect(errs).To(BeEmpty())
			}
		},
		Entry("no config update - no policy",
			&apisazure.BackupBucketConfig{
				Immutability: nil,
			},
			&apisazure.BackupBucketConfig{
				Immutability: nil,
			}, false, ""),
		Entry("no config update - unlocked policy",
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 24 * time.Hour},
				},
			},
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 24 * time.Hour},
				},
			}, false, ""),
		Entry("no config update - locked policy",
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 24 * time.Hour},
					Locked:          true,
				},
			},
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 24 * time.Hour},
					Locked:          true,
				},
			}, false, ""),
		Entry("valid config update: unlocked policy extension",
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 24 * time.Hour},
				},
			},
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 48 * time.Hour},
				},
			}, false, ""),
		Entry("valid config update: unlocked policy reduction",
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 48 * time.Hour},
				},
			},
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 24 * time.Hour},
				},
			}, false, ""),
		Entry("valid config update:  unlocked policy deletion",
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 48 * time.Hour},
				},
			},
			&apisazure.BackupBucketConfig{
				Immutability: nil,
			}, false, ""),
		Entry("valid config update: locking an unlocked policy",
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 24 * time.Hour},
				},
			},
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 24 * time.Hour},
					Locked:          true,
				},
			}, false, ""),
		Entry("valid config update: locked policy extension",
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 24 * time.Hour},
					Locked:          true,
				},
			},
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 48 * time.Hour},
					Locked:          true,
				},
			}, false, ""),
		Entry("invalid config update: unlocking a locked policy",
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 24 * time.Hour},
					Locked:          true,
				},
			},
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 24 * time.Hour},
					Locked:          false,
				},
			}, true, "immutable retention policy lock cannot be unlocked once it is locked"),
		Entry("invalid config update: locked policy reduction",
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 24 * time.Hour},
					Locked:          true,
				},
			},
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 23 * time.Hour},
					Locked:          true,
				},
			}, true, "reducing the retention period from"),
		Entry("invalid config update: locked policy deletion",
			&apisazure.BackupBucketConfig{
				Immutability: &apisazure.ImmutableConfig{
					RetentionType:   apisazure.BucketLevelImmutability,
					RetentionPeriod: metav1.Duration{Duration: 24 * time.Hour},
					Locked:          true,
				},
			},
			&apisazure.BackupBucketConfig{
				Immutability: nil,
			}, true, "immutability cannot be disabled once it is locked"),
	)
})
