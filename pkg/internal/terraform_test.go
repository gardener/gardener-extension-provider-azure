// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Terraform", func() {
	Describe("#TerraformerEnvVars", func() {
		It("should correctly create the environment variables when workload identity is disabled", func() {
			secretRef := corev1.SecretReference{Name: "cloud"}
			Expect(TerraformerEnvVars(secretRef, false)).To(ConsistOf(
				corev1.EnvVar{
					Name: "TF_VAR_SUBSCRIPTION_ID",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretRef.Name,
						},
						Key: "subscriptionID",
					}},
				},
				corev1.EnvVar{
					Name: "TF_VAR_TENANT_ID",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretRef.Name,
						},
						Key: "tenantID",
					}},
				},
				corev1.EnvVar{
					Name: "TF_VAR_CLIENT_ID",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretRef.Name,
						},
						Key: "clientID",
					}},
				},
				corev1.EnvVar{
					Name: "TF_VAR_CLIENT_SECRET",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretRef.Name,
						},
						Key: "clientSecret",
					}},
				},
			))
		})

		It("should correctly create the environment variables when workload identity is enabled", func() {
			secretRef := corev1.SecretReference{Name: "cloud"}
			Expect(TerraformerEnvVars(secretRef, true)).To(ConsistOf(
				corev1.EnvVar{
					Name: "TF_VAR_SUBSCRIPTION_ID",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretRef.Name,
						},
						Key: "subscriptionID",
					}},
				},
				corev1.EnvVar{
					Name: "TF_VAR_TENANT_ID",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretRef.Name,
						},
						Key: "tenantID",
					}},
				},
				corev1.EnvVar{
					Name: "TF_VAR_CLIENT_ID",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretRef.Name,
						},
						Key: "clientID",
					}},
				},
			))
		})
	})
})
