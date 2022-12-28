// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package infraflow

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// TfResourceGroup is the attribute name for the isCreate function of the tf adapter
const TfResourceGroup = "resourceGroup"

// TfVnet is the attribute name for the isCreate function of the tf adapter
const TfVnet = "vnet"

// TfAvailabilitySet is the attribute name for the isCreate function of the tf adapter
const TfAvailabilitySet = "availabilitySet"

// TerraformAdapter retrieves all configuration logic needed for reconcilation from the (soon) legacy Terraform configuration code
type TerraformAdapter struct {
	values map[string]interface{}
	config *azure.InfrastructureConfig
}

// RouteTableName returns the name of the route table
func (t TerraformAdapter) RouteTableName() string {
	return "worker_route_table"
}

// SecurityGroupName returns the name of the security group
func (t TerraformAdapter) SecurityGroupName() string {
	return t.ClusterName() + "-workers"
}

type identityTf struct {
	Name          string
	ResourceGroup string
}

type availablilitySetTf struct {
	Name               string
	CountFaultDomains  int32
	CountUpdateDomains int32
}

// AvailabilitySet returns the availability set configuration
func (t TerraformAdapter) AvailabilitySet() availablilitySetTf {
	return availablilitySetTf{t.availabilitySetName(), t.countFaultDomains(), t.countUpdateDomains()}
}

// Identity returns the identity configuration
func (t TerraformAdapter) Identity() *identityTf {
	identity := t.values["identity"].(map[string]interface{})
	name, ok := identity["name"]
	if !ok {
		return nil
	}
	resourceGroup, ok := identity["resourceGroup"]
	if !ok {
		return nil
	}
	return &identityTf{name.(string), resourceGroup.(string)}
}

// StaticInfrastructureStatus returns the status part that only depends on the configuration and does not rely on metadata from resource reconcilation
func (t TerraformAdapter) StaticInfrastructureStatus() *v1alpha1.InfrastructureStatus {
	infraState := v1alpha1.InfrastructureStatus{
		TypeMeta: infrastructure.StatusTypeMeta,
		ResourceGroup: v1alpha1.ResourceGroup{
			Name: t.ResourceGroup(),
		},
		Networks: v1alpha1.NetworkStatus{
			VNet: v1alpha1.VNetStatus{
				Name: t.Vnet().Name(), // if not set by user then assumes tf default (no state necessary)
			},
		},
		AvailabilitySets: []v1alpha1.AvailabilitySet{},
		RouteTables: []v1alpha1.RouteTable{
			{Purpose: v1alpha1.PurposeNodes, Name: t.RouteTableName()},
		},
		SecurityGroups: []v1alpha1.SecurityGroup{
			{Name: t.SecurityGroupName(), Purpose: v1alpha1.PurposeNodes},
		},
		Zoned: false,
	}
	if t.config.Zoned {
		infraState.Zoned = true
	}

	if t.config.Networks.Workers == nil {
		infraState.Networks.Layout = v1alpha1.NetworkLayoutMultipleSubnet
	} else {
		infraState.Networks.Layout = v1alpha1.NetworkLayoutSingleSubnet
	}

	for _, subnet := range t.Zones() {
		subnetV1 := v1alpha1.Subnet{
			Name:    subnet.SubnetName(),
			Purpose: v1alpha1.PurposeNodes,
			Zone:    subnet.Zone(),
		}
		if subnet.migrated != nil {
			subnetV1.Migrated = *subnet.migrated
		}
		infraState.Networks.Subnets = append(infraState.Networks.Subnets, subnetV1)
	}

	infraState.Networks.VNet.ResourceGroup = t.Vnet().ResourceGroup()

	return &infraState
}

// NewTerraformAdapter creates a new TerraformAdapter
func NewTerraformAdapter(infra *extensionsv1alpha1.Infrastructure, config *azure.InfrastructureConfig, cluster *controller.Cluster) (TerraformAdapter, error) {
	tfValues, err := infrastructure.ComputeTerraformerTemplateValues(infra, config, cluster) // use for migration of values..
	return TerraformAdapter{tfValues, config}, err
}

func (t TerraformAdapter) isCreate(resource string) bool {
	create := t.values["create"].(map[string]interface{})
	return create[resource].(bool)
}

// Vnet returns the vnet configuration
func (t TerraformAdapter) Vnet() vnetTf {
	cm := t.values["resourceGroup"].(map[string]interface{})
	return vnetTf(cm["vnet"].(map[string]interface{}))
}

// ResourceGroup returns the resource group name
func (t TerraformAdapter) ResourceGroup() string {
	return t.values["resourceGroup"].(map[string]interface{})["name"].(string)
}

// Region returns the region
func (t TerraformAdapter) Region() string {
	return t.values["azure"].(map[string]interface{})["region"].(string)
}

func (t TerraformAdapter) countUpdateDomains() int32 {
	return t.values["azure"].(map[string]interface{})["countUpdateDomains"].(int32)
}

func (t TerraformAdapter) countFaultDomains() int32 {
	return t.values["azure"].(map[string]interface{})["countFaultDomains"].(int32)
}

// ClusterName returns the cluster name
func (t TerraformAdapter) ClusterName() string {
	return t.values["clusterName"].(string)
}

func (t TerraformAdapter) subnetName(subnet map[string]interface{}) string {
	name := t.ClusterName() + "-nodes"
	isMigrated, isMultiSubnet := subnet["migrated"]
	if isMultiSubnet {
		if !isMigrated.(bool) {
			name = fmt.Sprintf("%s-z%d", name, subnet["name"].(int32))
		}
	}
	return name
}

// EnabledNats returns all zones with NAT enabled
func (t TerraformAdapter) EnabledNats() []zoneTf {
	res := make([]zoneTf, 0)
	for _, nat := range t.Zones() {
		if nat.enabled {
			res = append(res, nat)
		}
	}
	return res
}

func (t TerraformAdapter) availabilitySetName() string {
	return t.ClusterName() + "-avset-workers"
}

type userManagedIP struct {
	Name          string
	ResourceGroup string
	SubnetName    string
}

// UserManagedIPs returns all user managed IPs
// tpl l.139
func (t TerraformAdapter) UserManagedIPs() []userManagedIP {
	res := make([]userManagedIP, 0)
	rawSubnets := t.values["networks"].(map[string]interface{})["subnets"]
	if rawSubnets == nil {
		return res
	}
	for _, subnet := range rawSubnets.([]map[string]interface{}) {
		subnetName := t.subnetName(subnet)
		natRaw := subnet["natGateway"].(map[string]interface{})
		_, ok := natRaw["zone"]
		if ok {
			ipAddrRaw, ipOk := natRaw["ipAddresses"]
			if ipOk {
				ipAddrs := ipAddrRaw.([]map[string]interface{})
				for _, addr := range ipAddrs {
					ipName := addr["name"].(string)
					ipRgroup := addr["resourceGroup"].(string)
					res = append(res, userManagedIP{ipName, ipRgroup, subnetName})
				}
			}
		}
	}
	return res
}

// Zones returns all zone configurations
func (t TerraformAdapter) Zones() []zoneTf {
	res := make([]zoneTf, 0)
	rawSubnets := t.values["networks"].(map[string]interface{})["subnets"]
	if rawSubnets == nil {
		return res
	}
	for _, subnet := range rawSubnets.([]map[string]interface{}) {
		natRaw := subnet["natGateway"].(map[string]interface{})

		var idleConnectionTimeoutMinutes *int32
		if _, ok := natRaw["idleConnectionTimeoutMinutes"]; ok {
			idleConnectionTimeoutMinutes = to.Ptr(natRaw["idleConnectionTimeoutMinutes"].(int32))
		}
		cidr := subnet["cidr"].(string)
		serviceEndpoints := subnet["serviceEndpoints"].([]string)

		// only for multi subnets
		var isMigrated *bool
		isMigratedRaw, isMultiSubnet := subnet["migrated"]
		if isMultiSubnet {
			isMigrated = to.Ptr(isMigratedRaw.(bool))
		}

		var zone *string
		zoneRaw, ok := natRaw["zone"]
		if ok {
			zone = to.Ptr(fmt.Sprintf("%d", zoneRaw.(int32)))
		}

		var rawNetNumber *int32
		netNumberRaw, ok := subnet["name"]
		if ok {
			rawNetNumber = to.Ptr(netNumberRaw.(int32))
		}
		res = append(res, zoneTf{rawNetNumber, natRaw["enabled"].(bool), idleConnectionTimeoutMinutes, isMigrated, t.ClusterName(), zone, cidr, serviceEndpoints})
	}
	return res
}

type zoneTf struct {
	rawNetNumber                 *int32
	enabled                      bool
	idleConnectionTimeoutMinutes *int32
	migrated                     *bool
	clusterName                  string
	zone                         *string
	cidr                         string
	serviceEndpoints             []string
}

func (nat zoneTf) NatName() string {
	name := nat.clusterName + "-nat-gateway"
	if nat.migrated != nil && !*nat.migrated && nat.rawNetNumber != nil {
		name = fmt.Sprintf("%s-z%d", name, *nat.rawNetNumber)
	}
	return name
}

func (nat zoneTf) SubnetName() string {
	name := nat.clusterName + "-nodes"
	if nat.migrated != nil && !*nat.migrated && nat.rawNetNumber != nil {
		name = fmt.Sprintf("%s-z%d", name, *nat.rawNetNumber)
	}
	return name
}

func (nat zoneTf) IpName() string {
	return nat.NatName() + "-ip"
}

func (nat zoneTf) Zone() *string {
	return nat.zone
}

type vnetTf (map[string]interface{})

func (v vnetTf) Cidr() *string {
	val, ok := v["cidr"]
	if ok {
		return to.Ptr(val.(string))
	} else {
		return nil
	}
}

func (v vnetTf) DDosProtectionPlanID() *string {
	val, ok := v["ddosProtectionPlanID"]
	if ok {
		return to.Ptr(val.(string))
	} else {
		return nil
	}
}

func (v vnetTf) Name() string {
	return v["name"].(string)
}

func (v vnetTf) ResourceGroup() *string {
	val, ok := v["resourceGroup"]
	if !ok {
		return nil
	}
	return to.Ptr(val.(string))
}
