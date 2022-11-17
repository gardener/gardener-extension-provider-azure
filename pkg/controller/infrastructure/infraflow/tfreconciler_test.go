package infraflow_test

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	mockclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client/mock"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// will also work for new Reonciler
var _ = Describe("TfReconciler", func() {
	location := "westeurope"
	clusterName := "test_cluster"
	infra := &v1alpha1.Infrastructure{Spec: v1alpha1.InfrastructureSpec{Region: location}, ObjectMeta: metav1.ObjectMeta{Namespace: clusterName}}
	resourceGroupName := infra.Namespace //if not specified this is assumed name "t-i545428" // TODO what if resource group not given? by default Tf uses infra.Namespace
	vnetName := infra.Namespace          //if not specified this is assumed name "vnet-i545428"
	cluster := infrastructure.MakeCluster("11.0.0.0/16", "12.0.0.0/16", infra.Spec.Region, 1, 1)
	cfg := &azure.InfrastructureConfig{
		//ResourceGroup: &azure.ResourceGroup{Name: resourceGroupName},
		Networks: azure.NetworkConfig{
			VNet: azure.VNet{
				//Name:          to.Ptr(vnetName), // only specify when using existing group
				//ResourceGroup: to.Ptr(resourceGroupName),
				CIDR: to.Ptr("10.0.0.0/8"),
			},
			Workers:          to.Ptr("10.0.0.0/16"),
			ServiceEndpoints: []string{},
			/// TODO how to specify multi subnet.. resource group not needed?
			//Zones:            []azure.Zone{{Name: 1, CIDR: "10.0.0.0/16", NatGateway: &azure.ZonedNatGatewayConfig{Enabled: true, IPAddresses: []azure.ZonedPublicIPReference{{Name: "my-ip", ResourceGroup: resourceGroupName}}}}, {Name: 2, CIDR: "10.1.0.0/16"}}, // subnets
		},
	}
	parameters := armnetwork.VirtualNetwork{
		Location: to.Ptr(location),
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{
				AddressPrefixes: []*string{cfg.Networks.VNet.CIDR},
			},
		},
	}

	Context("reconcile new vnet with ddosId", func() {
		ddosId := "ddos-plan-id"
		cfg.Networks.VNet.DDosProtectionPlanID = to.Ptr(ddosId)
		var vnet *mockclient.MockVnet
		BeforeEach(func() {
			ctrl := gomock.NewController(GinkgoT())
			vnet = mockclient.NewMockVnet(ctrl)

			parameters.Properties.EnableDdosProtection = to.Ptr(true)
			parameters.Properties.DdosProtectionPlan = &armnetwork.SubResource{ID: to.Ptr(ddosId)}
			vnet.EXPECT().CreateOrUpdate(gomock.Any(), resourceGroupName, vnetName, parameters).Return(nil)
		})
		It("calls the client with the correct parameters: vnet name, resource group, region ,cidr, ddos id", func() {
			sut, err := infraflow.NewTfReconciler(infra, cfg, cluster)
			Expect(err).ToNot(HaveOccurred())
			sut.Vnet(context.TODO(), vnet)
		})
	})
	Context("reconcile new vnet without ddosId", func() {
		var vnet *mockclient.MockVnet
		BeforeEach(func() {
			ctrl := gomock.NewController(GinkgoT())
			vnet = mockclient.NewMockVnet(ctrl)
			parameters.Properties.AddressSpace.AddressPrefixes = []*string{cfg.Networks.VNet.CIDR}

			vnet.EXPECT().CreateOrUpdate(gomock.Any(), resourceGroupName, vnetName, parameters).Return(nil)
		})
		It("calls the client with the correct parameters: vnet name, resource group, region ,cidr, ddos id", func() {
			sut, err := infraflow.NewTfReconciler(infra, cfg, cluster)
			Expect(err).ToNot(HaveOccurred())
			sut.Vnet(context.TODO(), vnet)
		})
	})
})

type MatchParameters (armnetwork.VirtualNetwork)

func (m MatchParameters) Matches(x interface{}) bool {
	bytes, _ := armnetwork.VirtualNetwork(m).MarshalJSON()
	Otherbytes, _ := x.(armnetwork.VirtualNetwork).MarshalJSON()
	println(string(bytes))
	println(string(Otherbytes))
	return string(bytes) == string(Otherbytes)
}

func (m MatchParameters) String() string {
	bytes, _ := armnetwork.VirtualNetwork(m).MarshalJSON()
	return string(bytes)
}
