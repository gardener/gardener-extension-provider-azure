// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"fmt"
	"reflect"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
)

// AzureResourceKind is a string describing the resource type.
type AzureResourceKind string

func (a AzureResourceKind) String() string {
	return string(a)
}

const (
	// KindAvailabilitySet is the kind for an availability set.
	KindAvailabilitySet AzureResourceKind = "Microsoft.Compute/availabilitySets"
	// KindNatGateway is the kind for a NAT Gateway.
	KindNatGateway AzureResourceKind = "Microsoft.Network/natGateways"
	// KindPublicIP is the kind for a public ip.
	KindPublicIP AzureResourceKind = "Microsoft.Network/publicIPAddresses"
	// KindResourceGroup is the kind for a resource group.
	KindResourceGroup AzureResourceKind = "Microsoft.Resources/resourceGroups"
	// KindRouteTable is the kind for a route table.
	KindRouteTable AzureResourceKind = "Microsoft.Network/routeTables"
	// KindSecurityGroup is the kind for a security group.
	KindSecurityGroup AzureResourceKind = "Microsoft.Network/networkSecurityGroups"
	// KindSubnet is the kind for a subnet
	KindSubnet AzureResourceKind = "Microsoft.Network/virtualNetworks/subnets"
	// KindVirtualNetwork is the kind for a virtual network.
	KindVirtualNetwork AzureResourceKind = "Microsoft.Network/virtualNetworks"
	// KindLoadBalancer is the kind for a load balancer.
	KindLoadBalancer AzureResourceKind = "Microsoft.Network/loadBalancers"
)

const (
	// KeyPublicIPAddresses is the key used to store public IP addresses in the FlowContext's whiteboard.
	KeyPublicIPAddresses = "PublicIpAddresses"
)

const (
	// TemplateAvailabilitySet the template for the ID of an availability set.
	TemplateAvailabilitySet = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/availabilitySets/%s"
	// TemplateNatGateway the template for the id of a NAT Gateway.
	TemplateNatGateway = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/natGateways/%s"
	// TemplatePublicIP the template for the id of a public IP.
	TemplatePublicIP = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/publicIPAddresses/%s"
	// TemplateResourceGroup is the template for the id of a resource group.
	TemplateResourceGroup = "/subscriptions/%s/resourceGroups/%s"
	// TemplateRouteTable is the template for the id of a route table.
	TemplateRouteTable = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/routeTables/%s"
	// TemplateSecurityGroup is the template for the id of a security group.
	TemplateSecurityGroup = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/networkSecurityGroups/%s"
	// TemplateVirtualNetwork is the template for the id of a virtual network.
	TemplateVirtualNetwork = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s"
	// TemplateSubnet is the template for the id of a subnet.
	TemplateSubnet                  = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/%s"
	TemplateFrontendIPConfiguration = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers/%s/frontendIPConfigurations/%s"
	TemplateBackendAddressPool      = "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers/%s/backendAddressPools/%s"
)

// ResourceGroupIdFromTemplate returns the id of a resource group.
func ResourceGroupIdFromTemplate(subscription, name string) string {
	return fmt.Sprintf(TemplateResourceGroup, subscription, name)
}

// GetIdFromTemplate returns the ID of a resource based on the target template.
func GetIdFromTemplate(template, subscription, rgName, name string) string {
	return fmt.Sprintf(template, subscription, rgName, name)
}

// GetIdFromTemplateWithParent returns the ID of a resource based on the target template.
func GetIdFromTemplateWithParent(template, subscription, rgName, parent, name string) string {
	return fmt.Sprintf(template, subscription, rgName, parent, name)
}

// AzureResourceMetadata is able to uniquely identify a resource.
type AzureResourceMetadata struct {
	ResourceGroup string
	Name          string
	Parent        string
	Kind          AzureResourceKind
}

// ForceNewIp checks if the resource can be reconciled. If not, returns the name of the field and value that couldn't be updated.
func ForceNewIp(current, target *armnetwork.PublicIPAddress) (bool, string, any) {
	if !reflect.DeepEqual(current.Location, target.Location) {
		return true, "Location", *current.Location
	}
	if !reflect.DeepEqual(current.Zones, target.Zones) {
		return true, "Zones", current.Zones
	}
	if !reflect.DeepEqual(current.Properties.PublicIPAllocationMethod, target.Properties.PublicIPAllocationMethod) {
		return true, "PublicIPAllocationMethod", current.Properties.PublicIPAllocationMethod
	}
	return false, "", nil
}

// ForceNewNat checks if the resource can be reconciled. If not, returns the name of the field and value that couldn't be updated.
func ForceNewNat(current, target *armnetwork.NatGateway) (bool, string, any) {
	if !reflect.DeepEqual(current.Location, target.Location) {
		return true, "Location", *current.Location
	}

	if !reflect.DeepEqual(current.Zones, target.Zones) {
		return true, "Zones", current.Zones
	}

	return false, "", nil
}

// ForceNewSubnet checks if the resource can be reconciled. If not, returns the name of the field and value that couldn't be updated.
func ForceNewSubnet(_, _ *armnetwork.Subnet) (bool, string, any) {
	return false, "", nil
}
