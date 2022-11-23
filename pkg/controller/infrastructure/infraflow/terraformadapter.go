package infraflow

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
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
func (t TerraformAdapter) Nats() []natTf {
	res := make([]natTf, 0)
	rawSubnets := t.values["networks"].(map[string]interface{})["subnets"]
	if rawSubnets == nil {
		return res
	}
	for _, subnet := range rawSubnets.([]map[string]interface{}) {
		rawNetNumber := subnet["name"].(int32)

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
