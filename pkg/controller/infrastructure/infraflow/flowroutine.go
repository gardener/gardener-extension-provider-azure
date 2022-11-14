package infraflow

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"golang.org/x/sync/errgroup"
)

type FlowRoutine struct {
	group           *errgroup.Group
	securityGroupCh chan armnetwork.SecurityGroupsClientCreateOrUpdateResponse
	routeTableCh    chan armnetwork.RouteTable
	ipCh            chan map[string]armnetwork.PublicIPAddressesClientCreateOrUpdateResponse
	natGatewayCh    chan map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse
}

func NewFlowRoutine(ctx context.Context) *FlowRoutine {
	securityGroupCh := make(chan armnetwork.SecurityGroupsClientCreateOrUpdateResponse)
	routeTableCh := make(chan armnetwork.RouteTable)
	ipCh := make(chan map[string]armnetwork.PublicIPAddressesClientCreateOrUpdateResponse)
	natGatewayCh := make(chan map[string]armnetwork.NatGatewaysClientCreateOrUpdateResponse, 2)
	g, ctx := errgroup.WithContext(ctx) //https://stackoverflow.com/questions/45500836/close-multiple-goroutine-if-an-error-occurs-in-one-in-go
	return &FlowRoutine{g, securityGroupCh, routeTableCh, ipCh, natGatewayCh}
}

func (f *FlowRoutine) AddTask() {
	f.group.Go(func() error {
		//resp, err := f.reconcilePublicIPsFromTf(ctx, tfAdapter)
		//ipCh <- resp
		return nil //err
	})
}
