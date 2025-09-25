// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	consts "github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

// InfrastructureAdapter contains information about the infrastructure resources that are either static, or otherwise
// inferable based on the shoot configuration. It acts as an intermediate step to make the configuration easier to process
// for the ensurer step.
type InfrastructureAdapter struct {
	infra   *extensionsv1alpha1.Infrastructure
	config  *azure.InfrastructureConfig
	status  *azure.InfrastructureStatus
	profile *azure.CloudProfileConfig
	cluster *extensionscontroller.Cluster

	// cached configuration
	vnetConfig  VirtualNetworkConfig
	zoneConfigs []ZoneConfig
}

// NewInfrastructureAdapter returns a new instance of the InfrastructureAdapter.
func NewInfrastructureAdapter(
	infra *extensionsv1alpha1.Infrastructure,
	config *azure.InfrastructureConfig,
	status *azure.InfrastructureStatus,
	profile *azure.CloudProfileConfig,
	cluster *extensionscontroller.Cluster,
) (*InfrastructureAdapter, error) {
	ia := &InfrastructureAdapter{
		infra:   infra.DeepCopy(),
		config:  config.DeepCopy(),
		profile: profile.DeepCopy(),
		cluster: cluster,
		status:  status,
	}
	ia.vnetConfig = ia.virtualNetworkConfig()
	ia.zoneConfigs = ia.zonesConfig()
	return ia, nil
}

// TechnicalName the cluster's "base" name. Used as a name or as a prefix by other resources.
func (ia *InfrastructureAdapter) TechnicalName() string {
	return infrastructure.ShootResourceGroupName(ia.infra, ia.config, ia.status)
}

// ResourceGroupConfig contains the configuration for a resource group.
type ResourceGroupConfig struct {
	AzureResourceMetadata
	Location string
}

// ShootInfo contains information about the shoot the resource belongs.
type ShootInfo struct {
	ShootName string
}

// ResourceGroup returns the configuration for the shoot's resource group.
func (ia *InfrastructureAdapter) ResourceGroup() ResourceGroupConfig {
	return ResourceGroupConfig{
		AzureResourceMetadata: AzureResourceMetadata{
			Name: ia.ResourceGroupName(),
			Kind: KindResourceGroup,
		},
		Location: ia.Region(),
	}
}

// ResourceGroupName returns the shoot's resource group's name.
func (ia *InfrastructureAdapter) ResourceGroupName() string {
	return ia.TechnicalName()
}

// VirtualNetworkConfig contains configuration for the virtual network
type VirtualNetworkConfig struct {
	AzureResourceMetadata
	// Managed is true if the vnet is managed by gardener.
	Managed bool
	// Location is a reference to the region.
	Location string
	// Cidr is the vnet's CIDR.
	CIDR *string
	// DDoSPlanID is the ID reference of the DDoS protection plan.
	DDoSPlanID *string
}

// Region is the region of the shoot.
func (ia *InfrastructureAdapter) Region() string {
	return ia.infra.Spec.Region
}

// VirtualNetworkConfig returns the virtual network configuration.
func (ia *InfrastructureAdapter) VirtualNetworkConfig() VirtualNetworkConfig {
	return ia.vnetConfig
}

func (ia *InfrastructureAdapter) virtualNetworkConfig() VirtualNetworkConfig {
	name := ia.TechnicalName()
	rg := ia.ResourceGroupName()
	managed := ia.isGardenerManagedVirtualNetwork()
	if !managed {
		name = *ia.config.Networks.VNet.Name
		rg = *ia.config.Networks.VNet.ResourceGroup
	}
	vnc := VirtualNetworkConfig{
		AzureResourceMetadata: AzureResourceMetadata{
			Name:          name,
			ResourceGroup: rg,
			Kind:          KindVirtualNetwork,
		},
		Managed:    managed,
		Location:   ia.Region(),
		DDoSPlanID: ia.config.Networks.VNet.DDosProtectionPlanID,
	}

	if cidr := ia.config.Networks.VNet.CIDR; cidr != nil {
		// copy string
		vnc.CIDR = to.Ptr(*cidr)
	} else if cidr = ia.config.Networks.Workers; cidr != nil {
		vnc.CIDR = to.Ptr(*cidr)
	}

	return vnc
}

// isGardenerManagedVirtualNetwork returns true if gardener manages the shoot's virtual network.
func (ia *InfrastructureAdapter) isGardenerManagedVirtualNetwork() bool {
	return ia.config.Networks.VNet.ResourceGroup == nil
}

// RouteTableConfig is the desired configuration for a route table.
type RouteTableConfig struct {
	AzureResourceMetadata
	Location string
}

// RouteTableConfig returns configuration for the shoot's route table.
func (ia *InfrastructureAdapter) RouteTableConfig() RouteTableConfig {
	return RouteTableConfig{
		AzureResourceMetadata: AzureResourceMetadata{
			ResourceGroup: ia.ResourceGroupName(),
			Name:          "worker_route_table",
			Kind:          KindRouteTable,
		},
		Location: ia.Region(),
	}
}

// SecurityGroupConfig is the desired configuration for a security group.
type SecurityGroupConfig struct {
	AzureResourceMetadata
	Location string
}

// SecurityGroupConfig returns the configuration for our desired security group.
func (ia *InfrastructureAdapter) SecurityGroupConfig() SecurityGroupConfig {
	return SecurityGroupConfig{
		AzureResourceMetadata: AzureResourceMetadata{
			ResourceGroup: ia.ResourceGroupName(),
			Name:          fmt.Sprintf("%s-workers", ia.TechnicalName()),
			Kind:          KindSecurityGroup,
		},
		Location: ia.Region(),
	}
}

// PublicIPConfig contains configuration for a public IP resource.
type PublicIPConfig struct {
	AzureResourceMetadata
	ShootInfo
	Zones    []string
	Location string
	Managed  bool
}

// NatGatewayConfig contains configuration for a NAT Gateway.
type NatGatewayConfig struct {
	AzureResourceMetadata
	Location     string
	Zone         *string
	IdleTimeout  *int32
	PublicIPList []PublicIPConfig
}

// SubnetConfig is the specification for a subnet
type SubnetConfig struct {
	AzureResourceMetadata
	cidr                  string
	serviceEndpoint       []string
	zone                  *string
	defaultOutboundAccess bool
}

// ZoneConfig is the specification for a zone.
type ZoneConfig struct {
	Subnet     SubnetConfig
	NatGateway *NatGatewayConfig
	Migrated   bool
}

func (ia *InfrastructureAdapter) natGatewayName() string {
	return fmt.Sprintf("%s-nat-gateway", ia.TechnicalName())
}

func (ia *InfrastructureAdapter) natGatewayNameForZone(zone int32, migrated bool) string {
	if migrated {
		return ia.natGatewayName()
	}

	return fmt.Sprintf("%s-z%d", ia.natGatewayName(), zone)
}

func (ia *InfrastructureAdapter) shootSubnetNamePrefix() string {
	return fmt.Sprintf("%s-nodes", ia.TechnicalName())
}

func (ia *InfrastructureAdapter) subnetName(zone *int32, migrated bool) string {
	n := ia.shootSubnetNamePrefix()
	if zone != nil && !migrated {
		n = fmt.Sprintf("%s-z%d", n, *zone)
	}
	return n
}

// IsOwnSubnetName returns a bool indicating whether the subnet with the given name was created by the
// reconciliation of the current shoot.
//
// This is needed to distinguish between subnets by unfortunately named shoots (i.e. the current shoot's name
// is a prefix to another's) that deploy in the same vnet.
func (ia *InfrastructureAdapter) IsOwnSubnetName(name *string) bool {
	if name == nil {
		return false
	}
	expectedPrefix := ia.shootSubnetNamePrefix()
	if _, found := strings.CutPrefix(*name, expectedPrefix); found {
		return true
		// No need to check further. The important thing to check is that there is nothing
		// between the technical name and the next expected part.
	}
	return false
}

func (ia *InfrastructureAdapter) publicIPName(natName string) string {
	return fmt.Sprintf("%s-ip", natName)
}

// Zones returns the target specification for the zones that need to be reconciled.
func (ia *InfrastructureAdapter) Zones() []ZoneConfig {
	return ia.zoneConfigs
}

func (ia *InfrastructureAdapter) hasDisableDefaultOutBoundAccessAnnotation() bool {
	ok, _ := strconv.ParseBool(ia.cluster.Shoot.Annotations[consts.DisableDefaultOutboundAccessAnnotation])
	return ok
}

func (ia *InfrastructureAdapter) zonesConfig() []ZoneConfig {
	if len(ia.config.Networks.Zones) == 0 {
		return ia.defaultZone()
	}

	var zones []ZoneConfig
	migratedZone, ok := ia.infra.Annotations[consts.NetworkLayoutZoneMigrationAnnotation]
	for _, configZone := range ia.config.Networks.Zones {
		zoneString := helper.InfrastructureZoneToString(configZone.Name)
		isMigratedZone := ok && migratedZone == zoneString
		z := ZoneConfig{
			Subnet: SubnetConfig{
				AzureResourceMetadata: AzureResourceMetadata{
					ResourceGroup: ia.vnetConfig.ResourceGroup,
					Name:          ia.subnetName(&configZone.Name, isMigratedZone),
					Parent:        ia.vnetConfig.Name,
					Kind:          KindSubnet,
				},
				cidr:                  configZone.CIDR,
				serviceEndpoint:       configZone.ServiceEndpoints,
				zone:                  &zoneString,
				defaultOutboundAccess: !ia.hasDisableDefaultOutBoundAccessAnnotation(),
			},
			Migrated: isMigratedZone,
		}

		if configZone.NatGateway != nil && configZone.NatGateway.Enabled {
			ngw := &NatGatewayConfig{
				AzureResourceMetadata: AzureResourceMetadata{
					ResourceGroup: ia.ResourceGroupName(),
					Name:          ia.natGatewayNameForZone(configZone.Name, isMigratedZone),
					Kind:          KindNatGateway,
				},
				IdleTimeout: configZone.NatGateway.IdleConnectionTimeoutMinutes,
				Location:    ia.Region(),
				Zone:        to.Ptr(zoneString),
			}
			z.NatGateway = ngw

			if len(configZone.NatGateway.IPAddresses) > 0 {
				for _, ipRef := range configZone.NatGateway.IPAddresses {
					ip := PublicIPConfig{
						ShootInfo: ShootInfo{
							ShootName: ia.TechnicalName(),
						},
						AzureResourceMetadata: AzureResourceMetadata{
							ResourceGroup: ipRef.ResourceGroup,
							Name:          ipRef.Name,
							Kind:          KindPublicIP,
						},
						Zones:   []string{zoneString},
						Managed: false,
					}
					ngw.PublicIPList = append(ngw.PublicIPList, ip)
				}
			} else {
				ip := PublicIPConfig{
					ShootInfo: ShootInfo{
						ShootName: ia.TechnicalName(),
					},
					AzureResourceMetadata: AzureResourceMetadata{
						ResourceGroup: ia.ResourceGroupName(),
						Name:          ia.publicIPName(ngw.Name),
						Kind:          KindPublicIP,
					},
					Managed:  true,
					Zones:    []string{zoneString},
					Location: ia.Region(),
				}
				ngw.PublicIPList = append(ngw.PublicIPList, ip)
			}
		}
		zones = append(zones, z)
	}

	return zones
}

func (ia *InfrastructureAdapter) defaultZone() []ZoneConfig {
	config := ia.config
	z := ZoneConfig{
		Subnet: SubnetConfig{
			AzureResourceMetadata: AzureResourceMetadata{
				ResourceGroup: ia.vnetConfig.ResourceGroup,
				Name:          ia.subnetName(nil, false),
				Parent:        ia.vnetConfig.Name,
				Kind:          KindSubnet,
			},
			cidr:                  *config.Networks.Workers,
			serviceEndpoint:       config.Networks.ServiceEndpoints,
			defaultOutboundAccess: !ia.hasDisableDefaultOutBoundAccessAnnotation(),
		},
		Migrated: false,
	}
	if config.Networks.NatGateway == nil || !config.Networks.NatGateway.Enabled {
		return []ZoneConfig{z}
	}

	ngw := &NatGatewayConfig{
		AzureResourceMetadata: AzureResourceMetadata{
			ResourceGroup: ia.ResourceGroupName(),
			Name:          ia.natGatewayName(),
			Kind:          KindNatGateway,
		},
		IdleTimeout: config.Networks.NatGateway.IdleConnectionTimeoutMinutes,
		Location:    ia.Region(),
	}
	if z := config.Networks.NatGateway.Zone; z != nil {
		ngw.Zone = to.Ptr(strconv.Itoa(int(*z)))
	}

	if len(config.Networks.NatGateway.IPAddresses) > 0 {
		for _, ipRef := range config.Networks.NatGateway.IPAddresses {
			ip := PublicIPConfig{
				ShootInfo: ShootInfo{
					ShootName: ia.TechnicalName(),
				},
				AzureResourceMetadata: AzureResourceMetadata{
					ResourceGroup: ipRef.ResourceGroup,
					Name:          ipRef.Name,
					Kind:          KindPublicIP,
				},
				Managed: false,
			}
			ip.Zones = append(ip.Zones, strconv.Itoa(int(ipRef.Zone)))
			ngw.PublicIPList = append(ngw.PublicIPList, ip)
		}
	} else {
		ip := PublicIPConfig{
			ShootInfo: ShootInfo{
				ShootName: ia.TechnicalName(),
			},
			AzureResourceMetadata: AzureResourceMetadata{
				ResourceGroup: ia.ResourceGroupName(),
				Name:          ia.publicIPName(ngw.Name),
				Kind:          KindPublicIP,
			},
			Managed:  true,
			Location: ia.Region(),
		}
		if ngw.Zone != nil {
			ip.Zones = append(ip.Zones, *ngw.Zone)
		}
		ngw.PublicIPList = append(ngw.PublicIPList, ip)
	}
	z.NatGateway = ngw

	return []ZoneConfig{z}
}

// ManagedIpConfigs returns a filtered list of only the public IPs that are managed by gardener.
func (ia *InfrastructureAdapter) ManagedIpConfigs() map[string]PublicIPConfig {
	res := make(map[string]PublicIPConfig)
	for _, z := range ia.zoneConfigs {
		if z.NatGateway == nil {
			continue
		}

		for _, ip := range z.NatGateway.PublicIPList {
			// we can return a map with the name as key because we know that the names are unique within the resource group.
			if ip.Managed {
				res[ip.Name] = ip
			}
		}
	}

	return res
}

// IpConfigs is the configuration for the desired public IPs.
func (ia *InfrastructureAdapter) IpConfigs() []PublicIPConfig {
	var res []PublicIPConfig
	for _, z := range ia.zoneConfigs {
		if z.NatGateway == nil {
			continue
		}
		res = append(res, z.NatGateway.PublicIPList...)
	}

	return res
}

// IsPublicIPPinned checks whether a public IP found in the shoot's resource group is also pinned via the provider spec
// as a NATGateway IP.
func (ia *InfrastructureAdapter) IsPublicIPPinned(name string) bool {
	for _, z := range ia.zoneConfigs {
		if z.NatGateway == nil {
			continue
		}
		for _, ip := range z.NatGateway.PublicIPList {
			if ip.Managed {
				continue
			}
			if ip.ResourceGroup == ia.ResourceGroupName() && ip.Name == name {
				return true
			}
		}
	}

	return false
}

// NatGatewayConfigs is the configuration for the desired NAT Gateways.
func (ia *InfrastructureAdapter) NatGatewayConfigs() map[string]NatGatewayConfig {
	res := make(map[string]NatGatewayConfig)
	for _, z := range ia.Zones() {
		if z.NatGateway != nil {
			res[z.NatGateway.Name] = *z.NatGateway
		}
	}

	return res
}

// HasShootPrefix returns true if the target resource's name is prefixed with the shoot's canonical name.
func (ia *InfrastructureAdapter) HasShootPrefix(name *string) bool {
	if name == nil {
		return false
	}
	return strings.HasPrefix(*name, ia.TechnicalName())
}

// ToProvider translates the config into the actual providerAccess object.
func (ip *PublicIPConfig) ToProvider(base *armnetwork.PublicIPAddress) *armnetwork.PublicIPAddress {
	if base == nil {
		base = &armnetwork.PublicIPAddress{}
	}
	target := &armnetwork.PublicIPAddress{
		Location: to.Ptr(ip.Location),
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
		},
		SKU: &armnetwork.PublicIPAddressSKU{
			Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard),
			Tier: to.Ptr(armnetwork.PublicIPAddressSKUTierRegional),
		},
		Name: to.Ptr(ip.Name),
	}
	if len(ip.Zones) > 0 {
		// if no zones selected, zones has to be nil, to match what the API returns - otherwise reflect.DeepEqual fails the check.
		target.Zones = to.SliceOfPtrs(ip.Zones...)
	}

	target.Tags = utils.MergeStringMaps(base.Tags, map[string]*string{
		TagManagedByGardener: to.Ptr("true"),
		TagShootName:         to.Ptr(ip.ShootName),
	})

	// inherited from base
	target.ID = base.ID
	return target
}

// ToProvider translates the config into the actual providerAccess object.
func (nat *NatGatewayConfig) ToProvider(base *armnetwork.NatGateway) *armnetwork.NatGateway {
	target := &armnetwork.NatGateway{
		ID:       nil,
		Location: to.Ptr(nat.Location),
		Properties: &armnetwork.NatGatewayPropertiesFormat{
			IdleTimeoutInMinutes: nat.IdleTimeout,
		},
		SKU: &armnetwork.NatGatewaySKU{
			Name: to.Ptr(armnetwork.NatGatewaySKUNameStandard),
		},
		Name: to.Ptr(nat.Name),
	}
	if nat.Zone != nil {
		target.Zones = []*string{nat.Zone}
	}

	// inherited from base
	if base != nil {
		target.Properties.PublicIPPrefixes = base.Properties.PublicIPPrefixes
		target.ID = base.ID
	}
	return target
}

// ToProvider translates the config into the actual providerAccess object.
func (s *SubnetConfig) ToProvider(base *armnetwork.Subnet) *armnetwork.Subnet {
	target := &armnetwork.Subnet{
		Name: to.Ptr(s.Name),
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix:         to.Ptr(s.cidr),
			DefaultOutboundAccess: to.Ptr(s.defaultOutboundAccess),
		},
		Etag: nil,
	}
	for _, endpoint := range s.serviceEndpoint {
		target.Properties.ServiceEndpoints = append(target.Properties.ServiceEndpoints, &armnetwork.ServiceEndpointPropertiesFormat{
			Service: to.Ptr(endpoint),
		})
	}

	// inherited from base
	if base != nil {
		target.ID = base.ID
		// Code comments suggest that the option to set DefaultOutboundAccess is only applicable during shoot creation.
		// However, the API does not enforce this restriction, and it can be updated later as well.
		// target.Properties.DefaultOutboundAccess = base.Properties.DefaultOutboundAccess

		// For now, use whatever is already existing in the remote object. We will later overwrite them with what we consider appropriate.
		target.Properties.NatGateway = base.Properties.NatGateway
		target.Properties.NetworkSecurityGroup = base.Properties.NetworkSecurityGroup
		target.Properties.RouteTable = base.Properties.RouteTable

		target.Properties.ServiceEndpointPolicies = base.Properties.ServiceEndpointPolicies
		target.Properties.PrivateLinkServiceNetworkPolicies = base.Properties.PrivateLinkServiceNetworkPolicies

		target.Properties.PrivateEndpoints = base.Properties.PrivateEndpoints
		target.Properties.PrivateEndpointNetworkPolicies = base.Properties.PrivateEndpointNetworkPolicies
		target.Properties.Delegations = base.Properties.Delegations
	}

	return target
}

// ToProvider translates the config into the actual providerAccess object.
func (v *VirtualNetworkConfig) ToProvider(base *armnetwork.VirtualNetwork) *armnetwork.VirtualNetwork {
	desired := &armnetwork.VirtualNetwork{
		Location:   to.Ptr(v.Location),
		Name:       to.Ptr(v.Name),
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{},
	}

	if base != nil {
		desired.Tags = base.Tags
		if base.Properties != nil {
			desired.Properties = base.Properties
		}
	}

	// apply the desired changes in place.
	desired.Properties.AddressSpace = &armnetwork.AddressSpace{
		AddressPrefixes: []*string{v.CIDR},
	}
	if ddosId := v.DDoSPlanID; ddosId != nil {
		desired.Properties.EnableDdosProtection = to.Ptr(true)
		desired.Properties.DdosProtectionPlan = &armnetwork.SubResource{ID: ddosId}
	} else {
		desired.Properties.DdosProtectionPlan = nil
		desired.Properties.EnableDdosProtection = to.Ptr(false)
	}

	return desired
}

// ToProvider translates the config into the actual providerAccess object.
func (r *SecurityGroupConfig) ToProvider(base *armnetwork.SecurityGroup) *armnetwork.SecurityGroup {
	desired := &armnetwork.SecurityGroup{
		Location:   to.Ptr(r.Location),
		Name:       to.Ptr(r.Name),
		Properties: &armnetwork.SecurityGroupPropertiesFormat{},
	}

	if base != nil && base.Properties != nil {
		desired.Properties = base.Properties
	}

	return desired
}

// ToProvider translates the config into the actual providerAccess object.
func (r *RouteTableConfig) ToProvider(base *armnetwork.RouteTable) *armnetwork.RouteTable {
	desired := &armnetwork.RouteTable{
		Location:   to.Ptr(r.Location),
		Name:       to.Ptr(r.Name),
		Properties: &armnetwork.RouteTablePropertiesFormat{},
	}
	if base != nil && base.Properties != nil {
		desired.Properties = base.Properties
	}

	return desired
}
