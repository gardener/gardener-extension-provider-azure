// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"strings"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func purgeSeedRemedyControllerResources(ctx context.Context, c client.Client, log logr.Logger) error {
	log.Info("Starting the deletion of obsolete remedy controller resources")

	var remedyControllerDeployments appsv1.DeploymentList
	if err := c.List(ctx, &remedyControllerDeployments, client.MatchingLabels{
		"app": "remedy-controller-azure",
	}); err != nil {
		return fmt.Errorf("failed to list remedy controller deployments: %w", err)
	}
	for _, deployment := range remedyControllerDeployments.Items {
		log.Info("Deleting deployment", "name", client.ObjectKeyFromObject(&deployment))
		if err := kutil.DeleteObject(
			ctx,
			c,
			&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: deployment.Namespace, Name: deployment.Name}},
		); err != nil {
			return fmt.Errorf("failed to delete deployment %s: %w", client.ObjectKeyFromObject(&deployment), err)
		}
	}

	log.Info("Successfully removed remedy controller deployments")
	return nil
}

// TODO (kon-angelo): Remove after the release of version 1.46.0
func purgeTerraformerRBACResources(ctx context.Context, c client.Client, log logr.Logger) error {
	log.Info("Starting the deletion of obsolete terraformer resources")

	const (
		terraformerRoleName = "gardener.cloud:system:terraformer"
	)

	var (
		roleBindingList    = &rbacv1.RoleBindingList{}
		roleList           = &rbacv1.RoleList{}
		serviceAccountList = &corev1.ServiceAccountList{}
	)

	// list serviceAccount bindings in all namespaces
	if err := c.List(ctx, roleBindingList); err != nil {
		return fmt.Errorf("failed to list RoleBindings: %w", err)
	}

	for _, roleBinding := range roleBindingList.Items {
		if strings.EqualFold(roleBinding.Name, terraformerRoleName) {
			log.Info("Deleting RoleBinding", "roleBinding", client.ObjectKeyFromObject(&roleBinding))
			if err := kutil.DeleteObject(
				ctx,
				c,
				&rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Namespace: roleBinding.Namespace, Name: roleBinding.Name}},
			); err != nil {
				return fmt.Errorf("failed to delete roleBinding %s: %w", client.ObjectKeyFromObject(&roleBinding), err)
			}
		}
	}
	log.Info("Successfully deleted the obsolete RoleBindings for terraformer")

	if err := c.List(ctx, roleList); err != nil {
		return fmt.Errorf("failed to list roles: %w", err)
	}

	for _, role := range roleList.Items {
		if strings.EqualFold(role.Name, terraformerRoleName) {
			log.Info("Deleting Role", "role", client.ObjectKeyFromObject(&role))
			if err := kutil.DeleteObject(
				ctx,
				c,
				&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Namespace: role.Namespace, Name: role.Name}},
			); err != nil {
				return fmt.Errorf("failed to delete Role %s: %w", client.ObjectKeyFromObject(&role), err)
			}
		}
	}
	log.Info("Successfully deleted the obsolete Roles for terraformer")

	if err := c.List(ctx, serviceAccountList); err != nil {
		return fmt.Errorf("failed to list roles: %w", err)
	}

	for _, serviceAccount := range serviceAccountList.Items {
		if strings.EqualFold(serviceAccount.Name, "terraformer") {
			log.Info("Deleting ServiceAccount", "serviceAccount", client.ObjectKeyFromObject(&serviceAccount))
			if err := kutil.DeleteObject(
				ctx,
				c,
				&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: serviceAccount.Namespace, Name: serviceAccount.Name}},
			); err != nil {
				return fmt.Errorf("failed to delete ServiceAccount %s: %w", client.ObjectKeyFromObject(&serviceAccount), err)
			}
		}
	}
	log.Info("Successfully deleted the obsolete ServiceAccounts for terraformer")

	return nil
}
