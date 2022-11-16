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
	Context("reconcile vnet with ddosId", func() {
		resourceGroupName := "t-i545428" // TODO what if resource group not given? by default Tf uses infra.Namespace
		location := "westeurope"
		vnetName := "vnet-i545428"
		clusterName := "test_cluster"
		ddosId := "ddos-plan-id"
		infra := &v1alpha1.Infrastructure{Spec: v1alpha1.InfrastructureSpec{Region: location}, ObjectMeta: metav1.ObjectMeta{Namespace: clusterName}}
		cfg := &azure.InfrastructureConfig{
			ResourceGroup: &azure.ResourceGroup{Name: resourceGroupName},
			Networks: azure.NetworkConfig{
				VNet: azure.VNet{
					Name:                 to.Ptr(vnetName),
					ResourceGroup:        to.Ptr(resourceGroupName),
					CIDR:                 to.Ptr("10.0.0.0/8"),
					DDosProtectionPlanID: to.Ptr(ddosId),
				},
				Workers:          to.Ptr("10.0.0.0/16"),
				ServiceEndpoints: []string{},
				Zones:            []azure.Zone{{Name: 1, CIDR: "10.0.0.0/16", NatGateway: &azure.ZonedNatGatewayConfig{Enabled: true, IPAddresses: []azure.ZonedPublicIPReference{{Name: "my-ip", ResourceGroup: resourceGroupName}}}}, {Name: 2, CIDR: "10.1.0.0/16"}}, // subnets
			},
		}
		cluster := infrastructure.MakeCluster("11.0.0.0/16", "12.0.0.0/16", infra.Spec.Region, 1, 1)

		var vnet *mockclient.MockVnet
		BeforeEach(func() {
			ctrl := gomock.NewController(GinkgoT())
			vnet = mockclient.NewMockVnet(ctrl)
			parameters := armnetwork.VirtualNetwork{
				Location: to.Ptr(location),
				Properties: &armnetwork.VirtualNetworkPropertiesFormat{
					AddressSpace: &armnetwork.AddressSpace{},
				},
			}
			parameters.Properties.AddressSpace.AddressPrefixes = []*string{cfg.Networks.VNet.CIDR}

			parameters.Properties.EnableDdosProtection = to.Ptr(true)
			parameters.Properties.DdosProtectionPlan = &armnetwork.SubResource{ID: to.Ptr(ddosId)}
			vnet.EXPECT().CreateOrUpdate(gomock.Any(), resourceGroupName, vnetName, gomock.Any()).Return(nil)
		})
		It("", func() {
			sut, err := infraflow.NewTfReconciler(infra, cfg, cluster)
			Expect(err).ToNot(HaveOccurred())
			sut.Vnet(context.TODO(), vnet)
		})
	})
})
