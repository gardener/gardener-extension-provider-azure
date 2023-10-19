//  Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

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
)

// ResourceGroupIdFromTemplate returns the id of a resource group.
func ResourceGroupIdFromTemplate(subscription, name string) string {
	return fmt.Sprintf(TemplateResourceGroup, subscription, name)
}

// GetIdFromTemplate returns the ID of a resource based on the target template.
func GetIdFromTemplate(template, subscription, rgName, name string) string {
	return fmt.Sprintf(template, subscription, rgName, name)
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
