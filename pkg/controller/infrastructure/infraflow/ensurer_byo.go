// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow/shared"
)

// EnsureUserSubnet is the BYO-mode replacement for EnsureSubnets. It performs a read-only
// discovery of the user-referenced subnet in the BYO VNet: verifies the subnet exists, reads its
// route-table and network-security-group associations, and persists the discovered names and
// resource groups on the whiteboard for status emission, azure.json rendering, and the delete
// flow. Never issues a PUT/PATCH against the subnet, NSG, or route table.
func (fctx *FlowContext) EnsureUserSubnet(ctx context.Context) error {
	if !helper.IsUsingUserManagedEgress(fctx.cfg) {
		return nil
	}

	log := shared.LogFromContext(ctx)
	vnetCfg := fctx.adapter.VirtualNetworkConfig()
	subnetRef := fctx.cfg.Networks.Subnet

	c, err := fctx.factory.Subnet()
	if err != nil {
		return err
	}

	subnet, err := c.Get(ctx, vnetCfg.ResourceGroup, vnetCfg.Name, subnetRef.Name, nil)
	if err != nil {
		return fmt.Errorf("failed to look up BYO subnet %q in vnet %s/%s: %w", subnetRef.Name, vnetCfg.ResourceGroup, vnetCfg.Name, err)
	}
	if subnet == nil {
		return fmt.Errorf("BYO subnet %q was not found in vnet %s/%s", subnetRef.Name, vnetCfg.ResourceGroup, vnetCfg.Name)
	}
	if subnet.Properties == nil {
		return fmt.Errorf("BYO subnet %q has no properties", subnetRef.Name)
	}

	byo := fctx.whiteboard.GetChild(ChildKeyBYO)

	// NSG discovery is mandatory; the CCM requires a non-empty securityGroupName in azure.json.
	// Because the subnet is unique per shoot (see the design proposal's route-table ownership
	// section), the NSG attached to it is 1:1 with this shoot and can be safely handed to the
	// CCM without the shared-NSG destruction hazard.
	if subnet.Properties.NetworkSecurityGroup == nil || subnet.Properties.NetworkSecurityGroup.ID == nil {
		return fmt.Errorf("BYO subnet %q has no network security group attached; attach an NSG before reconciling the shoot", subnetRef.Name)
	}
	nsgID := *subnet.Properties.NetworkSecurityGroup.ID
	nsgResID, err := arm.ParseResourceID(nsgID)
	if err != nil {
		return fmt.Errorf("failed to parse discovered NSG resource ID %q: %w", nsgID, err)
	}
	byo.Set(KeyBYONSGID, nsgID)
	byo.Set(KeyBYONSGName, nsgResID.Name)
	byo.Set(KeyBYONSGResourceGroup, nsgResID.ResourceGroupName)

	// Route table discovery is optional when the shoot uses an overlay CNI: pod-to-pod traffic is
	// encapsulated at the node level and no per-node routes are written into the underlying VNet.
	// Signal comes from the shoot's networking provider config, not from the InfrastructureConfig.
	overlayEnabled, err := helper.IsOverlayEnabled(fctx.cluster.Shoot.Spec.Networking)
	if err != nil {
		return fmt.Errorf("failed to determine overlay networking mode: %w", err)
	}
	if subnet.Properties.RouteTable != nil && subnet.Properties.RouteTable.ID != nil {
		rtID := *subnet.Properties.RouteTable.ID
		rtResID, err := arm.ParseResourceID(rtID)
		if err != nil {
			return fmt.Errorf("failed to parse discovered route-table resource ID %q: %w", rtID, err)
		}
		byo.Set(KeyBYORTID, rtID)
		byo.Set(KeyBYORTName, rtResID.Name)
		byo.Set(KeyBYORTResourceGroup, rtResID.ResourceGroupName)
	} else if !overlayEnabled {
		return fmt.Errorf("BYO subnet %q has no route table attached; either attach one or enable an overlay CNI on the shoot's networking (Cilium/Calico with VXLAN or Geneve)", subnetRef.Name)
	}

	if subnet.Properties.AddressPrefix != nil {
		byo.Set(KeyBYOSubnetCIDR, *subnet.Properties.AddressPrefix)
	} else if len(subnet.Properties.AddressPrefixes) > 0 && subnet.Properties.AddressPrefixes[0] != nil {
		byo.Set(KeyBYOSubnetCIDR, *subnet.Properties.AddressPrefixes[0])
	}

	log.Info("discovered BYO subnet associations",
		"subnet", subnetRef.Name,
		"nsg", byo.Get(KeyBYONSGName),
		"nsgResourceGroup", byo.Get(KeyBYONSGResourceGroup),
		"routeTable", byo.Get(KeyBYORTName),
		"routeTableResourceGroup", byo.Get(KeyBYORTResourceGroup),
	)

	return nil
}

// getUserManagedEgressInfrastructureStatus builds the InfrastructureStatus for a BYO-subnet shoot.
// The discovered NSG and route table (name + resource group) come from the whiteboard populated by
// EnsureUserSubnet. EgressCIDRs is left nil because Gardener has no reliable way to know the user's
// egress IPs in this mode.
func (fctx *FlowContext) getUserManagedEgressInfrastructureStatus() (*v1alpha1.InfrastructureStatus, error) {
	byo := fctx.whiteboard.GetChild(ChildKeyBYO)

	nsgName := byo.Get(KeyBYONSGName)
	nsgRG := byo.Get(KeyBYONSGResourceGroup)
	if nsgName == nil || nsgRG == nil {
		return nil, fmt.Errorf("BYO NSG has not been discovered yet; EnsureUserSubnet must run before status is built")
	}

	status := &v1alpha1.InfrastructureStatus{
		TypeMeta: metav1.TypeMeta{
			Kind:       "InfrastructureStatus",
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
		},
		Networks: v1alpha1.NetworkStatus{
			VNet: v1alpha1.VNetStatus{
				Name:          fctx.adapter.VirtualNetworkConfig().Name,
				ResourceGroup: to.Ptr(fctx.adapter.VirtualNetworkConfig().ResourceGroup),
			},
			Layout:             v1alpha1.NetworkLayoutSingleSubnet,
			OutboundAccessType: v1alpha1.OutboundAccessTypeUserManaged,
			Subnets: []v1alpha1.Subnet{
				{
					Purpose: v1alpha1.PurposeNodes,
					Name:    fctx.cfg.Networks.Subnet.Name,
				},
			},
		},
		ResourceGroup: v1alpha1.ResourceGroup{
			Name: fctx.adapter.ResourceGroupName(),
		},
		SecurityGroups: []v1alpha1.SecurityGroup{
			{
				Purpose:       v1alpha1.PurposeNodes,
				Name:          *nsgName,
				ResourceGroup: to.Ptr(*nsgRG),
			},
		},
		Zoned: fctx.cfg.Zoned,
	}

	if rtName := byo.Get(KeyBYORTName); rtName != nil {
		rtRG := byo.Get(KeyBYORTResourceGroup)
		status.RouteTables = []v1alpha1.RouteTable{
			{
				Purpose:       v1alpha1.PurposeNodes,
				Name:          *rtName,
				ResourceGroup: to.Ptr(*rtRG),
			},
		}
	}

	if identity := fctx.cfg.Identity; identity != nil {
		status.Identity = &v1alpha1.IdentityStatus{
			ID:        *fctx.whiteboard.Get(KeyManagedIdentityId),
			ClientID:  *fctx.whiteboard.Get(KeyManagedIdentityClientId),
			ACRAccess: identity.ACRAccess != nil && *identity.ACRAccess,
		}
	}

	return status, nil
}
