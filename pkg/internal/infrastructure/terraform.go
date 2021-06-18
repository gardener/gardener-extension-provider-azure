// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

const (
	// TerraformerPurpose is the terraformer infrastructure purpose.
	TerraformerPurpose = "infra"

	// TerraformerOutputKeyResourceGroupName is the key for the resourceGroupName output
	TerraformerOutputKeyResourceGroupName = "resourceGroupName"
	// TerraformerOutputKeyVNetName is the key for the vnetName output
	TerraformerOutputKeyVNetName = "vnetName"
	// TerraformerOutputKeyVNetResourceGroup is the key for the vnetResourceGroup output
	TerraformerOutputKeyVNetResourceGroup = "vnetResourceGroup"
	// TerraformerOutputKeySubnetName is the key for the subnetName output
	TerraformerOutputKeySubnetName = "subnetName"
	// TerraformerOutputKeySubnetNamePrefix is the key for the subnetName output
	TerraformerOutputKeySubnetNamePrefix = "subnetName-z"
	// TerraformerOutputKeyAvailabilitySetID is the key for the availabilitySetID output
	TerraformerOutputKeyAvailabilitySetID = "availabilitySetID"
	// TerraformerOutputKeyAvailabilitySetName is the key for the availabilitySetName output
	TerraformerOutputKeyAvailabilitySetName = "availabilitySetName"
	// TerraformerOutputKeyCountFaultDomains is the key for the fault domain count output.
	TerraformerOutputKeyCountFaultDomains = "countFaultDomains"
	// TerraformerOutputKeyCountUpdateDomains is the key for the update domain count output.
	TerraformerOutputKeyCountUpdateDomains = "countUpdateDomains"
	// TerraformerOutputKeyRouteTableName is the key for the routeTableName output
	TerraformerOutputKeyRouteTableName = "routeTableName"
	// TerraformerOutputKeySecurityGroupName is the key for the securityGroupName output
	TerraformerOutputKeySecurityGroupName = "securityGroupName"
	// TerraformerOutputKeyIdentityID is the key for the identityID output
	TerraformerOutputKeyIdentityID = "identityID"
	// TerraformerOutputKeyIdentityClientID is the key for the identityClientID output
	TerraformerOutputKeyIdentityClientID = "identityClientID"
)

// StatusTypeMeta is the TypeMeta of the Azure InfrastructureStatus
var StatusTypeMeta = metav1.TypeMeta{
	APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
	Kind:       "InfrastructureStatus",
}

// RenderTerraformerTemplate renders the azure infrastructure template with the given values.
func RenderTerraformerTemplate(
	infra *extensionsv1alpha1.Infrastructure,
	config *api.InfrastructureConfig,
	cluster *controller.Cluster,
) (
	*TerraformFiles,
	error,
) {
	values, err := ComputeTerraformerTemplateValues(infra, config, cluster)
	if err != nil {
		return nil, err
	}

	var mainTF bytes.Buffer

	if err := mainTemplate.Execute(&mainTF, values); err != nil {
		return nil, fmt.Errorf("could not render Terraform template: %+v", err)
	}

	return &TerraformFiles{
		Main:      mainTF.String(),
		Variables: variablesTF,
		TFVars:    terraformTFVars,
	}, nil
}

// ComputeTerraformerTemplateValues computes the values for the Azure Terraformer chart.
func ComputeTerraformerTemplateValues(
	infra *extensionsv1alpha1.Infrastructure,
	config *api.InfrastructureConfig,
	cluster *controller.Cluster,
) (
	map[string]interface{},
	error,
) {
	var (
		createResourceGroup   = true
		createVNet            = true
		createAvailabilitySet = false
		resourceGroupName     = infra.Namespace

		identityConfig map[string]interface{}
		azureConfig    = map[string]interface{}{
			"region": infra.Spec.Region,
		}
		vnetConfig = map[string]interface{}{
			"name": infra.Namespace,
		}
		outputKeys = map[string]interface{}{
			"resourceGroupName": TerraformerOutputKeyResourceGroupName,
			"vnetName":          TerraformerOutputKeyVNetName,
			"subnetName":        TerraformerOutputKeySubnetName,
			"routeTableName":    TerraformerOutputKeyRouteTableName,
			"securityGroupName": TerraformerOutputKeySecurityGroupName,
		}
	)

	primaryAvSetRequired, err := isPrimaryAvailabilitySetRequired(infra, config, cluster)
	if err != nil {
		return nil, err
	}

	// check if we should use an existing ResourceGroup or create a new one
	if config.ResourceGroup != nil {
		createResourceGroup = false
		resourceGroupName = config.ResourceGroup.Name
	}

	// VNet settings.
	if config.Networks.VNet.Name != nil && config.Networks.VNet.ResourceGroup != nil {
		// Deploy in existing vNet.
		createVNet = false
		vnetConfig["name"] = *config.Networks.VNet.Name
		vnetConfig["resourceGroup"] = *config.Networks.VNet.ResourceGroup
		outputKeys["vnetResourceGroup"] = TerraformerOutputKeyVNetResourceGroup
	} else if config.Networks.VNet.CIDR != nil {
		// Apply a custom cidr for the vNet.
		vnetConfig["cidr"] = *config.Networks.VNet.CIDR
	} else if config.Networks.Workers != nil {
		// Use worker cidr as default for the vNet.
		vnetConfig["cidr"] = *config.Networks.Workers
	} else {
		return nil, fmt.Errorf("no VNet or workers configuration provided")
	}

	if primaryAvSetRequired {
		createAvailabilitySet = true
		outputKeys["availabilitySetID"] = TerraformerOutputKeyAvailabilitySetID
		outputKeys["availabilitySetName"] = TerraformerOutputKeyAvailabilitySetName
		outputKeys["countFaultDomains"] = TerraformerOutputKeyCountFaultDomains
		outputKeys["countUpdateDomains"] = TerraformerOutputKeyCountUpdateDomains

		count, err := findDomainCounts(cluster, infra)
		if err != nil {
			return nil, err
		}

		azureConfig["countFaultDomains"] = count.faultDomains
		azureConfig["countUpdateDomains"] = count.updateDomains
	}

	if config.Identity != nil && config.Identity.Name != "" && config.Identity.ResourceGroup != "" {
		identityConfig = map[string]interface{}{
			"name":          config.Identity.Name,
			"resourceGroup": config.Identity.ResourceGroup,
		}
		outputKeys["identityID"] = TerraformerOutputKeyIdentityID
		outputKeys["identityClientID"] = TerraformerOutputKeyIdentityClientID
	}

	networkConfig, err := computeNetworkConfig(config)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"azure": azureConfig,
		"create": map[string]interface{}{
			"resourceGroup":   createResourceGroup,
			"vnet":            createVNet,
			"availabilitySet": createAvailabilitySet,
		},
		"resourceGroup": map[string]interface{}{
			"name": resourceGroupName,
			"vnet": vnetConfig,
		},
		"clusterName": infra.Namespace,
		"networks":    networkConfig,
		"identity":    identityConfig,
		"outputKeys":  outputKeys,
	}
	return result, nil
}

func computeNetworkConfig(config *api.InfrastructureConfig) (map[string]interface{}, error) {
	var (
		networkCfg = make(map[string]interface{})
		subnets    []map[string]interface{}
	)
	if config.Networks.Workers != nil {
		subnet := map[string]interface{}{
			"cidr":             *config.Networks.Workers,
			"serviceEndpoints": config.Networks.ServiceEndpoints,
			"natGateway":       generateNatGatewayValues(config.Networks.NatGateway),
		}
		subnets = append(subnets, subnet)
	} else {
		for _, zone := range config.Networks.Zones {
			natGateway := generateNatGatewayValues(zone.NatGateway)
			zoneConfig := map[string]interface{}{
				"cidr":             zone.CIDR,
				"serviceEndpoints": zone.ServiceEndpoints,
				"natGateway":       natGateway,
			}
			subnets = append(subnets, zoneConfig)
		}
	}

	networkCfg["subnets"] = subnets
	return networkCfg, nil
}

func generateNatGatewayValues(nat *api.NatGatewayConfig) map[string]interface{} {
	natGatewayConfig := map[string]interface{}{
		"enabled": false,
	}

	if nat == nil || !nat.Enabled {
		return natGatewayConfig
	}

	natGatewayConfig["enabled"] = true
	if nat.IdleConnectionTimeoutMinutes != nil {
		natGatewayConfig["idleConnectionTimeoutMinutes"] = *nat.IdleConnectionTimeoutMinutes
	}

	if nat.Zone != nil {
		natGatewayConfig["zone"] = *nat.Zone
	}

	if len(nat.IPAddresses) > 0 {
		var ipAddresses = make([]map[string]interface{}, len(nat.IPAddresses))
		for i, ip := range nat.IPAddresses {
			ipAddresses[i] = map[string]interface{}{
				"name":          ip.Name,
				"resourceGroup": ip.ResourceGroup,
			}
		}
		natGatewayConfig["ipAddresses"] = ipAddresses
	}

	return natGatewayConfig
}

// TerraformFiles are the files that have been rendered from the infrastructure chart.
type TerraformFiles struct {
	Main      string
	Variables string
	TFVars    []byte
}

// TerraformState is the Terraform state for an infrastructure.
type TerraformState struct {
	// VPCName is the name of the VNet created for an infrastructure.
	VNetName string
	// VNetResourceGroupName is the name of the resource group where the vnet is deployed to.
	VNetResourceGroupName string
	// ResourceGroupName is the name of the resource group.
	ResourceGroupName string
	// AvailabilitySetID is the ID for the created availability set.
	AvailabilitySetID string
	// CountFaultDomains is the fault domain count for the created availability set.
	CountFaultDomains int
	// CountUpdateDomains is the update domain count for the created availability set.
	CountUpdateDomains int
	// AvailabilitySetName the ID for the created availability set .
	AvailabilitySetName string
	// SubnetName is the name of the created subnet.
	SubnetNames []string
	// RouteTableName is the name of the route table.
	RouteTableName string
	// SecurityGroupName is the name of the security group.
	SecurityGroupName string
	// IdentityID is the id of the identity.
	IdentityID string
	// IdentityClientID is the client id of the identity.
	IdentityClientID string
}

// ExtractTerraformState extracts the TerraformState from the given Terraformer.
func ExtractTerraformState(ctx context.Context, tf terraformer.Terraformer, infra *extensionsv1alpha1.Infrastructure, config *api.InfrastructureConfig, cluster *controller.Cluster) (*TerraformState, error) {
	var (
		outputKeys = []string{
			TerraformerOutputKeyResourceGroupName,
			TerraformerOutputKeyRouteTableName,
			TerraformerOutputKeySecurityGroupName,
			TerraformerOutputKeyVNetName,
		}
	)
	var subnetOutputKeys []string
	subnetOutputKeys = append(subnetOutputKeys, TerraformerOutputKeySubnetName)
	if len(config.Networks.Zones) > 0 {
		for i := range config.Networks.Zones[1:] {
			key := fmt.Sprintf("%s%d", TerraformerOutputKeySubnetNamePrefix, i+1)
			subnetOutputKeys = append(subnetOutputKeys, key)
		}
	}
	outputKeys = append(outputKeys, subnetOutputKeys...)

	primaryAvSetRequired, err := isPrimaryAvailabilitySetRequired(infra, config, cluster)
	if err != nil {
		return nil, err
	}

	if config.Networks.VNet.Name != nil && config.Networks.VNet.ResourceGroup != nil {
		outputKeys = append(outputKeys, TerraformerOutputKeyVNetResourceGroup)
	}

	if primaryAvSetRequired {
		outputKeys = append(outputKeys, TerraformerOutputKeyAvailabilitySetID, TerraformerOutputKeyAvailabilitySetName, TerraformerOutputKeyCountFaultDomains, TerraformerOutputKeyCountUpdateDomains)
	}

	if config.Identity != nil && config.Identity.Name != "" && config.Identity.ResourceGroup != "" {
		outputKeys = append(outputKeys, TerraformerOutputKeyIdentityID, TerraformerOutputKeyIdentityClientID)
	}

	vars, err := tf.GetStateOutputVariables(ctx, outputKeys...)
	if err != nil {
		return nil, err
	}

	var tfState = TerraformState{
		VNetName:          vars[TerraformerOutputKeyVNetName],
		ResourceGroupName: vars[TerraformerOutputKeyResourceGroupName],
		RouteTableName:    vars[TerraformerOutputKeyRouteTableName],
		SecurityGroupName: vars[TerraformerOutputKeySecurityGroupName],
	}

	if config.Networks.VNet.Name != nil && config.Networks.VNet.ResourceGroup != nil {
		tfState.VNetResourceGroupName = vars[TerraformerOutputKeyVNetResourceGroup]
	}

	if primaryAvSetRequired {
		tfState.AvailabilitySetID = vars[TerraformerOutputKeyAvailabilitySetID]
		tfState.AvailabilitySetName = vars[TerraformerOutputKeyAvailabilitySetName]
		countFaultDomains, err := strconv.Atoi(vars[TerraformerOutputKeyCountFaultDomains])
		if err != nil {
			return nil, fmt.Errorf("error while parsing countFaultDomain from state: %v", err)
		}
		tfState.CountFaultDomains = countFaultDomains
		countUpdateDomains, err := strconv.Atoi(vars[TerraformerOutputKeyCountUpdateDomains])
		if err != nil {
			return nil, fmt.Errorf("error while parsing countUpdateDomain from state: %v", err)
		}
		tfState.CountUpdateDomains = countUpdateDomains
	}

	if config.Identity != nil && config.Identity.Name != "" && config.Identity.ResourceGroup != "" {
		tfState.IdentityID = vars[TerraformerOutputKeyIdentityID]
		tfState.IdentityClientID = vars[TerraformerOutputKeyIdentityClientID]
	}

	for _, key := range subnetOutputKeys {
		tfState.SubnetNames = append(tfState.SubnetNames, vars[key])
	}

	return &tfState, nil
}

// StatusFromTerraformState computes an InfrastructureStatus from the given
// Terraform variables.
func StatusFromTerraformState(config *api.InfrastructureConfig, tfState *TerraformState) *apiv1alpha1.InfrastructureStatus {
	var infraState = apiv1alpha1.InfrastructureStatus{
		TypeMeta: StatusTypeMeta,
		ResourceGroup: apiv1alpha1.ResourceGroup{
			Name: tfState.ResourceGroupName,
		},
		Networks: apiv1alpha1.NetworkStatus{
			VNet: apiv1alpha1.VNetStatus{
				Name: tfState.VNetName,
			},
		},
		AvailabilitySets: []apiv1alpha1.AvailabilitySet{},
		RouteTables: []apiv1alpha1.RouteTable{
			{Purpose: apiv1alpha1.PurposeNodes, Name: tfState.RouteTableName},
		},
		SecurityGroups: []apiv1alpha1.SecurityGroup{
			{Name: tfState.SecurityGroupName, Purpose: apiv1alpha1.PurposeNodes},
		},
		Zoned: false,
	}

	if config.Zoned {
		infraState.Zoned = true
	}

	if config.Networks.Workers != nil {
		if config.Zoned {
			infraState.Networks.Topology = apiv1alpha1.TopologyZonalSingleSubnet
		} else {
			infraState.Networks.Topology = apiv1alpha1.TopologyRegional
		}
	} else {
		infraState.Networks.Topology = apiv1alpha1.TopologyZonal
	}

	switch infraState.Networks.Topology {
	case apiv1alpha1.TopologyZonal:
		for i, subnet := range tfState.SubnetNames {
			zoneStr := fmt.Sprintf("%d", config.Networks.Zones[i].Name)
			infraState.Networks.Subnets = append(infraState.Networks.Subnets, apiv1alpha1.Subnet{
				Name:    subnet,
				Purpose: apiv1alpha1.PurposeNodes,
				Zone:    &zoneStr,
			})
		}
	default:
		for _, subnet := range tfState.SubnetNames {
			infraState.Networks.Subnets = append(infraState.Networks.Subnets, apiv1alpha1.Subnet{
				Name:    subnet,
				Purpose: apiv1alpha1.PurposeNodes,
			})
		}
	}

	if tfState.VNetResourceGroupName != "" {
		infraState.Networks.VNet.ResourceGroup = &tfState.VNetResourceGroupName
	}

	if tfState.IdentityID != "" && tfState.IdentityClientID != "" {
		infraState.Identity = &apiv1alpha1.IdentityStatus{
			ID:       tfState.IdentityID,
			ClientID: tfState.IdentityClientID,
		}
	}

	// Add AvailabilitySet to the infrastructure tfState if an AvailabilitySet is part of the Terraform tfState.
	if tfState.AvailabilitySetID != "" && tfState.AvailabilitySetName != "" {
		infraState.AvailabilitySets = append(infraState.AvailabilitySets, apiv1alpha1.AvailabilitySet{
			Name:               tfState.AvailabilitySetName,
			ID:                 tfState.AvailabilitySetID,
			CountFaultDomains:  pointer.Int32Ptr(int32(tfState.CountFaultDomains)),
			CountUpdateDomains: pointer.Int32Ptr(int32(tfState.CountUpdateDomains)),
			Purpose:            apiv1alpha1.PurposeNodes,
		})
	}

	// Since v1.21.0 requires upgrading at least to a version >=v1.15.0, we can assume that the NATGateway public IP
	// migration has been completed. Therefore, always set NatGatewayPublicIPMigrated to true.
	infraState.NatGatewayPublicIPMigrated = true

	return &infraState
}

// ComputeStatus computes the status based on the Terraformer and the given InfrastructureConfig.
func ComputeStatus(ctx context.Context, tf terraformer.Terraformer, infra *extensionsv1alpha1.Infrastructure, config *api.InfrastructureConfig, cluster *controller.Cluster) (*apiv1alpha1.InfrastructureStatus, error) {
	state, err := ExtractTerraformState(ctx, tf, infra, config, cluster)
	if err != nil {
		return nil, err
	}
	status := StatusFromTerraformState(config, state)

	// Check if ACR access should be configured.
	if config.Identity != nil && config.Identity.ACRAccess != nil && *config.Identity.ACRAccess && status.Identity != nil {
		status.Identity.ACRAccess = true
	}

	return status, nil
}

type domainCounts struct {
	faultDomains  int32
	updateDomains int32
}

func findDomainCounts(cluster *controller.Cluster, infra *extensionsv1alpha1.Infrastructure) (*domainCounts, error) {
	var (
		faultDomainCount  *int32
		updateDomainCount *int32
	)

	if infra.Status.ProviderStatus != nil {
		infrastructureStatus, err := helper.InfrastructureStatusFromInfrastructure(infra)
		if err != nil {
			return nil, fmt.Errorf("error obtaining update and fault domain counts from infrastructure status: %v", err)
		}
		nodesAvailabilitySet, err := helper.FindAvailabilitySetByPurpose(infrastructureStatus.AvailabilitySets, api.PurposeNodes)
		if err != nil {
			return nil, fmt.Errorf("error obtaining update and fault domain counts from infrastructure status: %v", err)
		}

		// Take values from the availability set status.
		// Domain counts can still be nil, esp. if the status was written by an earlier version of this provider extension.
		if nodesAvailabilitySet != nil {
			faultDomainCount = nodesAvailabilitySet.CountFaultDomains
			updateDomainCount = nodesAvailabilitySet.CountUpdateDomains
		}
	}

	cloudProfileConfig, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return nil, err
	}

	if faultDomainCount == nil {
		count, err := helper.FindDomainCountByRegion(cloudProfileConfig.CountFaultDomains, infra.Spec.Region)
		if err != nil {
			return nil, err
		}
		faultDomainCount = &count
	}

	if updateDomainCount == nil {
		count, err := helper.FindDomainCountByRegion(cloudProfileConfig.CountUpdateDomains, infra.Spec.Region)
		if err != nil {
			return nil, err
		}
		updateDomainCount = &count
	}

	return &domainCounts{
		faultDomains:  *faultDomainCount,
		updateDomains: *updateDomainCount,
	}, nil
}

// isPrimaryAvailabilitySetRequired determines if a cluster primary AvailabilitySet is required.
func isPrimaryAvailabilitySetRequired(infra *extensionsv1alpha1.Infrastructure, config *api.InfrastructureConfig, cluster *controller.Cluster) (bool, error) {
	if config.Zoned {
		return false, nil
	}
	if cluster.Shoot == nil {
		return false, errors.New("cannot determine if primary availability set is required as cluster.Shoot is not set")
	}

	hasVmoAnnotation := helper.HasShootVmoAlphaAnnotation(cluster.Shoot.Annotations)

	// If the infrastructureStatus is not exists that mean it is a new Infrastucture.
	if infra.Status.ProviderStatus == nil {
		if hasVmoAnnotation {
			return false, nil
		}
		return true, nil
	}

	// If the infrastructureStatus already exists that mean the Infrastucture is already created.
	infrastructureStatus, err := helper.InfrastructureStatusFromInfrastructure(infra)
	if err != nil {
		return false, err
	}

	if len(infrastructureStatus.AvailabilitySets) > 0 {
		if _, err := helper.FindAvailabilitySetByPurpose(infrastructureStatus.AvailabilitySets, api.PurposeNodes); err == nil {
			if hasVmoAnnotation {
				return false, errors.New("cannot use vmss orchestration mode VM (VMO) as this cluster already used an availability set")
			}
			return true, nil
		}
	}

	return false, nil
}
