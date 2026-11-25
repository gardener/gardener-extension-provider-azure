// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

// MakeCluster returns a cluster object used for testing.
func MakeCluster(pods, services string, region string, countFaultDomain, countUpdateDomain int32) *controller.Cluster {
	var (
		shoot = gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Networking: &gardencorev1beta1.Networking{
					Pods:     &pods,
					Services: &services,
				},
			},
		}
		cloudProfileConfig = apiv1alpha1.CloudProfileConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
				Kind:       "CloudProfileConfig",
			},
			CountFaultDomains: []apiv1alpha1.DomainCount{
				{Region: region, Count: countFaultDomain},
			},
			CountUpdateDomains: []apiv1alpha1.DomainCount{
				{Region: region, Count: countUpdateDomain},
			},
		}
		cloudProfileConfigJSON, _ = json.Marshal(cloudProfileConfig)
		cloudProfile              = gardencorev1beta1.CloudProfile{
			Spec: gardencorev1beta1.CloudProfileSpec{
				ProviderConfig: &runtime.RawExtension{
					Raw: cloudProfileConfigJSON,
				},
			},
		}
	)

	return &controller.Cluster{
		Shoot:        &shoot,
		CloudProfile: &cloudProfile,
	}
}

// IsShootResourceGroupAvailable determines if the managed resource group exists on Azure.
func IsShootResourceGroupAvailable(ctx context.Context, factory azureclient.Factory, infra *extensionsv1alpha1.Infrastructure, infraConfig *api.InfrastructureConfig) (bool, error) {
	if infraConfig.ResourceGroup != nil {
		return true, nil
	}

	groupClient, err := factory.Group()
	if err != nil {
		return false, err
	}

	resourceGroup, err := groupClient.Get(ctx, infra.Namespace)
	if err != nil {
		return false, err
	}

	if resourceGroup == nil {
		return false, nil
	}

	return true, nil
}

// IsShootSubnetName reports whether name matches a complete subnet name generated for the shoot's technical name.
// Subnets use either <technicalName>-nodes (single-subnet and migrated layouts) or
// <technicalName>-nodes-z<zone>, where zone is a canonical positive int32.
// Match all zones, including ones removed from the desired configuration, so they can still be cleaned up.
func IsShootSubnetName(technicalName string, name *string) bool {
	if technicalName == "" || name == nil {
		return false
	}

	suffix, found := strings.CutPrefix(*name, technicalName+"-nodes")
	if !found {
		return false
	}
	if suffix == "" {
		return true
	}

	zoneName, found := strings.CutPrefix(suffix, "-z")
	if !found {
		return false
	}
	zone, err := strconv.ParseInt(zoneName, 10, 32)
	return err == nil && zone > 0 && strconv.FormatInt(zone, 10) == zoneName
}

// DeleteNodeSubnetIfExists will delete the nodes subnet(s) if exists.
func DeleteNodeSubnetIfExists(ctx context.Context, factory azureclient.Factory, infra *extensionsv1alpha1.Infrastructure, infraConfig *api.InfrastructureConfig) error {
	if infraConfig.Networks.VNet.ResourceGroup == nil || infraConfig.Networks.VNet.Name == nil {
		return nil
	}

	subnetClient, err := factory.Subnet()
	if err != nil {
		return err
	}

	subnets, err := subnetClient.List(ctx, *infraConfig.Networks.VNet.ResourceGroup, *infraConfig.Networks.VNet.Name)
	if err != nil {
		return err
	}

	for _, subnet := range subnets {
		if subnet == nil || !IsShootSubnetName(infra.Namespace, subnet.Name) {
			continue
		}

		if err := subnetClient.Delete(ctx, *infraConfig.Networks.VNet.ResourceGroup, *infraConfig.Networks.VNet.Name, *subnet.Name); err != nil {
			return err
		}
	}

	return nil
}

// DeleteShootResourceGroupIfExists will delete the shoot's resource group if it exists.
func DeleteShootResourceGroupIfExists(ctx context.Context, factory azureclient.Factory, infra *extensionsv1alpha1.Infrastructure, cfg *api.InfrastructureConfig, status *api.InfrastructureStatus) error {
	// skip if using user resource group.
	if cfg.ResourceGroup != nil {
		return nil
	}

	groupClient, err := factory.Group()
	if err != nil {
		return err
	}

	return groupClient.Delete(ctx, ShootResourceGroupName(infra, cfg, status))
}

// ShootResourceGroupName returns the expected name of the resource group.
func ShootResourceGroupName(infra *extensionsv1alpha1.Infrastructure, cfg *api.InfrastructureConfig, status *api.InfrastructureStatus) string {
	if cfg.ResourceGroup != nil {
		return cfg.ResourceGroup.Name
	}

	if status != nil && len(status.ResourceGroup.Name) > 0 {
		return status.ResourceGroup.Name
	}

	return infra.Namespace
}

// PatchProviderStatusAndState patches the infrastructure resource with the given provider status and state.
func PatchProviderStatusAndState(
	ctx context.Context,
	runtimeClient client.Client,
	infra *extensionsv1alpha1.Infrastructure,
	status *apiv1alpha1.InfrastructureStatus,
	state *runtime.RawExtension,
	egressCidrs []string,
) error {
	patch := client.MergeFrom(infra.DeepCopy())
	if status != nil {
		infra.Status.ProviderStatus = &runtime.RawExtension{Object: status}
		if egressCidrs != nil {
			infra.Status.EgressCIDRs = egressCidrs
		}
	}
	if state != nil {
		infra.Status.State = state
	}

	return runtimeClient.Status().Patch(ctx, infra, patch)
}
