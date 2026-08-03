// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

// configValidator implements infrastructure.ConfigValidator for the Azure provider. It performs
// pre-flight, cloud-side checks that require an ARM lookup and therefore cannot be performed in the
// stateless API-level validation. Today it only validates BYO-subnet / user-managed-egress
// configurations (covering acceptance criteria C8-C13); non-BYO configurations pass through as
// no-ops.
type configValidator struct {
	client         client.Client
	logger         logr.Logger
	factoryBuilder factoryBuilder
}

// factoryBuilder constructs an Azure client factory from an infrastructure secret reference.
// Parameterized so tests can inject a fake factory.
type factoryBuilder func(ctx context.Context, c client.Client, secretRef corev1.SecretReference) (azureclient.Factory, error)

// NewConfigValidator returns a new infrastructure ConfigValidator for the Azure provider.
func NewConfigValidator(mgr clientManager) infrastructure.ConfigValidator {
	return &configValidator{
		client:         mgr.GetClient(),
		logger:         logf.Log.WithName("azure-infrastructure-config-validator"),
		factoryBuilder: defaultFactoryBuilder,
	}
}

// clientManager is a minimal interface satisfied by controller-runtime's manager.Manager, kept
// small so callers can pass a mock in tests.
type clientManager interface {
	GetClient() client.Client
}

func defaultFactoryBuilder(ctx context.Context, c client.Client, secretRef corev1.SecretReference) (azureclient.Factory, error) {
	return azureclient.NewAzureClientFactoryFromSecret(ctx, c, secretRef, false)
}

// Validate implements infrastructure.ConfigValidator.
func (cv *configValidator) Validate(ctx context.Context, infra *extensionsv1alpha1.Infrastructure) field.ErrorList {
	var allErrs field.ErrorList

	infraConfig, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return append(allErrs, field.InternalError(field.NewPath("spec", "providerConfig"), err))
	}

	// Skip cloud-side validation for managed-mode shoots; they have no BYO surface to verify.
	if !helper.IsUsingUserManagedEgress(infraConfig) {
		return allErrs
	}

	cluster, err := extensionscontroller.GetCluster(ctx, cv.client, infra.Namespace)
	if err != nil {
		return append(allErrs, field.InternalError(field.NewPath(""), fmt.Errorf("failed to get cluster for infrastructure %s: %w", infra.Name, err)))
	}

	auth, _, err := azureclient.GetClientAuthData(ctx, cv.client, infra.Spec.SecretRef, false)
	if err != nil {
		return append(allErrs, field.InternalError(field.NewPath("spec", "secretRef"), fmt.Errorf("failed to read cloud credentials: %w", err)))
	}

	factory, err := cv.factoryBuilder(ctx, cv.client, infra.Spec.SecretRef)
	if err != nil {
		return append(allErrs, field.InternalError(field.NewPath("spec", "secretRef"), fmt.Errorf("failed to build Azure client factory: %w", err)))
	}

	subnetsClient, err := factory.Subnet()
	if err != nil {
		return append(allErrs, field.InternalError(field.NewPath(""), fmt.Errorf("failed to build Azure subnets client: %w", err)))
	}

	basePath := field.NewPath("spec", "providerConfig", "networks")
	return append(allErrs, cv.validateUserManagedEgress(ctx, infraConfig, cluster, auth.SubscriptionID, subnetsClient, basePath)...)
}

// validateUserManagedEgress performs the runtime checks required for BYO-subnet mode. It reads the
// referenced subnet from ARM, verifies the required associations exist, and checks CIDR / cross-
// subscription constraints. Covers C8-C13.
func (cv *configValidator) validateUserManagedEgress(
	ctx context.Context,
	infraConfig *apisazure.InfrastructureConfig,
	cluster *extensionscontroller.Cluster,
	shootSubscriptionID string,
	subnetsClient azureclient.Subnet,
	basePath *field.Path,
) field.ErrorList {
	var (
		allErrs    field.ErrorList
		subnetRef  = infraConfig.Networks.Subnet
		vnetName   = *infraConfig.Networks.VNet.Name
		vnetRG     = *infraConfig.Networks.VNet.ResourceGroup
		subnetPath = basePath.Child("subnet", "name")
	)

	// C8: subnet must exist.
	subnet, err := subnetsClient.Get(ctx, vnetRG, vnetName, subnetRef.Name, nil)
	if err != nil {
		return append(allErrs, field.InternalError(subnetPath, fmt.Errorf("failed to look up subnet %q in vnet %s/%s: %w", subnetRef.Name, vnetRG, vnetName, err)))
	}
	if subnet == nil {
		return append(allErrs, field.NotFound(subnetPath, fmt.Sprintf("subnet %q was not found in vnet %s/%s", subnetRef.Name, vnetRG, vnetName)))
	}

	if subnet.Properties == nil {
		return append(allErrs, field.Invalid(subnetPath, subnetRef.Name, fmt.Sprintf("subnet %q in vnet %s/%s has no properties", subnetRef.Name, vnetRG, vnetName)))
	}

	// C9: subnet must have an NSG association.
	nsgID := ""
	if subnet.Properties.NetworkSecurityGroup != nil && subnet.Properties.NetworkSecurityGroup.ID != nil {
		nsgID = *subnet.Properties.NetworkSecurityGroup.ID
	}
	if nsgID == "" {
		allErrs = append(allErrs, field.Invalid(subnetPath, subnetRef.Name, fmt.Sprintf("subnet %q must have a network security group attached before it can be used as a BYO worker subnet", subnetRef.Name)))
	}

	// C10: subnet must have an RT association (unless the shoot uses an overlay CNI).
	rtID := ""
	if subnet.Properties.RouteTable != nil && subnet.Properties.RouteTable.ID != nil {
		rtID = *subnet.Properties.RouteTable.ID
	}
	overlayEnabled := false
	if cluster != nil && cluster.Shoot != nil {
		var overlayErr error
		overlayEnabled, overlayErr = helper.IsOverlayEnabled(cluster.Shoot.Spec.Networking)
		if overlayErr != nil {
			return append(allErrs, field.Invalid(field.NewPath("spec", "networking", "providerConfig"), "", fmt.Sprintf("failed to determine overlay networking mode: %v", overlayErr)))
		}
	}
	if rtID == "" && !overlayEnabled {
		allErrs = append(allErrs, field.Invalid(subnetPath, subnetRef.Name, fmt.Sprintf("subnet %q must have a route table attached before it can be used as a BYO worker subnet; alternatively enable an overlay CNI on the shoot's networking (Cilium/Calico with VXLAN or Geneve) so the seed CCM's route controller is not needed", subnetRef.Name)))
	}

	// C13: NSG and RT (if present) must live in the same subscription as the shoot.
	if nsgID != "" {
		if rid, perr := arm.ParseResourceID(nsgID); perr != nil {
			allErrs = append(allErrs, field.Invalid(subnetPath, nsgID, fmt.Sprintf("failed to parse NSG resource ID %q: %v", nsgID, perr)))
		} else if rid.SubscriptionID != shootSubscriptionID {
			allErrs = append(allErrs, field.Invalid(subnetPath, nsgID, fmt.Sprintf("NSG %q is in subscription %q but the shoot uses subscription %q; cross-subscription NSG references are not supported", nsgID, rid.SubscriptionID, shootSubscriptionID)))
		}
	}
	if rtID != "" {
		if rid, perr := arm.ParseResourceID(rtID); perr != nil {
			allErrs = append(allErrs, field.Invalid(subnetPath, rtID, fmt.Sprintf("failed to parse route-table resource ID %q: %v", rtID, perr)))
		} else if rid.SubscriptionID != shootSubscriptionID {
			allErrs = append(allErrs, field.Invalid(subnetPath, rtID, fmt.Sprintf("route table %q is in subscription %q but the shoot uses subscription %q; cross-subscription route-table references are not supported", rtID, rid.SubscriptionID, shootSubscriptionID)))
		}
	}

	// C11 + C12: CIDR containment / non-overlap with pods / services.
	subnetCIDR := ""
	if subnet.Properties.AddressPrefix != nil {
		subnetCIDR = *subnet.Properties.AddressPrefix
	}
	if subnetCIDR == "" && len(subnet.Properties.AddressPrefixes) > 0 && subnet.Properties.AddressPrefixes[0] != nil {
		subnetCIDR = *subnet.Properties.AddressPrefixes[0]
	}
	if subnetCIDR == "" {
		return append(allErrs, field.Invalid(subnetPath, subnetRef.Name, fmt.Sprintf("subnet %q has no address prefix", subnetRef.Name)))
	}

	if cluster != nil && cluster.Shoot != nil && cluster.Shoot.Spec.Networking != nil {
		net := cluster.Shoot.Spec.Networking
		subnetCIDRVal := cidrvalidation.NewCIDR(subnetCIDR, subnetPath)
		allErrs = append(allErrs, subnetCIDRVal.ValidateParse()...)

		if net.Nodes != nil {
			nodesCIDR := cidrvalidation.NewCIDR(*net.Nodes, field.NewPath("networking", "nodes"))
			// C11: subnet CIDR must be a subset of shoot.spec.networking.nodes
			allErrs = append(allErrs, nodesCIDR.ValidateSubset(subnetCIDRVal)...)
		}
		// C12: subnet CIDR must not overlap pods / services
		var others []cidrvalidation.CIDR
		if net.Pods != nil {
			others = append(others, cidrvalidation.NewCIDR(*net.Pods, field.NewPath("networking", "pods")))
		}
		if net.Services != nil {
			others = append(others, cidrvalidation.NewCIDR(*net.Services, field.NewPath("networking", "services")))
		}
		if len(others) > 0 {
			allErrs = append(allErrs, subnetCIDRVal.ValidateNotOverlap(others...)...)
		}
	}

	return allErrs
}
