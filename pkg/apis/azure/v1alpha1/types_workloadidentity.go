// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkloadIdentityConfig contains configuration settings for workload identity.
type WorkloadIdentityConfig struct {
	metav1.TypeMeta

	// ClientID is the ID of the Azure client.
	ClientID string `json:"clientID,omitempty"`
	// TenantID is the ID of the Azure tenant.
	TenantID string `json:"tenantID,omitempty"`
	// SubscriptionID is the ID of the subscription.
	SubscriptionID string `json:"subscriptionID,omitempty"`
}
