// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/bastion"
	"github.com/go-logr/logr"
	"golang.org/x/crypto/ssh"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

const (
	// SSHPort is the default SSH port.
	SSHPort = "22"
)

type actuator struct {
	client client.Client
}

func newActuator(mgr manager.Manager) bastion.Actuator {
	return &actuator{
		client: mgr.GetClient(),
	}
}

func createBastionInstance(ctx context.Context, factory azureclient.Factory, opt *Options, parameters armcompute.VirtualMachine) (*armcompute.VirtualMachine, error) {
	vmClient, err := factory.VirtualMachine()
	if err != nil {
		return nil, err
	}

	instance, err := vmClient.CreateOrUpdate(ctx, opt.ResourceGroupName, opt.BastionInstanceName, parameters)
	if err != nil {
		return nil, fmt.Errorf("unable to create VM instance %s: %w", opt.BastionInstanceName, err)
	}
	return instance, nil
}

func createOrUpdatePublicIP(ctx context.Context, factory azureclient.Factory, opt *Options, parameters *armnetwork.PublicIPAddress) (*armnetwork.PublicIPAddress, error) {
	publicClient, err := factory.PublicIP()
	if err != nil {
		return nil, err
	}

	ip, err := publicClient.CreateOrUpdate(ctx, opt.ResourceGroupName, opt.BastionPublicIPName, *parameters)
	if err != nil {
		return nil, fmt.Errorf("unable to create or update Public IP address %s: %w", opt.BastionPublicIPName, err)
	}
	return ip, nil
}

func createOrUpdateNetworkSecGroup(ctx context.Context, factory azureclient.Factory, opt *Options, parameters *armnetwork.SecurityGroup) error {
	if parameters == nil || parameters.Properties.SecurityRules == nil {
		return fmt.Errorf("network security group nor SecurityRules can't be nil, securityGroupName: %s", opt.SecurityGroupName)
	}

	nsgClient, err := factory.NetworkSecurityGroup()
	if err != nil {
		return err
	}

	_, err = nsgClient.CreateOrUpdate(ctx, opt.ResourceGroupName, opt.SecurityGroupName, *parameters)
	if err != nil {
		return fmt.Errorf("can't update Network Security Group %s: %w", opt.SecurityGroupName, err)
	}
	return nil
}

func getBastionInstance(ctx context.Context, log logr.Logger, factory azureclient.Factory, opt *Options) (*armcompute.VirtualMachine, error) {
	vmClient, err := factory.VirtualMachine()
	if err != nil {
		return nil, err
	}

	instance, err := vmClient.Get(ctx, opt.ResourceGroupName, opt.BastionInstanceName, to.Ptr(armcompute.InstanceViewTypesInstanceView))
	if err != nil {
		if azureclient.IsAzureAPINotFoundError(err) {
			log.Info("Instance not found,", "instance_name", opt.BastionInstanceName)
			return nil, nil
		}
		return nil, err
	}
	return instance, nil
}

func getNic(ctx context.Context, log logr.Logger, factory azureclient.Factory, opt *Options) (*armnetwork.Interface, error) {
	nicClient, err := factory.NetworkInterface()
	if err != nil {
		return nil, err
	}

	nic, err := nicClient.Get(ctx, opt.ResourceGroupName, opt.NicName)
	if err != nil {
		if azureclient.IsAzureAPINotFoundError(err) {
			log.Info("Nic not found,", "nic_name", opt.NicName)
			return nil, nil
		}
		return nil, err
	}

	return nic, nil
}

func getNetworkSecurityGroup(ctx context.Context, log logr.Logger, factory azureclient.Factory, opt *Options) (*armnetwork.SecurityGroup, error) {
	nsgClient, err := factory.NetworkSecurityGroup()
	if err != nil {
		return nil, err
	}

	nsgResp, err := nsgClient.Get(ctx, opt.ResourceGroupName, opt.SecurityGroupName)
	if err != nil {
		if azureclient.IsAzureAPINotFoundError(err) {
			log.Error(err, "Network Security Group not found, test environment?", "nsg_name", opt.SecurityGroupName)
			return nil, err
		}
		return nil, err
	}
	return nsgResp, nil
}

func getWorkersCIDR(cluster *controller.Cluster) ([]string, error) {
	infrastructureConfig := &azure.InfrastructureConfig{}
	err := json.Unmarshal(cluster.Shoot.Spec.Provider.InfrastructureConfig.Raw, infrastructureConfig)
	if err != nil {
		return nil, err
	}

	if len(infrastructureConfig.Networks.Zones) > 1 {
		var res []string
		for _, z := range infrastructureConfig.Networks.Zones {
			res = append(res, z.CIDR)
			return res, nil
		}
	}

	if infrastructureConfig.Networks.Workers != nil {
		return []string{*infrastructureConfig.Networks.Workers}, nil
	}
	return nil, fmt.Errorf("InfrastructureConfig.Networks.Workers is nil")
}

func getPublicIP(ctx context.Context, log logr.Logger, factory azureclient.Factory, opt *Options) (*armnetwork.PublicIPAddress, error) {
	ipClient, err := factory.PublicIP()
	if err != nil {
		return nil, err
	}

	ip, err := ipClient.Get(ctx, opt.ResourceGroupName, opt.BastionPublicIPName, nil)
	if err != nil {
		if azureclient.IsAzureAPINotFoundError(err) {
			log.Info("public IP not found,", "publicIP_name", opt.BastionPublicIPName)
			return nil, nil
		}
		return nil, err
	}
	return ip, nil
}

func getSubnet(ctx context.Context, log logr.Logger, factory azureclient.Factory, infrastructureStatus *azure.InfrastructureStatus, opt *Options) (*armnetwork.Subnet, error) {
	var sg string
	subnetClient, err := factory.Subnet()
	if err != nil {
		return nil, err
	}

	if infrastructureStatus.Networks.VNet.ResourceGroup != nil {
		sg = *infrastructureStatus.Networks.VNet.ResourceGroup
	} else {
		sg = opt.ResourceGroupName
	}

	subnet, err := subnetClient.Get(ctx, sg, infrastructureStatus.Networks.VNet.Name, infrastructureStatus.Networks.Subnets[0].Name, nil)
	if err != nil {
		return nil, err
	}

	if subnet == nil {
		log.Info("subnet not found,", "subnet_name", infrastructureStatus.Networks.Subnets[0].Name)
		return nil, nil
	}

	return subnet, nil
}

func deleteSecurityRuleDefinitionsByName(rulesArr []*armnetwork.SecurityRule, namesToRemove ...string) ([]*armnetwork.SecurityRule, bool) {
	rulesWereDeleted := false
	if rulesArr == nil {
		return rulesArr, rulesWereDeleted
	}

	result := make([]*armnetwork.SecurityRule, 0, len(rulesArr))
rules:
	for _, rule := range rulesArr {
		for _, nameToDelete := range namesToRemove {
			if rule.Name != nil && *rule.Name == nameToDelete {
				rulesWereDeleted = true
				continue rules
			}
		}
		result = append(result, rule)
	}
	return result, rulesWereDeleted
}

func equalNotNil(str1 *string, str2 *string) bool {
	if str1 == nil || str2 == nil {
		return false
	}
	return str1 == str2
}

func notEqualNotNil(str1 *string, str2 *string) bool {
	if str1 == nil || str2 == nil {
		return false
	}
	return str1 != str2
}

func createSSHPublicKey() (string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", err
	}

	// generate and write public key
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", err
	}

	return string(ssh.MarshalAuthorizedKey(pub)), nil
}
