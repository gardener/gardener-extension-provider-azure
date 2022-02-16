// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Terraform", func() {
	Describe("#TerraformerEnvVars", func() {
		It("should correctly create the environment variables", func() {
			secretRef := corev1.SecretReference{Name: "cloud"}
			Expect(TerraformerEnvVars(secretRef)).To(ConsistOf(
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
				}))
		})
	})
})
