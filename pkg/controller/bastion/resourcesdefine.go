// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"encoding/base64"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"k8s.io/utils/ptr"
)

func nicDefine(opt *Options, publicIP *armnetwork.PublicIPAddress, subnet *armnetwork.Subnet) *armnetwork.Interface {
	return &armnetwork.Interface{
		Name:     &opt.NicName,
		Location: &opt.Location,
		Properties: &armnetwork.InterfacePropertiesFormat{
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
				{
					Name: to.Ptr("ipConfig1"),
					Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
						Subnet: &armnetwork.Subnet{
							ID: subnet.ID,
						},
						PrivateIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodDynamic),
						PublicIPAddress:           publicIP,
					},
				},
			},
		},
		Tags: opt.Tags,
	}
}

func publicIPAddressDefine(opt *Options) *armnetwork.PublicIPAddress {
	return &armnetwork.PublicIPAddress{
		Name:     &opt.BastionPublicIPName,
		Location: &opt.Location,
		SKU: &armnetwork.PublicIPAddressSKU{
			Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard),
		},
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAddressVersion:   to.Ptr(armnetwork.IPVersionIPv4),
			PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
		},
		Tags: opt.Tags,
	}
}

func computeInstanceDefine(opt *Options, bastion *extensionsv1alpha1.Bastion, publickey string) armcompute.VirtualMachine {
	return armcompute.VirtualMachine{
		Location: &opt.Location,
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: ptr.To(armcompute.VirtualMachineSizeTypes(opt.MachineType)),
			},
			StorageProfile: &armcompute.StorageProfile{
				ImageReference: opt.ImageRef,
				OSDisk: &armcompute.OSDisk{
					CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
					DiskSizeGB:   to.Ptr(int32(32)),
					Name:         &opt.DiskName,
				},
			},
			OSProfile: &armcompute.OSProfile{
				ComputerName:  &opt.BastionInstanceName,
				AdminUsername: to.Ptr("gardener"),
				LinuxConfiguration: &armcompute.LinuxConfiguration{
					DisablePasswordAuthentication: to.Ptr(true),
					SSH: &armcompute.SSHConfiguration{
						PublicKeys: []*armcompute.SSHPublicKey{
							{
								Path: to.Ptr("/home/gardener/.ssh/authorized_keys"),
								// Random, temporary SSH public key to suffice the azure API, as creating an instance without a public key is not possible. The UserData will overwrite it later.
								// We could have also used the user's public SSH key but currently it's not available on the `Bastion` extension resource.
								KeyData: to.Ptr(publickey),
							},
						},
					},
				},
			},
			UserData: to.Ptr(base64.StdEncoding.EncodeToString(bastion.Spec.UserData)),
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
					{
						ID: &opt.NicID,
						Properties: &armcompute.NetworkInterfaceReferenceProperties{
							Primary: to.Ptr(true),
						},
					},
				},
			},
		},
		Tags: opt.Tags,
	}
}

func nsgIngressAllowSSH(ruleName string, destinationAddress string, sourceAddresses []string) *armnetwork.SecurityRule {
	return &armnetwork.SecurityRule{
		Name: to.Ptr(ruleName),
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
			SourceAddressPrefixes:    to.SliceOfPtrs(sourceAddresses...),
			SourcePortRange:          to.Ptr("*"),
			DestinationAddressPrefix: &destinationAddress,
			DestinationPortRange:     to.Ptr(SSHPort),
			Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
			Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
			Priority:                 to.Ptr(int32(400)),
			Description:              to.Ptr("SSH access for Bastion"),
		},
	}
}

func nsgEgressDenyAllIPv4(opt *Options) *armnetwork.SecurityRule {
	return &armnetwork.SecurityRule{
		Name: to.Ptr(NSGEgressDenyAllResourceName(opt.BastionInstanceName)),
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolAsterisk),
			SourceAddressPrefix:      &opt.PrivateIPAddressV4,
			SourcePortRange:          to.Ptr("*"),
			DestinationAddressPrefix: to.Ptr("*"),
			DestinationPortRange:     to.Ptr("*"),
			Access:                   to.Ptr(armnetwork.SecurityRuleAccessDeny),
			Direction:                to.Ptr(armnetwork.SecurityRuleDirectionOutbound),
			Priority:                 to.Ptr(int32(1000)),
			Description:              to.Ptr("Bastion egress deny ipv4"),
		},
	}
}

func nsgEgressAllowSSHToWorkerIPv4(opt *Options) *armnetwork.SecurityRule {
	return &armnetwork.SecurityRule{
		Name: to.Ptr(NSGEgressAllowOnlyResourceName(opt.BastionInstanceName)),
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Protocol:                   to.Ptr(armnetwork.SecurityRuleProtocolTCP),
			SourceAddressPrefix:        &opt.PrivateIPAddressV4,
			SourcePortRange:            to.Ptr("*"),
			DestinationAddressPrefixes: to.SliceOfPtrs(opt.WorkersCIDR...),
			DestinationPortRange:       to.Ptr(SSHPort),
			Access:                     to.Ptr(armnetwork.SecurityRuleAccessAllow),
			Direction:                  to.Ptr(armnetwork.SecurityRuleDirectionOutbound),
			Priority:                   to.Ptr(int32(401)),
			Description:                to.Ptr("Allow Bastion egress to Shoot workers ipv4"),
		},
	}
}
