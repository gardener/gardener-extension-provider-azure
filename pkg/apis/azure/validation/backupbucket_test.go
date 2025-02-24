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
