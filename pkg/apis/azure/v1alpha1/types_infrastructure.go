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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureConfig infrastructure configuration resource
type InfrastructureConfig struct {
	metav1.TypeMeta `json:",inline"`
	// ResourceGroup is azure resource group.
	// +optional
	ResourceGroup *ResourceGroup `json:"resourceGroup,omitempty"`
	// Networks is the network configuration (VNet, subnets, etc.).
	Networks NetworkConfig `json:"networks"`
	// Identity contains configuration for the assigned managed identity.
	// +optional
	Identity *IdentityConfig `json:"identity,omitempty"`
	// Zoned indicates whether the cluster uses availability zones.
	// +optional
	Zoned bool `json:"zoned,omitempty"`
}

// ResourceGroup is azure resource group
type ResourceGroup struct {
	// Name is the name of the resource group
	Name string `json:"name"`
}

// NetworkConfig holds information about the Kubernetes and infrastructure networks.
type NetworkConfig struct {
	// VNet indicates whether to use an existing VNet or create a new one.
	VNet VNet `json:"vnet"`
	// Workers is the worker subnet range to create (used for the VMs).
	// +optional
	Workers *string `json:"workers"`
	// NatGateway contains the configuration for the NatGateway.
	// +optional
	NatGateway *NatGatewayConfig `json:"natGateway,omitempty"`
	// ServiceEndpoints is a list of Azure ServiceEndpoints which should be associated with the worker subnet.
	// +optional
	ServiceEndpoints []string `json:"serviceEndpoints,omitempty"`
	// Zones is a list of zones with their respective configuration.
	Zones []Zone `json:"zones,omitempty"`
}

// Zone describes the configuration for a subnet that is used for VMs on that region.
type Zone struct {
	// Name is the name of the zone and should match with the name the infrastructure provider is using for the zone.
	Name int32 `json:"name"`
	// CIDR is the CIDR range used for the zone's subnet.
	CIDR string `json:"cidr"`
	// ServiceEndpoints is a list of Azure ServiceEndpoints which should be associated with the zone's subnet.
	// +optional
	ServiceEndpoints []string `json:"serviceEndpoints,omitempty"`
	// NatGateway contains the configuration for the NatGateway associated with this subnet.
	// +optional
	NatGateway *NatGatewayConfig `json:"natGateway,omitempty"`
}

// NatGatewayConfig contains configuration for the NAT gateway and the attached resources.
type NatGatewayConfig struct {
	// Enabled is an indicator if NAT gateway should be deployed.
	Enabled bool `json:"enabled"`
	// IdleConnectionTimeoutMinutes specifies the idle connection timeout limit for NAT gateway in minutes.
	// +optional
	IdleConnectionTimeoutMinutes *int32 `json:"idleConnectionTimeoutMinutes,omitempty"`
	// Zone specifies the zone in which the NAT gateway should be deployed to.
	// +optional
	Zone *int32 `json:"zone,omitempty"`
	// IPAddresses is a list of ip addresses which should be assigned to the NAT gateway.
	// +optional
	IPAddresses []PublicIPReference `json:"ipAddresses,omitempty"`
}

// PublicIPReference contains information about a public ip.
type PublicIPReference struct {
	// Name is the name of the public ip.
	Name string `json:"name"`
	// ResourceGroup is the name of the resource group where the public ip is assigned to.
	ResourceGroup string `json:"resourceGroup"`
	// Zone is the zone in which the public ip is deployed to.
	Zone int32 `json:"zone"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// InfrastructureStatus contains information about created infrastructure resources.
type InfrastructureStatus struct {
	metav1.TypeMeta `json:",inline"`
	// Networks is the status of the networks of the infrastructure.
	Networks NetworkStatus `json:"networks"`
	// ResourceGroup is azure resource group
	ResourceGroup ResourceGroup `json:"resourceGroup"`
	// AvailabilitySets is a list of created availability sets
	AvailabilitySets []AvailabilitySet `json:"availabilitySets"`
	// AvailabilitySets is a list of created route tables
	RouteTables []RouteTable `json:"routeTables"`
	// SecurityGroups is a list of created security groups
	SecurityGroups []SecurityGroup `json:"securityGroups"`
	// Identity is the status of the managed identity.
	// +optional
	Identity *IdentityStatus `json:"identity,omitempty"`
	// Zoned indicates whether the cluster uses zones
	// +optional
	Zoned bool `json:"zoned,omitempty"`
	// NatGatewayPublicIPMigrated is an indicator if the Gardener managed public ip address is already migrated.
	// TODO(natipmigration) This can be removed in future versions when the ip migration has been completed.
	// +optional
	NatGatewayPublicIPMigrated bool `json:"natGatewayPublicIpMigrated,omitempty"`
}

// NetworkStatus is the current status of the infrastructure networks.
type NetworkStatus struct {
	// VNetStatus states the name of the infrastructure VNet.
	VNet VNetStatus `json:"vnet"`

	// Subnets are the subnets that have been created.
	Subnets []Subnet `json:"subnets"`

	// Topology describes the network topology of the cluster.
	Topology NetworkTopologyType `json:"topology"`
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
	Name string `json:"name"`
	// Purpose is the purpose for which the subnet was created.
	Purpose Purpose `json:"purpose"`
	// Zone is the name of the zone for which the subnet was created.
	// +optional
	Zone *string `json:"zone,omitempty"`
}

// AvailabilitySet contains information about the azure availability set
type AvailabilitySet struct {
	// Purpose is the purpose of the availability set
	Purpose Purpose `json:"purpose"`
	// ID is the id of the availability set
	ID string `json:"id"`
	// Name is the name of the availability set
	Name string `json:"name"`
	// CountFaultDomains is the count of fault domains.
	// +optional
	CountFaultDomains *int32 `json:"countFaultDomains,omitempty"`
	// CountUpdateDomains is the count of update domains.
	// +optional
	CountUpdateDomains *int32 `json:"countUpdateDomains,omitempty"`
}

// RouteTable is the azure route table
type RouteTable struct {
	// Purpose is the purpose of the route table
	Purpose Purpose `json:"purpose"`
	// Name is the name of the route table
	Name string `json:"name"`
}

// SecurityGroup contains information about the security group
type SecurityGroup struct {
	// Purpose is the purpose of the security group
	Purpose Purpose `json:"purpose"`
	// Name is the name of the security group
	Name string `json:"name"`
}

// VNet contains information about the VNet and some related resources.
type VNet struct {
	// Name is the name of an existing vNet which should be used.
	// +optional
	Name *string `json:"name,omitempty"`
	// ResourceGroup is the resource group where the existing vNet blongs to.
	// +optional
	ResourceGroup *string `json:"resourceGroup,omitempty"`
	// CIDR is the VNet CIDR
	// +optional
	CIDR *string `json:"cidr,omitempty"`
}

// VNetStatus contains the VNet name.
type VNetStatus struct {
	// Name is the VNet name.
	Name string `json:"name"`
	// ResourceGroup is the resource group where the existing vNet belongs to.
	// +optional
	ResourceGroup *string `json:"resourceGroup,omitempty"`
}

// IdentityConfig contains configuration for the managed identity.
type IdentityConfig struct {
	// Name is the name of the identity.
	Name string `json:"name"`
	// ResourceGroup is the resource group where the identity belongs to.
	ResourceGroup string `json:"resourceGroup"`
	// ACRAccess indicated if the identity should be used by the Shoot worker nodes to pull from an Azure Container Registry.
	// +optional
	ACRAccess *bool `json:"acrAccess,omitempty"`
}

// IdentityStatus contains the status information of the created managed identity.
type IdentityStatus struct {
	// ID is the Azure resource if of the identity.
	ID string `json:"id"`
	// ClientID is the client id of the identity.
	ClientID string `json:"clientID"`
	// ACRAccess specifies if the identity should be used by the Shoot worker nodes to pull from an Azure Container Registry.
	ACRAccess bool `json:"acrAccess"`
}
