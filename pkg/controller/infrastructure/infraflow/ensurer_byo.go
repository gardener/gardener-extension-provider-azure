// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v10"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow/shared"
)

// EnsureUserSubnet is the BYO-mode replacement for EnsureSubnets. It performs a read-only
// discovery of the user-referenced subnet in the BYO VNet: verifies the subnet exists, reads its
// route-table association, and persists the discovered route-table name and resource group on the
// whiteboard for status emission, azure.json rendering, tag management, and the delete flow.
//
// The subnet's NetworkSecurityGroup association (if any) is user-owned and is intentionally
// ignored: the CCM-facing NSG is created by EnsureSecurityGroup in the shoot's cluster RG and is
// attached to worker NICs by MCM at machine creation time, not to the user's subnet. See the
// [NSG mutation contract] in the design proposal for the rationale.
//
// This method never issues a PUT/PATCH against the subnet.
//
// [NSG mutation contract]: ../../../docs/proposals/flexible-network-configuration-proposal.md#nsg-mutation-contract
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
		"routeTable", byo.Get(KeyBYORTName),
		"routeTableResourceGroup", byo.Get(KeyBYORTResourceGroup),
		"userSubnetNSG", subnetNSGID(subnet),
	)

	return nil
}

// subnetNSGID returns the ARM ID of the NSG attached to the subnet, or an empty string if none.
// Used for logging only.
func subnetNSGID(subnet *armnetwork.Subnet) string {
	if subnet == nil || subnet.Properties == nil || subnet.Properties.NetworkSecurityGroup == nil || subnet.Properties.NetworkSecurityGroup.ID == nil {
		return ""
	}
	return *subnet.Properties.NetworkSecurityGroup.ID
}

// EnsureBYOResourceTags applies the observability tag `kubernetes.io/cluster/<technicalName>=shared`
// to the BYO VNet and route table. Best-effort: a permission failure on any single resource is
// logged as a warning and does not fail the reconcile. The tag is merged with any pre-existing
// tags. No-ops in non-BYO mode.
//
// The Gardener-owned NSG in the shoot's cluster RG does not receive this tag — it's owned end-to-end
// by the shoot's cluster RG and dies with the cascade delete, so there's nothing to observe.
func (fctx *FlowContext) EnsureBYOResourceTags(ctx context.Context) error {
	if !helper.IsUsingUserManagedEgress(fctx.cfg) {
		return nil
	}
	log := shared.LogFromContext(ctx)
	tagKey := TagKeyClusterPrefix + fctx.adapter.TechnicalName()

	// VNet
	vnetCfg := fctx.adapter.VirtualNetworkConfig()
	if err := fctx.tagVNet(ctx, vnetCfg.ResourceGroup, vnetCfg.Name, tagKey); err != nil {
		log.Info("skipping observability tag on BYO VNet (best-effort)", "vnet", vnetCfg.Name, "resourceGroup", vnetCfg.ResourceGroup, "error", err.Error())
	} else {
		fctx.whiteboard.GetChild(ChildKeyBYO).Set(KeyBYOVNetTagged, "true")
	}

	// Route table (may be absent in overlay-CNI mode)
	if rtName := fctx.whiteboard.GetChild(ChildKeyBYO).Get(KeyBYORTName); rtName != nil {
		rtRG := *fctx.whiteboard.GetChild(ChildKeyBYO).Get(KeyBYORTResourceGroup)
		if err := fctx.tagRouteTable(ctx, rtRG, *rtName, tagKey); err != nil {
			log.Info("skipping observability tag on BYO route table (best-effort)", "routeTable", *rtName, "resourceGroup", rtRG, "error", err.Error())
		} else {
			fctx.whiteboard.GetChild(ChildKeyBYO).Set(KeyBYORTTagged, "true")
		}
	}

	return nil
}

// RemoveBYOResourceTags removes the observability tag applied by this shoot from the BYO VNet and
// route table. Best-effort: any per-resource failure is logged and does not block shoot deletion.
// Only resources marked as tagged by this shoot are touched.
func (fctx *FlowContext) RemoveBYOResourceTags(ctx context.Context) error {
	if !helper.IsUsingUserManagedEgress(fctx.cfg) {
		return nil
	}
	log := shared.LogFromContext(ctx)
	tagKey := TagKeyClusterPrefix + fctx.adapter.TechnicalName()
	byo := fctx.whiteboard.GetChild(ChildKeyBYO)

	if v := byo.Get(KeyBYOVNetTagged); v != nil && *v == "true" {
		vnetCfg := fctx.adapter.VirtualNetworkConfig()
		if err := fctx.untagVNet(ctx, vnetCfg.ResourceGroup, vnetCfg.Name, tagKey); err != nil {
			log.Info("failed to remove observability tag from BYO VNet (best-effort)", "vnet", vnetCfg.Name, "error", err.Error())
		}
	}
	if v := byo.Get(KeyBYORTTagged); v != nil && *v == "true" {
		if rtName := byo.Get(KeyBYORTName); rtName != nil {
			rtRG := *byo.Get(KeyBYORTResourceGroup)
			if err := fctx.untagRouteTable(ctx, rtRG, *rtName, tagKey); err != nil {
				log.Info("failed to remove observability tag from BYO route table (best-effort)", "routeTable", *rtName, "error", err.Error())
			}
		}
	}
	return nil
}

func (fctx *FlowContext) tagVNet(ctx context.Context, rg, name, key string) error {
	c, err := fctx.factory.Vnet()
	if err != nil {
		return err
	}
	vnet, err := c.Get(ctx, rg, name)
	if err != nil {
		return err
	}
	if vnet == nil {
		return fmt.Errorf("vnet %s/%s not found", rg, name)
	}
	if hasTag(vnet.Tags, key, TagValueShared) {
		return nil
	}
	if vnet.Tags == nil {
		vnet.Tags = map[string]*string{}
	}
	vnet.Tags[key] = to.Ptr(TagValueShared)
	_, err = c.CreateOrUpdate(ctx, rg, name, *vnet)
	return err
}

func (fctx *FlowContext) untagVNet(ctx context.Context, rg, name, key string) error {
	c, err := fctx.factory.Vnet()
	if err != nil {
		return err
	}
	vnet, err := c.Get(ctx, rg, name)
	if err != nil {
		return err
	}
	if vnet == nil || vnet.Tags == nil {
		return nil
	}
	if _, ok := vnet.Tags[key]; !ok {
		return nil
	}
	delete(vnet.Tags, key)
	_, err = c.CreateOrUpdate(ctx, rg, name, *vnet)
	return err
}

func (fctx *FlowContext) tagRouteTable(ctx context.Context, rg, name, key string) error {
	c, err := fctx.factory.RouteTables()
	if err != nil {
		return err
	}
	rt, err := c.Get(ctx, rg, name)
	if err != nil {
		return err
	}
	if rt == nil {
		return fmt.Errorf("route table %s/%s not found", rg, name)
	}
	if hasTag(rt.Tags, key, TagValueShared) {
		return nil
	}
	if rt.Tags == nil {
		rt.Tags = map[string]*string{}
	}
	rt.Tags[key] = to.Ptr(TagValueShared)
	// Tags-only patch, keeping routes intact (the CCM writes per-node pod-CIDR routes here).
	patch := armnetwork.RouteTable{
		Location: rt.Location,
		Tags:     rt.Tags,
	}
	_, err = c.CreateOrUpdate(ctx, rg, name, patch)
	return err
}

func (fctx *FlowContext) untagRouteTable(ctx context.Context, rg, name, key string) error {
	c, err := fctx.factory.RouteTables()
	if err != nil {
		return err
	}
	rt, err := c.Get(ctx, rg, name)
	if err != nil {
		return err
	}
	if rt == nil || rt.Tags == nil {
		return nil
	}
	if _, ok := rt.Tags[key]; !ok {
		return nil
	}
	delete(rt.Tags, key)
	patch := armnetwork.RouteTable{
		Location: rt.Location,
		Tags:     rt.Tags,
	}
	_, err = c.CreateOrUpdate(ctx, rg, name, patch)
	return err
}

func hasTag(tags map[string]*string, key, value string) bool {
	if tags == nil {
		return false
	}
	v, ok := tags[key]
	if !ok {
		return false
	}
	return v != nil && *v == value
}

// getUserManagedEgressInfrastructureStatus builds the InfrastructureStatus for a BYO-subnet shoot.
// The route table identifier (name + resource group) comes from the whiteboard populated by
// EnsureUserSubnet. The NSG identifier comes from the Gardener-owned NSG created by
// EnsureSecurityGroup in the shoot's cluster RG (same as managed mode). EgressCIDRs is left nil
// because Gardener has no reliable way to know the user's egress IPs in this mode.
func (fctx *FlowContext) getUserManagedEgressInfrastructureStatus() (*v1alpha1.InfrastructureStatus, error) {
	byo := fctx.whiteboard.GetChild(ChildKeyBYO)

	sgCfg := fctx.adapter.SecurityGroupConfig()

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
				Purpose: v1alpha1.PurposeNodes,
				Name:    sgCfg.Name,
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
