// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bastion

import (
	"encoding/base64"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-03-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-05-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

func nicDefine(opt *Options, publicIP *network.PublicIPAddress, subnet *armnetwork.SubnetsClientGetResponse) *network.Interface {
	return &network.Interface{
		Name:     &opt.NicName,
		Location: &opt.Location,
		InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
			IPConfigurations: &[]network.InterfaceIPConfiguration{
				{
					Name: to.StringPtr("ipConfig1"),
					InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
						Subnet: &network.Subnet{
							ID: subnet.ID,
						},
						PrivateIPAllocationMethod: network.Dynamic,
						PublicIPAddress:           publicIP,
					},
				},
			},
		},
		Tags: opt.Tags,
	}
}

func publicIPAddressDefine(opt *Options) *network.PublicIPAddress {
	return &network.PublicIPAddress{
		Name:     &opt.BastionPublicIPName,
		Location: &opt.Location,
		Sku: &network.PublicIPAddressSku{
			Name: network.PublicIPAddressSkuNameStandard,
		},
		PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
			PublicIPAddressVersion:   network.IPv4,
			PublicIPAllocationMethod: network.Static,
		},
		Tags: opt.Tags,
	}
}

func computeInstanceDefine(opt *Options, bastion *extensionsv1alpha1.Bastion, publickey string) *compute.VirtualMachine {
	return &compute.VirtualMachine{
		Location: &opt.Location,
		VirtualMachineProperties: &compute.VirtualMachineProperties{
			HardwareProfile: &compute.HardwareProfile{
				VMSize: compute.VirtualMachineSizeTypesStandardB1s,
			},
			StorageProfile: &compute.StorageProfile{
				ImageReference: &compute.ImageReference{
					Publisher: to.StringPtr("Canonical"),
					Offer:     to.StringPtr("UbuntuServer"),
					Sku:       to.StringPtr("18.04-LTS"),
					Version:   to.StringPtr("latest"),
				},
				OsDisk: &compute.OSDisk{
					CreateOption: compute.DiskCreateOptionTypesFromImage,
					DiskSizeGB:   to.Int32Ptr(32),
					Name:         &opt.DiskName,
				},
			},
			OsProfile: &compute.OSProfile{
				ComputerName:  &opt.BastionInstanceName,
				AdminUsername: to.StringPtr("gardener"),
				LinuxConfiguration: &compute.LinuxConfiguration{
					DisablePasswordAuthentication: to.BoolPtr(true),
					SSH: &compute.SSHConfiguration{
						PublicKeys: &[]compute.SSHPublicKey{
							{
								Path: to.StringPtr("/home/gardener/.ssh/authorized_keys"),
								// Random, temporary SSH public key to suffice the azure API, as creating an instance without a public key is not possible. The UserData will overwrite it later.
								// We could have also used the user's public SSH key but currently it's not available on the `Bastion` extension resource.
								KeyData: to.StringPtr(publickey),
							},
						},
					},
				},
			},
			UserData: to.StringPtr(base64.StdEncoding.EncodeToString(bastion.Spec.UserData)),
			NetworkProfile: &compute.NetworkProfile{
				NetworkInterfaces: &[]compute.NetworkInterfaceReference{
					{
						ID: &opt.NicID,
						NetworkInterfaceReferenceProperties: &compute.NetworkInterfaceReferenceProperties{
							Primary: to.BoolPtr(true),
						},
					},
				},
			},
		},
		Tags: opt.Tags,
	}
}

func nsgIngressAllowSSH(ruleName string, destinationAddress string, sourceAddresses []string) network.SecurityRule {
	return network.SecurityRule{
		Name: to.StringPtr(ruleName),
		SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
			Protocol:                 network.SecurityRuleProtocolTCP,
			SourceAddressPrefixes:    &sourceAddresses,
			SourcePortRange:          to.StringPtr("*"),
			DestinationAddressPrefix: &destinationAddress,
			DestinationPortRange:     to.StringPtr(SSHPort),
			Access:                   network.SecurityRuleAccessAllow,
			Direction:                network.SecurityRuleDirectionInbound,
			Priority:                 to.Int32Ptr(400),
			Description:              to.StringPtr("SSH access for Bastion"),
		},
	}
}

func nsgEgressDenyAllIPv4(opt *Options) network.SecurityRule {
	return network.SecurityRule{
		Name: to.StringPtr(NSGEgressDenyAllResourceName(opt.BastionInstanceName)),
		SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
			Protocol:                 network.SecurityRuleProtocolAsterisk,
			SourceAddressPrefix:      &opt.PrivateIPAddressV4,
			SourcePortRange:          to.StringPtr("*"),
			DestinationAddressPrefix: to.StringPtr("*"),
			DestinationPortRange:     to.StringPtr("*"),
			Access:                   network.SecurityRuleAccessDeny,
			Direction:                network.SecurityRuleDirectionOutbound,
			Priority:                 to.Int32Ptr(1000),
			Description:              to.StringPtr("Bastion egress deny ipv4"),
		},
	}
}

func nsgEgressAllowSSHToWorkerIPv4(opt *Options) network.SecurityRule {
	return network.SecurityRule{
		Name: to.StringPtr(NSGEgressAllowOnlyResourceName(opt.BastionInstanceName)),
		SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
			Protocol:                   network.SecurityRuleProtocolTCP,
			SourceAddressPrefix:        &opt.PrivateIPAddressV4,
			SourcePortRange:            to.StringPtr("*"),
			DestinationAddressPrefixes: to.StringSlicePtr(opt.WorkersCIDR),
			DestinationPortRange:       to.StringPtr(SSHPort),
			Access:                     network.SecurityRuleAccessAllow,
			Direction:                  network.SecurityRuleDirectionOutbound,
			Priority:                   to.Int32Ptr(401),
			Description:                to.StringPtr("Allow Bastion egress to Shoot workers ipv4"),
		},
	}
}
