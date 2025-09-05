// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	// DisassociatePublicIPFromNAT from the NAT Gateway it is attached.
	DisassociatePublicIPFromNAT(ctx context.Context, rgName, natName, pipId string) error
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
		err := p.DisassociatePublicIPFromNAT(ctx, rgName, *pip.Properties.NatGateway.Name, *pip.ID)
		if err != nil {
			return err
		}
	}
	if pip.Properties.IPConfiguration != nil && pip.Properties.IPConfiguration.ID != nil {
		ipcResource, err := arm.ParseResourceID(*pip.Properties.IPConfiguration.ID)
		if err != nil {
			return err
		}
		lbName := ipcResource.Parent.Name
		lbClient, err := p.f.LoadBalancer()
		if err != nil {
			return err
		}
		lb, err := lbClient.Get(ctx, rgName, lbName)
		if err != nil {
			return err
		}

		frontendIPConfigurations := make([]*armnetwork.FrontendIPConfiguration, 0)
		modifiedLB := false
		for _, frontend := range lb.Properties.FrontendIPConfigurations {
			if frontend == nil || frontend.Properties == nil || frontend.Properties.PublicIPAddress == nil ||
				frontend.Properties.PublicIPAddress.ID == nil || *frontend.Properties.PublicIPAddress.ID != *pip.ID {
				frontendIPConfigurations = append(frontendIPConfigurations, frontend)
				continue
			}

			modifiedLB = true

			// modifiedOR := false
			lbOutboundRules := make([]*armnetwork.OutboundRule, 0)
			for _, ob := range lb.Properties.OutboundRules {
				if ob.Properties.FrontendIPConfigurations != nil {
					var outboundRuleFrontendConfigs []*armnetwork.SubResource
					for _, fic := range ob.Properties.FrontendIPConfigurations {
						if fic != nil && fic.ID != nil && *fic.ID == *frontend.ID {
							continue // Skip the frontend config that is being removed
						}
						outboundRuleFrontendConfigs = append(outboundRuleFrontendConfigs, fic)
					}
					ob.Properties.FrontendIPConfigurations = outboundRuleFrontendConfigs
				}
				if len(ob.Properties.FrontendIPConfigurations) != 0 {
					lbOutboundRules = append(lbOutboundRules, ob)
					// } else {
					// 	modifiedOR = true
				}
			}
			lb.Properties.OutboundRules = lbOutboundRules
			// if modifiedOR {
			// 	lb, err = lbClient.CreateOrUpdate(ctx, rgName, lbName, *lb)
			// 	if err != nil {
			// 		return err
			// 	}
			// }

			// if frontend != nil && frontend.Properties != nil && frontend.Properties.PublicIPAddress != nil &&
			// 	frontend.Properties.PublicIPAddress.ID != nil && *frontend.Properties.PublicIPAddress.ID == *pip.ID {
			// 	// Remove the PublicIPConfig from the FrontendIPConfiguration
			// 	lb.Properties.FrontendIPConfigurations = append(lb.Properties.FrontendIPConfigurations[:i], lb.Properties.FrontendIPConfigurations[i+1:]...)
			//
			// 	for _, ob := range lb.Properties.OutboundRules {
			// 		if ob.Properties.FrontendIPConfigurations != nil {
			// 			var updatedFrontendConfigs []*armnetwork.SubResource
			// 			for _, fic := range ob.Properties.FrontendIPConfigurations {
			// 				if fic != nil && fic.ID != nil && *fic.ID == *frontend.ID
			// 					continue // Skip the frontend config that is being removed
			// 				}
			// 				updatedFrontendConfigs = append(updatedFrontendConfigs, fic)
			// 			}
			// 			ob.Properties.FrontendIPConfigurations = updatedFrontendConfigs
			// 		}
			// 	}
			// break
		}
		lb.Properties.FrontendIPConfigurations = frontendIPConfigurations
		if len(lb.Properties.FrontendIPConfigurations) == 0 {
			if err := lbClient.Delete(ctx, rgName, lbName); err != nil {
				return err
			}
		} else if modifiedLB {
			// Update the LoadBalancer to remove the PublicIPConfig
			if _, err = lbClient.CreateOrUpdate(ctx, rgName, lbName, *lb); err != nil {
				return err
			}
		}
	}

	return pipClient.Delete(ctx, rgName, pipName)
}

// DisassociatePublicIP disassociates a PublicIPConfig from it's attached NAT Gateway.
func (p *access) DisassociatePublicIPFromNAT(ctx context.Context, rgName, natName, pipId string) error {
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
