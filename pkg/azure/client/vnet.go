package client

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-05-01/network"
)

// idempotent
//
//	network.VirtualNetwork{
//		Location: to.StringPtr(config.Location()),
//		VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
//			AddressSpace: &network.AddressSpace{
//				AddressPrefixes: &[]string{"10.0.0.0/8"},
//			},
//		},
//	})}
//
// with subnet
//
//	VirtualNetworkPropertiesFormat{
//		AddressSpace: &network.AddressSpace{
//			AddressPrefixes: &[]string{"10.0.0.0/8"},
//		}``,
//		Subnets: &[]network.Subnet{
//			{
//				Name: to.StringPtr(subnet1Name),
//				SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
//					AddressPrefix: to.StringPtr("10.0.0.0/16"),
//				},
//			},
//			{
//				Name: to.StringPtr(subnet2Name),
//				SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
//					AddressPrefix: to.StringPtr("10.1.0.0/16"),
//				},
//			},
//		},
//	},
func (v VnetClient) Create(ctx context.Context, resourceGroupName string, name string, parameters network.VirtualNetwork) (vnet network.VirtualNetwork, err error) {
	future, err := v.client.CreateOrUpdate(
		ctx, resourceGroupName, name, parameters)
	if err != nil {
		return vnet, fmt.Errorf("cannot create virtual network: %v", err)
	}

	err = future.WaitForCompletionRef(ctx, v.client.Client)
	if err != nil {
		return vnet, fmt.Errorf("cannot get the vnet create or update future response: %v", err)
	}
	return future.Result(v.client)
}

// DeleteVirtualNetwork deletes a virtual network given an existing virtual network
func (v VnetClient) Delete(ctx context.Context, resourceGroup, vnetName string) (result network.VirtualNetworksDeleteFuture, err error) {
	return v.client.Delete(ctx, resourceGroup, vnetName)
}
