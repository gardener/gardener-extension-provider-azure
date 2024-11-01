// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"time"

	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"github.com/gardener/gardener-extension-provider-azure/imagevector"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

const (
	// TerraformVarSubscriptionID is the name of the terraform subscription id environment variable.
	TerraformVarSubscriptionID = "TF_VAR_SUBSCRIPTION_ID"
	// TerraformVarTenantID is the name of the terraform tenant id environment variable.
	TerraformVarTenantID = "TF_VAR_TENANT_ID"
	// TerraformVarClientID is the name of the terraform client id environment variable.
	TerraformVarClientID = "TF_VAR_CLIENT_ID"
	// TerraformVarClientSecret is the name of the client secret environment variable.
	TerraformVarClientSecret = "TF_VAR_CLIENT_SECRET" // #nosec G101 -- No credential.
)

// TerraformerEnvVars computes the Terraformer environment variables from the given secret reference.
func TerraformerEnvVars(secretRef corev1.SecretReference, useWorkloadIdentity bool) []corev1.EnvVar {
	envVars := []corev1.EnvVar{{
		Name: TerraformVarSubscriptionID,
		ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: secretRef.Name,
			},
			Key: azure.SubscriptionIDKey,
		}},
	}, {
		Name: TerraformVarTenantID,
		ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: secretRef.Name,
			},
			Key: azure.TenantIDKey,
		}},
	}, {
		Name: TerraformVarClientID,
		ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: secretRef.Name,
			},
			Key: azure.ClientIDKey,
		}},
	}}

	if !useWorkloadIdentity {
		envVars = append(
			envVars,
			corev1.EnvVar{
				Name: TerraformVarClientSecret,
				ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretRef.Name,
					},
					Key: azure.ClientSecretKey,
				}},
			},
		)
	}

	return envVars
}

var (
	// NewTerraformer initializes a new Terraformer. Exposed for testing
	NewTerraformer = defaultNewTerraformer

	// NewTerraformerWithAuth initializes a new Terraformer that has the azure auth credentials. Exposed for testing
	NewTerraformerWithAuth = defaultNewTerraformerWithAuth
)

func defaultNewTerraformer(
	logger logr.Logger,
	restConfig *rest.Config,
	purpose string,
	infra *extensionsv1alpha1.Infrastructure,
	disableProjectedTokenMount bool,
) (
	terraformer.Terraformer,
	error,
) {
	tf, err := terraformer.NewForConfig(logger, restConfig, purpose, infra.Namespace, infra.Name, imagevector.TerraformerImage())
	if err != nil {
		return nil, err
	}

	owner := metav1.NewControllerRef(infra, extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.InfrastructureResource))
	return tf.
		UseProjectedTokenMount(!disableProjectedTokenMount).
		SetTerminationGracePeriodSeconds(630).
		SetDeadlineCleaning(5 * time.Minute).
		SetDeadlinePod(15 * time.Minute).
		SetOwnerRef(owner), nil
}

func defaultNewTerraformerWithAuth(
	logger logr.Logger,
	restConfig *rest.Config,
	purpose string,
	infra *extensionsv1alpha1.Infrastructure,
	disableProjectedTokenMount bool,
	useWorkloadIdentity bool,
) (
	terraformer.Terraformer,
	error,
) {
	tf, err := NewTerraformer(logger, restConfig, purpose, infra, disableProjectedTokenMount)
	if err != nil {
		return nil, err
	}

	return tf.SetEnvVars(TerraformerEnvVars(infra.Spec.SecretRef, useWorkloadIdentity)...), nil
}
