// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v12"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	azureconsts "github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	azureclientmock "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client/mock"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow/shared"
)

const (
	subnetTestTechnicalName = "shoot--project--a"
	subnetTestSubscription  = "aaaaaaaa-1111-2222-3333-444444444444"
	subnetTestVNetGroup     = "shared-network-rg"
	subnetTestVNetName      = "shared-vnet"
)

type subnetTestLayout struct {
	name         string
	zones        []azure.Zone
	migratedZone string
	desiredNames []string
}

var subnetTestLayouts = []subnetTestLayout{
	{
		name:         "single subnet",
		desiredNames: []string{subnetTestTechnicalName + "-nodes"},
	},
	{
		name: "zonal subnets",
		zones: []azure.Zone{
			{Name: 1, CIDR: "10.250.0.0/24"},
			{Name: 2, CIDR: "10.250.1.0/24"},
		},
		desiredNames: []string{subnetTestTechnicalName + "-nodes-z1", subnetTestTechnicalName + "-nodes-z2"},
	},
	{
		name: "migrated subnet",
		zones: []azure.Zone{
			{Name: 1, CIDR: "10.250.0.0/24"},
			{Name: 2, CIDR: "10.250.1.0/24"},
		},
		migratedZone: "1",
		desiredNames: []string{subnetTestTechnicalName + "-nodes", subnetTestTechnicalName + "-nodes-z2"},
	},
}

var subnetTestOwnNames = []string{
	subnetTestTechnicalName + "-nodes",
	subnetTestTechnicalName + "-nodes-z1",
	subnetTestTechnicalName + "-nodes-z2",
	subnetTestTechnicalName + "-nodes-z3", // Removed from the desired configuration.
}

var subnetTestForeignNames = []string{
	subnetTestTechnicalName + "b-nodes",
	subnetTestTechnicalName + "-other-nodes-z1",
	subnetTestTechnicalName + "-nodes-nodes",
	subnetTestTechnicalName + "-nodes-z1-nodes",
	subnetTestTechnicalName + "-nodes-z1-nodes-z2",
	subnetTestTechnicalName + "-nodes-z1extra",
	subnetTestTechnicalName + "-nodes-z01",
	subnetTestTechnicalName + "-nodes-z",
	"unrelated-subnet",
}

func TestEnsureSubnetsSharedVNet(t *testing.T) {
	for _, layout := range subnetTestLayouts {
		t.Run(layout.name, func(t *testing.T) {
			g := NewWithT(t)
			fctx, subnetClient := newSubnetTestFlowContext(t, layout)
			subnetClient.EXPECT().List(gomock.Any(), subnetTestVNetGroup, subnetTestVNetName).Return(subnetTestCurrentSubnets(), nil)

			desiredNames := sets.New(layout.desiredNames...)
			for _, name := range subnetTestOwnNames {
				if desiredNames.Has(name) {
					subnetClient.EXPECT().CreateOrUpdate(gomock.Any(), subnetTestVNetGroup, subnetTestVNetName, name, gomock.Any()).Return(newTestSubnet(name), nil)
				} else {
					subnetClient.EXPECT().Delete(gomock.Any(), subnetTestVNetGroup, subnetTestVNetName, name).Return(nil)
				}
			}
			// No mutating calls are expected for sibling subnets, including names containing "-nodes".
			g.Expect(fctx.EnsureSubnets(context.Background())).To(Succeed())

			var inventoryNames []string
			for _, resource := range fctx.inventory.ByKind(KindSubnet) {
				inventoryNames = append(inventoryNames, resource.Name)
			}
			g.Expect(inventoryNames).To(ConsistOf(layout.desiredNames))
		})
	}
}

func TestDeleteSubnetsInForeignGroupSharedVNet(t *testing.T) {
	for _, layout := range subnetTestLayouts {
		t.Run(layout.name, func(t *testing.T) {
			g := NewWithT(t)
			fctx, subnetClient := newSubnetTestFlowContext(t, layout)
			subnetClient.EXPECT().List(gomock.Any(), subnetTestVNetGroup, subnetTestVNetName).Return(subnetTestCurrentSubnets(), nil)
			for _, name := range subnetTestOwnNames {
				subnetClient.EXPECT().Delete(gomock.Any(), subnetTestVNetGroup, subnetTestVNetName, name).Return(nil)
			}

			g.Expect(fctx.DeleteSubnetsInForeignGroup(context.Background())).To(Succeed())
		})
	}
}

func newSubnetTestFlowContext(t *testing.T, layout subnetTestLayout) (*FlowContext, *azureclientmock.MockSubnet) {
	t.Helper()
	g := NewWithT(t)
	ctrl := gomock.NewController(t)
	subnetClient := azureclientmock.NewMockSubnet(ctrl)
	factory := azureclientmock.NewMockFactory(ctrl)
	factory.EXPECT().Subnet().Return(subnetClient, nil)

	cfg := &azure.InfrastructureConfig{
		Networks: azure.NetworkConfig{
			VNet: azure.VNet{
				Name:          ptr.To(subnetTestVNetName),
				ResourceGroup: ptr.To(subnetTestVNetGroup),
			},
			Workers: ptr.To("10.250.0.0/24"),
			Zones:   layout.zones,
		},
	}
	infra := &extensionsv1alpha1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{Namespace: subnetTestTechnicalName},
	}
	if layout.migratedZone != "" {
		infra.Annotations = map[string]string{azureconsts.NetworkLayoutZoneMigrationAnnotation: layout.migratedZone}
	}
	cluster := &extensionscontroller.Cluster{Shoot: &gardencorev1beta1.Shoot{}}
	adapter, err := NewInfrastructureAdapter(infra, cfg, nil, nil, cluster)
	g.Expect(err).NotTo(HaveOccurred())
	wb := shared.NewWhiteboard()
	return &FlowContext{
		factory:    factory,
		auth:       &azureclient.ClientAuth{SubscriptionID: subnetTestSubscription},
		adapter:    adapter,
		whiteboard: wb,
		inventory:  NewSimpleInventory(wb),
	}, subnetClient
}

func newTestSubnet(name string) *armnetwork.Subnet {
	return &armnetwork.Subnet{
		ID:         ptr.To(GetIdFromTemplateWithParent(TemplateSubnet, subnetTestSubscription, subnetTestVNetGroup, subnetTestVNetName, name)),
		Name:       ptr.To(name),
		Properties: &armnetwork.SubnetPropertiesFormat{},
	}
}

func subnetTestCurrentSubnets() []*armnetwork.Subnet {
	subnets := []*armnetwork.Subnet{nil, {}, {Name: ptr.To("")}}
	for _, name := range subnetTestOwnNames {
		subnets = append(subnets, newTestSubnet(name))
	}
	for _, name := range subnetTestForeignNames {
		subnets = append(subnets, newTestSubnet(name))
	}
	return subnets
}
