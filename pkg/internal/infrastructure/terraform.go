// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow/shared"
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
	useWorkloadIdentity bool,
) (
	*TerraformFiles,
	error,
) {
	values, err := ComputeTerraformerTemplateValues(infra, config, cluster, useWorkloadIdentity)
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
	useWorkloadIdentity bool,
) (
	map[string]interface{},
	error,
) {
	var (
		createResourceGroup   = true
		createAvailabilitySet = false
		resourceGroupName     = infra.Namespace

		identityConfig map[string]interface{}
		azureConfig    = map[string]interface{}{
			"region": infra.Spec.Region,
		}
		outputKeys = map[string]interface{}{
			"resourceGroupName": TerraformerOutputKeyResourceGroupName,
			"vnetName":          TerraformerOutputKeyVNetName,
			"subnetName":        TerraformerOutputKeySubnetName,
			"subnetNamePrefix":  TerraformerOutputKeySubnetNamePrefix,
			"routeTableName":    TerraformerOutputKeyRouteTableName,
			"securityGroupName": TerraformerOutputKeySecurityGroupName,
		}
	)

	primaryAvSetRequired, err := IsPrimaryAvailabilitySetRequired(infra, config, cluster)
	if err != nil {
		return nil, err
	}

	// check if we should use an existing ResourceGroupName or create a new one
	if config.ResourceGroup != nil {
		createResourceGroup = false
		resourceGroupName = config.ResourceGroup.Name
	}

	createVNet, vnetConfig, additionalOutpuKeys, err := generateVNetConfig(&config.Networks, infra.Namespace)
	if err != nil {
		return nil, err
	}
	for k, v := range additionalOutpuKeys {
		outputKeys[k] = v
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

	var networkConfig map[string]interface{}
	if helper.IsUsingSingleSubnetLayout(config) {
		networkConfig, err = computeNetworkConfigSingleSubnetLayout(config)
	} else {
		networkConfig, err = computeNetworkConfigMultipleSubnetLayout(infra, config)
	}
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
		"clusterName":         infra.Namespace,
		"networks":            networkConfig,
		"identity":            identityConfig,
		"outputKeys":          outputKeys,
		"useWorkloadIdentity": useWorkloadIdentity,
	}
	return result, nil
}

func generateVNetConfig(n *api.NetworkConfig, defaultVnetName string) (bool, map[string]interface{}, map[string]interface{}, error) {
	var (
		createVNet = true
		vnetConfig = map[string]interface{}{
			"name": defaultVnetName,
		}
		outputKeys = map[string]interface{}{}
		err        error
	)

	switch {
	case n.VNet.Name != nil && n.VNet.ResourceGroup != nil:
		createVNet = false
		vnetConfig["name"] = *n.VNet.Name
		vnetConfig["resourceGroup"] = *n.VNet.ResourceGroup
		outputKeys["vnetResourceGroup"] = TerraformerOutputKeyVNetResourceGroup
	case n.VNet.CIDR != nil:
		vnetConfig["cidr"] = *n.VNet.CIDR
	case n.Workers != nil:
		vnetConfig["cidr"] = *n.Workers
	default:
		return false, nil, nil, fmt.Errorf("no VNet or workers configuration provided")
	}

	if createVNet && n.VNet.DDosProtectionPlanID != nil {
		vnetConfig["ddosProtectionPlanID"] = *n.VNet.DDosProtectionPlanID
	}

	return createVNet, vnetConfig, outputKeys, err
}

func computeNetworkConfigSingleSubnetLayout(config *api.InfrastructureConfig) (map[string]interface{}, error) {
	var (
		networkCfg = make(map[string]interface{})
		subnets    []map[string]interface{}
	)
	natGatewayConfig, err := generateNatGatewayValues(config.Networks.NatGateway)
	if err != nil {
		return nil, err
	}

	subnet := map[string]interface{}{
		"cidr":             *config.Networks.Workers,
		"serviceEndpoints": config.Networks.ServiceEndpoints,
		"natGateway":       natGatewayConfig,
	}

	subnets = append(subnets, subnet)
	networkCfg["subnets"] = subnets
	return networkCfg, nil
}

func computeNetworkConfigMultipleSubnetLayout(infra *extensionsv1alpha1.Infrastructure, config *api.InfrastructureConfig) (map[string]interface{}, error) {
	var (
		networkCfg = make(map[string]interface{})
		subnets    []map[string]interface{}
	)

	for _, zone := range config.Networks.Zones {
		migratedZone, ok := infra.Annotations[azure.NetworkLayoutZoneMigrationAnnotation]
		subnetConfig := map[string]interface{}{
			"name":             zone.Name,
			"cidr":             zone.CIDR,
			"serviceEndpoints": zone.ServiceEndpoints,
			"natGateway":       generateZonedNatGatewayValues(zone.NatGateway, zone.Name),
			"migrated":         ok && migratedZone == helper.InfrastructureZoneToString(zone.Name),
		}
		subnets = append(subnets, subnetConfig)
	}

	networkCfg["subnets"] = subnets
	return networkCfg, nil
}

func generateNatGatewayValues(nat *api.NatGatewayConfig) (map[string]interface{}, error) {
	natGatewayConfig := map[string]interface{}{
		"enabled": false,
	}

	if nat == nil || !nat.Enabled {
		return natGatewayConfig, nil
	}

	natGatewayConfig["enabled"] = true
	if nat.IdleConnectionTimeoutMinutes != nil {
		natGatewayConfig["idleConnectionTimeoutMinutes"] = *nat.IdleConnectionTimeoutMinutes
	}

	if nat.Zone != nil {
		natGatewayConfig["zone"] = *nat.Zone
	}

	if len(nat.IPAddresses) > 0 {
		ipAddresses := make([]map[string]interface{}, len(nat.IPAddresses))
		for i, ip := range nat.IPAddresses {
			ipAddresses[i] = map[string]interface{}{
				"name":          ip.Name,
				"resourceGroup": ip.ResourceGroup,
			}
		}
		natGatewayConfig["ipAddresses"] = ipAddresses
	}
	return natGatewayConfig, nil
}

func generateZonedNatGatewayValues(nat *api.ZonedNatGatewayConfig, zone int32) map[string]interface{} {
	natGatewayConfig := map[string]interface{}{
		"enabled": false,
	}

	if nat == nil || !nat.Enabled {
		return natGatewayConfig
	}

	natGatewayConfig["enabled"] = true
	natGatewayConfig["zone"] = zone
	if nat.IdleConnectionTimeoutMinutes != nil {
		natGatewayConfig["idleConnectionTimeoutMinutes"] = *nat.IdleConnectionTimeoutMinutes
	}

	if len(nat.IPAddresses) > 0 {
		ipAddresses := make([]map[string]interface{}, len(nat.IPAddresses))
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
	// Subnets contain information to identify the created subnets.
	Subnets []terraformSubnet
	// RouteTableName is the name of the route table.
	RouteTableName string
	// SecuritGroupName is the name of the security group.
	SecurityGroupName string
	// IdentityID is the id of the identity.
	IdentityID string
	// IdentityClientID is the client id of the identity.
	IdentityClientID string
}

type terraformSubnet struct {
	name     string
	zone     *string
	migrated bool
}

// ExtractTerraformState extracts the TerraformState from the given Terraformer.
func ExtractTerraformState(ctx context.Context, tf terraformer.Terraformer, infra *extensionsv1alpha1.Infrastructure, config *api.InfrastructureConfig, cluster *controller.Cluster) (*TerraformState, error) {
	outputKeys := []string{
		TerraformerOutputKeyResourceGroupName,
		TerraformerOutputKeyRouteTableName,
		TerraformerOutputKeySecurityGroupName,
		TerraformerOutputKeyVNetName,
	}

	outputKeys = append(outputKeys, computeSubnetOutputKeys(infra, config)...)
	primaryAvSetRequired, err := IsPrimaryAvailabilitySetRequired(infra, config, cluster)
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

	tfState := TerraformState{
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

	tfState.Subnets = computeInfrastructureSubnets(infra, vars)
	return &tfState, nil
}

// EgressCidrs retrieves the Egress CIDRs from the Terraform state and returns them.
func EgressCidrs(terraformState *terraformer.RawState) ([]string, error) {
	tfState, err := shared.UnmarshalTerraformStateFromTerraformer(terraformState)
	if err != nil {
		return nil, err
	}
	resources := tfState.FindManagedResourcesByType("azurerm_public_ip")

	egressCidrs := []string{}
	for _, resource := range resources {
		for _, instance := range resource.Instances {
			rawIpAddress := instance.Attributes["ip_address"]
			ipAddress, ok := rawIpAddress.(string)
			if !ok {
				return nil, fmt.Errorf("error parsing '%v' as IP-address from Terraform state", rawIpAddress)
			}
			egressCidrs = append(egressCidrs, ipAddress+"/32")
		}
	}
	return egressCidrs, nil
}

// StatusFromTerraformState computes an InfrastructureStatus from the given
// Terraform variables.
func StatusFromTerraformState(config *api.InfrastructureConfig, tfState *TerraformState) *apiv1alpha1.InfrastructureStatus {
	infraState := apiv1alpha1.InfrastructureStatus{
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

	if config.Networks.Workers == nil {
		infraState.Networks.Layout = apiv1alpha1.NetworkLayoutMultipleSubnet
	} else {
		infraState.Networks.Layout = apiv1alpha1.NetworkLayoutSingleSubnet
	}

	for _, subnet := range tfState.Subnets {
		infraState.Networks.Subnets = append(infraState.Networks.Subnets, apiv1alpha1.Subnet{
			Name:     subnet.name,
			Purpose:  apiv1alpha1.PurposeNodes,
			Zone:     subnet.zone,
			Migrated: subnet.migrated,
		})
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
			CountFaultDomains:  ptr.To(int32(tfState.CountFaultDomains)),  // #nosec: G115 - There's a validation for < 0 (overflow) in place.
			CountUpdateDomains: ptr.To(int32(tfState.CountUpdateDomains)), // #nosec: G115 - There's a validation for < 0 (overflow) in place.
			Purpose:            apiv1alpha1.PurposeNodes,
		})
	}

	return &infraState
}

// ComputeTerraformStatus computes the status based on the Terraformer and the given InfrastructureConfig.
func ComputeTerraformStatus(ctx context.Context, tf terraformer.Terraformer, infra *extensionsv1alpha1.Infrastructure, config *api.InfrastructureConfig, cluster *controller.Cluster) (*apiv1alpha1.InfrastructureStatus, error) {
	state, err := ExtractTerraformState(ctx, tf, infra, config, cluster)
	if err != nil {
		return nil, err
	}
	status := StatusFromTerraformState(config, state)

	// Check if ACR access should be configured.
	if config.Identity != nil && config.Identity.ACRAccess != nil && *config.Identity.ACRAccess && status.Identity != nil {
		status.Identity.ACRAccess = true
	}

	status.Networks.OutboundAccessType = apiv1alpha1.OutboundAccessTypeNatGateway
	if len(config.Networks.Zones) > 0 {
		for _, z := range config.Networks.Zones {
			if z.NatGateway == nil || !z.NatGateway.Enabled {
				status.Networks.OutboundAccessType = apiv1alpha1.OutboundAccessTypeLoadBalancer
			}
		}
	} else if config.Networks.NatGateway == nil || !config.Networks.NatGateway.Enabled {
		status.Networks.OutboundAccessType = apiv1alpha1.OutboundAccessTypeLoadBalancer
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
		infrastructureStatus, err := helper.InfrastructureStatusFromRaw(infra.Status.ProviderStatus)
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

// IsPrimaryAvailabilitySetRequired determines if a cluster primary AvailabilitySet is required.
func IsPrimaryAvailabilitySetRequired(infra *extensionsv1alpha1.Infrastructure, config *api.InfrastructureConfig, cluster *controller.Cluster) (bool, error) {
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
	infrastructureStatus, err := helper.InfrastructureStatusFromRaw(infra.Status.ProviderStatus)
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

func computeSubnetOutputKeys(infra *extensionsv1alpha1.Infrastructure, config *api.InfrastructureConfig) []string {
	var subnetOutputKeys []string

	if helper.IsUsingSingleSubnetLayout(config) {
		subnetOutputKeys = append(subnetOutputKeys, TerraformerOutputKeySubnetName)
		return subnetOutputKeys
	}

	migratedZone, ok := infra.Annotations[azure.NetworkLayoutZoneMigrationAnnotation]
	for _, z := range config.Networks.Zones {
		outputKey := fmt.Sprintf("%s%d", TerraformerOutputKeySubnetNamePrefix, z.Name)
		if ok && helper.InfrastructureZoneToString(z.Name) == migratedZone {
			outputKey = TerraformerOutputKeySubnetName
		}
		subnetOutputKeys = append(subnetOutputKeys, outputKey)
	}

	return subnetOutputKeys
}

func computeInfrastructureSubnets(infra *extensionsv1alpha1.Infrastructure, vars map[string]string) []terraformSubnet {
	result := []terraformSubnet{}

	for key, value := range vars {
		switch {
		case strings.HasPrefix(key, TerraformerOutputKeySubnetNamePrefix):
			result = append(result, terraformSubnet{
				name:     value,
				zone:     ptr.To(strings.TrimPrefix(key, TerraformerOutputKeySubnetNamePrefix)),
				migrated: false,
			})
		case key == TerraformerOutputKeySubnetName:
			subnet := terraformSubnet{
				name: vars[key],
			}
			if migratedZone, ok := infra.Annotations[azure.NetworkLayoutZoneMigrationAnnotation]; ok {
				subnet.migrated = ok
				subnet.zone = &migratedZone
			}
			result = append(result, subnet)
		default:
			continue
		}
	}
	return result
}
