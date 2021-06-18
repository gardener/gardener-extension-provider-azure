// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azure

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureConfig infrastructure configuration resource
type InfrastructureConfig struct {
	metav1.TypeMeta
	// ResourceGroup is azure resource group
	ResourceGroup *ResourceGroup
	// Networks is the network configuration (VNets, subnets, etc.)
	Networks NetworkConfig
	// Identity contains configuration for the assigned managed identity.
	Identity *IdentityConfig
	// Zoned indicates whether the cluster uses zones
	Zoned bool
}

// ResourceGroup is azure resource group
type ResourceGroup struct {
	// Name is the name of the resource group
	Name string
}

// NetworkConfig holds information about the Kubernetes and infrastructure networks.
type NetworkConfig struct {
	// VNet indicates whether to use an existing VNet or create a new one.
	VNet VNet
	// Workers is the worker subnet range to create (used for the VMs).
	Workers *string
	// NatGateway contains the configuration for the NatGateway.
	NatGateway *NatGatewayConfig
	// ServiceEndpoints is a list of Azure ServiceEndpoints which should be associated with the worker subnet.
	ServiceEndpoints []string
	// Zones is a list of zones with their respective configuration.
	Zones []Zone
}

// Zone describes the configuration for a subnet that is used for VMs on that region.
type Zone struct {
	// Name is the name of the zone and should match with the name the infrastructure provider is using for the zone.
	Name int32
	// CIDR is the CIDR range used for the zone's subnet.
	CIDR string
	// ServiceEndpoints is a list of Azure ServiceEndpoints which should be associated with the zone's subnet.
	ServiceEndpoints []string
	// NatGateway contains the configuration for the NatGateway associated with this subnet.
	NatGateway *NatGatewayConfig
}

// NatGatewayConfig contains configuration for the NAT gateway and the attached resources.
type NatGatewayConfig struct {
	// Enabled is an indicator if NAT gateway should be deployed.
	Enabled bool
	// IdleConnectionTimeoutMinutes specifies the idle connection timeout limit for NAT gateway in minutes.
	IdleConnectionTimeoutMinutes *int32
	// Zone specifies the zone in which the NAT gateway should be deployed to.
	Zone *int32
	// IPAddresses is a list of ip addresses which should be assigned to the NAT gateway.
	IPAddresses []PublicIPReference
}

// PublicIPReference contains information about a public ip.
type PublicIPReference struct {
	// Name is the name of the public ip.
	Name string
	// ResourceGroup is the name of the resource group where the public ip is assigned to.
	ResourceGroup string
	// Zone is the zone in which the public ip is deployed to.
	Zone int32
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureStatus contains information about created infrastructure resources.
type InfrastructureStatus struct {
	metav1.TypeMeta
	// Networks is the status of the networks of the infrastructure.
	Networks NetworkStatus
	// ResourceGroup is azure resource group
	ResourceGroup ResourceGroup
	// AvailabilitySets is a list of created availability sets
	AvailabilitySets []AvailabilitySet
	// AvailabilitySets is a list of created route tables
	RouteTables []RouteTable
	// SecurityGroups is a list of created security groups
	SecurityGroups []SecurityGroup
	// Identity is the status of the managed identity.
	Identity *IdentityStatus
	// Zoned indicates whether the cluster uses zones
	Zoned bool
	// NatGatewayPublicIPMigrated is an indicator if the Gardener managed public ip address is already migrated.
	// TODO(natipmigration) This can be removed in future versions when the ip migration has been completed.
	NatGatewayPublicIPMigrated bool
}

// NetworkStatus is the current status of the infrastructure networks.
type NetworkStatus struct {
	// VNet states the name of the infrastructure VNet.
	VNet VNetStatus
	// Subnets are the subnets that have been created.
	Subnets []Subnet
	// Topology describes the network topology of the cluster.
	Topology NetworkTopologyType
}

// Purpose is a purpose of a subnet.
type Purpose string

const (
	// PurposeNodes is a Purpose for nodes.
	PurposeNodes Purpose = "nodes"
	// PurposeInternal is a Purpose for internal use.
	PurposeInternal Purpose = "internal"
)

// NetworkTopologyType is the network topology type for the cluster.
type NetworkTopologyType string

const (
	// TopologyRegional is a network topology for clusters that do not make use of availability zones.
	TopologyRegional NetworkTopologyType = "regional"
	// TopologyZonalSingleSubnet is a network topology for zonal clusters. Clusters with this topology have a single
	// subnet that is shared among all availability zones.
	TopologyZonalSingleSubnet NetworkTopologyType = "zonalSingleSubnet"
	// TopologyZonal is a network topology for zonal clusters, where a subnet is created for each availability zone.
	TopologyZonal NetworkTopologyType = "zonal"
)

// Subnet is a subnet that was created.
type Subnet struct {
	// Name is the name of the subnet.
	Name string
	// Purpose is the purpose for which the subnet was created.
	Purpose Purpose
	// Zone is the name of the zone for which the subnet was created.
	Zone *string
}

// AvailabilitySet contains information about the azure availability set
type AvailabilitySet struct {
	// Purpose is the purpose of the availability set
	Purpose Purpose
	// ID is the id of the availability set
	ID string
	// Name is the name of the availability set
	Name string
	// CountFaultDomains is the count of fault domains.
	CountFaultDomains *int32
	// CountUpdateDomains is the count of update domains.
	CountUpdateDomains *int32
}

// RouteTable is the azure route table
type RouteTable struct {
	// Purpose is the purpose of the route table
	Purpose Purpose
	// Name is the name of the route table
	Name string
}

// SecurityGroup contains information about the security group
type SecurityGroup struct {
	// Purpose is the purpose of the security group
	Purpose Purpose
	// Name is the name of the security group
	Name string
}

// VNet contains information about the VNet and some related resources.
type VNet struct {
	// Name is the VNet name.
	Name *string
	// ResourceGroup is the resource group where the existing vNet belongs to.
	ResourceGroup *string
	// CIDR is the VNet CIDR
	CIDR *string
}

// VNetStatus contains the VNet name.
type VNetStatus struct {
	// Name is the VNet name.
	Name string
	// ResourceGroup is the resource group where the existing vNet belongs to.
	ResourceGroup *string
}

// IdentityConfig contains configuration for the managed identity.
type IdentityConfig struct {
	// Name is the name of the identity.
	Name string
	// ResourceGroup is the resource group where the identity belongs to.
	ResourceGroup string
	// ACRAccess indicated if the identity should be used by the Shoot worker nodes to pull from an Azure Container Registry.
	ACRAccess *bool
}

// IdentityStatus contains the status information of the created managed identity.
type IdentityStatus struct {
	// ID is the Azure resource if of the identity.
	ID string
	// ClientID is the client id of the identity.
	ClientID string
	// ACRAccess specifies if the identity should be used by the Shoot worker nodes to pull from an Azure Container Registry.
	ACRAccess bool
}
