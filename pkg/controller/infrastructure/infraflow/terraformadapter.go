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

// values for TerraformAdapter resources
const TfResourceGroup = "resourceGroup"
const TfVnet = "vnet"
const TfAvailabilitySet = "availabilitySet"

type TerraformAdapter struct {
	values map[string]interface{}
}

func (t TerraformAdapter) RouteTableName() string {
	return "worker_route_table"
}

func (t TerraformAdapter) SecurityGroupName() string {
	return t.ClusterName() + "-workers"
}

type identityTf struct {
	Name          string
	ResourceGroup string
}

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

// TODO verify correctness
func (t TerraformAdapter) InfrastructureStatus(config *azure.InfrastructureConfig) *v1alpha1.InfrastructureStatus {
	infraState := v1alpha1.InfrastructureStatus{
		TypeMeta: infrastructure.StatusTypeMeta,
		ResourceGroup: v1alpha1.ResourceGroup{
			Name: t.ResourceGroup(),
		},
		Networks: v1alpha1.NetworkStatus{
			VNet: v1alpha1.VNetStatus{
				Name: t.Vnet().Name(),
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

	if config.Zoned {
		infraState.Zoned = true
	}

	if config.Networks.Workers == nil {
		infraState.Networks.Layout = v1alpha1.NetworkLayoutMultipleSubnet
	} else {
		infraState.Networks.Layout = v1alpha1.NetworkLayoutSingleSubnet
	}

	for _, subnet := range t.Nats() {
		//if
		infraState.Networks.Subnets = append(infraState.Networks.Subnets, v1alpha1.Subnet{
			Name:    subnet.SubnetName(),
			Purpose: v1alpha1.PurposeNodes,
			Zone:    subnet.Zone(),
			//Migrated: *subnet.migrated,
		})
	}

	infraState.Networks.VNet.ResourceGroup = t.Vnet().ResourceGroup()

	//if tfState.IdentityID != "" && tfState.IdentityClientID != "" {
	//	infraState.Identity = &v1alpha1.IdentityStatus{
	//		ID:       tfState.IdentityID,
	//		ClientID: tfState.IdentityClientID,
	//	}
	//}

	// Add AvailabilitySet to the infrastructure tfState if an AvailabilitySet is part of the Terraform tfState.

	//if tfState.AvailabilitySetID != "" && tfState.AvailabilitySetName != "" {
	//	infraState.AvailabilitySets = append(infraState.AvailabilitySets, v1alpha1.AvailabilitySet{
	//		Name:               tfState.AvailabilitySetName,
	//		ID:                 tfState.AvailabilitySetID,
	//		CountFaultDomains:  pointer.Int32Ptr(int32(tfState.CountFaultDomains)),
	//		CountUpdateDomains: pointer.Int32Ptr(int32(tfState.CountUpdateDomains)),
	//		Purpose:            v1alpha1.PurposeNodes,
	//	})
	//}

	return &infraState
}

func NewTerraformAdapter(infra *extensionsv1alpha1.Infrastructure, config *azure.InfrastructureConfig, cluster *controller.Cluster) (TerraformAdapter, error) {
	tfValues, err := infrastructure.ComputeTerraformerTemplateValues(infra, config, cluster) // use for migration of values..
	return TerraformAdapter{tfValues}, err
}

// TODO not needed due to Create/Update?
func (t TerraformAdapter) isCreate(resource string) bool {
	create := t.values["create"].(map[string]interface{})
	return create[resource].(bool)
}

func (t TerraformAdapter) Vnet() vnetTf {
	cm := t.values["resourceGroup"].(map[string]interface{})
	return vnetTf(cm["vnet"].(map[string]interface{}))
}

func (t TerraformAdapter) ResourceGroup() string {
	return t.values["resourceGroup"].(map[string]interface{})["name"].(string)
}

func (t TerraformAdapter) Region() string {
	return t.values["azure"].(map[string]interface{})["region"].(string)
}

func (t TerraformAdapter) CountUpdateDomains() int32 {
	return t.values["azure"].(map[string]interface{})["countUpdateDomains"].(int32)
}

func (t TerraformAdapter) CountFaultDomains() int32 {
	return t.values["azure"].(map[string]interface{})["countFaultDomains"].(int32)
}

func (t TerraformAdapter) ClusterName() string {
	return t.values["clusterName"].(string)
}

func (t TerraformAdapter) Subnets() []subnetTf {
	res := make([]subnetTf, 0)
	rawSubnets := t.values["networks"].(map[string]interface{})["subnets"]
	if rawSubnets == nil {
		return res
	}
	for _, subnet := range rawSubnets.([]map[string]interface{}) {
		name := t.subnetName(subnet)
		cidr := subnet["cidr"].(string)
		serviceEndpoints := subnet["serviceEndpoints"].([]string)
		res = append(res, subnetTf{name, cidr, serviceEndpoints})
	}
	return res
}

func (t TerraformAdapter) subnetName(subnet map[string]interface{}) string {
	name := t.ClusterName() + "-nodes"
	isMigrated, isMultiSubnet := subnet["migrated"]
	if isMultiSubnet {
		if !isMigrated.(bool) {
			name = fmt.Sprintf("%s-z%d", t.ClusterName(), subnet["name"].(int32))
		}
	}
	return name
}

func (t TerraformAdapter) EnabledNats() []natTf {
	res := make([]natTf, 0)
	for _, nat := range t.Nats() {
		if nat.enabled {
			res = append(res, nat)
		}
	}
	return res
}

func (t TerraformAdapter) AvailabilitySetName() string {
	return t.ClusterName() + "-avset-workers"
}

type userManagedIP struct {
	Name          string
	ResourceGroup string
	SubnetName    string
}

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

func (t TerraformAdapter) Nats() []natTf {
	res := make([]natTf, 0)
	rawSubnets := t.values["networks"].(map[string]interface{})["subnets"]
	if rawSubnets == nil {
		return res
	}
	for _, subnet := range rawSubnets.([]map[string]interface{}) {
		rawNetNumber, ok := subnet["name"].(int32)
		if !ok {
			continue
		}

		natRaw := subnet["natGateway"].(map[string]interface{})

		var idleConnectionTimeoutMinutes *int32 = nil
		if _, ok := natRaw["idleConnectionTimeoutMinutes"]; ok {
			idleConnectionTimeoutMinutes = to.Ptr(natRaw["idleConnectionTimeoutMinutes"].(int32))
		}
		//cidr := subnet["cidr"].(string)
		//serviceEndpoints := subnet["serviceEndpoints"].([]string)
		var isMigrated *bool = nil
		isMigratedRaw, isMultiSubnet := subnet["migrated"]
		if isMultiSubnet {
			isMigrated = to.Ptr(isMigratedRaw.(bool))
		}

		var zone *string = nil
		zoneRaw, ok := natRaw["zone"]
		if ok {
			zone = to.Ptr(fmt.Sprintf("%d", zoneRaw.(int32)))
		}
		res = append(res, natTf{rawNetNumber, natRaw["enabled"].(bool), idleConnectionTimeoutMinutes, isMigrated, t.ClusterName(), zone})
	}
	return res
}

type natTf struct {
	rawNetNumber                 int32
	enabled                      bool
	idleConnectionTimeoutMinutes *int32
	migrated                     *bool
	clusterName                  string
	zone                         *string
}

func (nat natTf) NatName() string {
	name := nat.clusterName + "-nat-gateway"
	if nat.migrated != nil && !*nat.migrated {
		name = fmt.Sprintf("%s-z%d", name, nat.rawNetNumber)
	}
	return name
}

func (nat natTf) SubnetName() string {
	name := nat.clusterName + "-nodes"
	if nat.migrated != nil && !*nat.migrated {
		name = fmt.Sprintf("%s-z%d", name, nat.rawNetNumber)
	}
	return name
}

func (nat natTf) IpName() string {
	return nat.NatName() + "-ip"
}

func (nat natTf) Zone() *string {
	return nat.zone
}

type subnetTf struct {
	name             string
	cidr             string
	serviceEndpoints []string
}

type vnetTf (map[string]interface{})

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
