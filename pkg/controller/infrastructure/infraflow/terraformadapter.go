package infraflow

import (
	"fmt"

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
	create := t.values["create"].(map[string]bool)
	return create[resource]
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
		name := t.ClusterName() + "-nodes"
		isMigrated, isMultiSubnet := subnet["migrated"]
		if isMultiSubnet {
			if !isMigrated.(bool) {
				name = fmt.Sprintf("%s-z%s", t.ClusterName(), subnet["name"])
			}
		}
		cidr := subnet["cidr"].(string)
		serviceEndpoints := subnet["serviceEndpoints"].([]string)
		res = append(res, subnetTf{name, cidr, serviceEndpoints})
	}
	return res
}

type subnetTf struct {
	name             string
	cidr             string
	serviceEndpoints []string
}

type vnetTf (map[string]interface{})

//func (v vnetTf) CIDR() *string {
//	return v["cidr"].(*string)
//}
