// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"
	"errors"
	"reflect"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

var _ Access = &access{}

// Access provides additional methods that are build on top of the azure client primitives.
type Access interface {
	// DeletePublicIP deletes a public IP after disassociating it from the NAT Gateway if necessary.
	DeletePublicIP(ctx context.Context, rgName, pipName string) error
	// DisassociatePublicIP from the NAT Gateway it is attached.
	DisassociatePublicIP(ctx context.Context, rgName, natName, pipId string) error
	// DeleteNatGateway deletes a NAT Gateway after disassociating from all subnets attached to it.
	DeleteNatGateway(ctx context.Context, rgName, natName string) error
	// DisassociateNatGateway disassociates the NAT Gateway from attached subnets.
	DisassociateNatGateway(ctx context.Context, rgName, natName string) error
}

type access struct {
	f client.Factory
}

// DeletePublicIP deletes a public IP after disassociating it from the NAT Gateway if necessary.
func (p *access) DeletePublicIP(ctx context.Context, rgName, pipName string) error {
	pipClient, err := p.f.PublicIP()
	if err != nil {
		return err
	}

	pip, err := pipClient.Get(ctx, rgName, pipName, to.Ptr("natGateway"))
	if err != nil {
		return err
	}

	if pip.Properties.NatGateway != nil && pip.Properties.NatGateway.Name != nil {
		err := p.DisassociatePublicIP(ctx, rgName, *pip.Properties.NatGateway.Name, *pip.ID)
		if err != nil {
			return err
		}
	}

	return pipClient.Delete(ctx, rgName, pipName)
}

// DisassociatePublicIP disassociates a PublicIPConfig from it's attached NAT Gateway.
func (p *access) DisassociatePublicIP(ctx context.Context, rgName, natName, pipId string) error {
	natClient, err := p.f.NatGateway()
	if err != nil {
		return err
	}

	nat, err := natClient.Get(ctx, rgName, natName, nil)
	if err != nil {
		return err
	}

	var natPips []*armnetwork.SubResource
	for _, natPip := range nat.Properties.PublicIPAddresses {
		if natPip != nil && !reflect.DeepEqual(*natPip.ID, pipId) {
			natPips = append(natPips, natPip)
		}
	}
	nat.Properties.PublicIPAddresses = natPips

	_, err = natClient.CreateOrUpdate(ctx, rgName, natName, *nat)
	return err
}

// DeleteNatGateway deletes a NAT Gateway after disassociating from all subnets attached to it.
func (p *access) DeleteNatGateway(ctx context.Context, rgName, natName string) error {
	nc, err := p.f.NatGateway()
	if err != nil {
		return err
	}
	err = p.DisassociateNatGateway(ctx, rgName, natName)
	if err != nil {
		return err
	}
	return nc.Delete(ctx, rgName, natName)
}

// DisassociateNatGateway disassociates the NAT Gateway from attached subnets.
func (p *access) DisassociateNatGateway(ctx context.Context, rgName, natName string) error {
	nc, err := p.f.NatGateway()
	if err != nil {
		return err
	}
	sc, err := p.f.Subnet()
	if err != nil {
		return err
	}

	nat, err := nc.Get(ctx, rgName, natName, to.Ptr("subnets"))
	if err != nil {
		return err
	}

	var joinErr error
	for _, subnetId := range nat.Properties.Subnets {
		if subnetId == nil || subnetId.ID == nil {
			continue
		}
		subnetRID, err := arm.ParseResourceID(*subnetId.ID)
		if err != nil {
			return err
		}
		subnet, err := sc.Get(ctx, subnetRID.ResourceGroupName, subnetRID.Parent.Name, subnetRID.Name, nil)
		if err != nil {
			joinErr = errors.Join(joinErr, err)
			continue
		}
		if subnet == nil {
			continue
		}
		subnet.Properties.NatGateway = nil
		_, err = sc.CreateOrUpdate(ctx, subnetRID.ResourceGroupName, subnetRID.Parent.Name, subnetRID.Name, *subnet)
		joinErr = errors.Join(joinErr, err)
	}

	return joinErr
}
