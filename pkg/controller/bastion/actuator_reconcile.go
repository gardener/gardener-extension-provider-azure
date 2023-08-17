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
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-03-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-05-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	ctrlerror "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

const (
	// IMAGE_PUBLISHER a const for the image published used in bastion.
	IMAGE_PUBLISHER = "Canonical"
	// IMAGE_OFFER a const for the image offer used in bastion.
	IMAGE_OFFER = "0001-com-ubuntu-server-jammy"
)

// bastionEndpoints holds the endpoints the bastion host provides
type bastionEndpoints struct {
	// private is the private endpoint of the bastion. It is required when opening a port on the worker node ingress network security group rule to allow SSH access from the bastion
	private *corev1.LoadBalancerIngress
	//  public is the public endpoint where the enduser connects to establish the SSH connection.
	public *corev1.LoadBalancerIngress
}

// Ready returns true if both public and private interfaces each have either
// an IP or a hostname or both.
func (be *bastionEndpoints) Ready() bool {
	return be != nil && IngressReady(be.private) && IngressReady(be.public)
}

func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, bastion *extensionsv1alpha1.Bastion, cluster *controller.Cluster) error {
	factory := azureclient.NewAzureClientFactory(a.client)

	infrastructureStatus, err := getInfrastructureStatus(ctx, a, cluster)
	if err != nil {
		return err
	}

	opt, err := DetermineOptions(bastion, cluster, infrastructureStatus.ResourceGroup.Name)
	if err != nil {
		return err
	}

	publicIP, err := ensurePublicIPAddress(ctx, log, factory, opt)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	nic, err := ensureNic(ctx, log, factory, infrastructureStatus, opt, publicIP)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	opt.NicID = *nic.ID

	// assume it's not possible to not have an ipv4 address
	opt.PrivateIPAddressV4, err = getPrivateIPv4Address(nic)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	opt.PrivateIPAddressV6, err = getPrivateIPv6Address(nic)
	if err != nil {
		log.Info(err.Error())
	}

	err = ensureNetworkSecurityGroups(ctx, log, factory, opt)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	err = ensureComputeInstance(ctx, log, bastion, factory, opt)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	// check if the instance already exists and has an IP
	endpoints, err := getInstanceEndpoints(nic, publicIP)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	if !endpoints.Ready() {
		return &ctrlerror.RequeueAfterError{
			// requeue rather soon, so that the user (most likely gardenctl eventually)
			// doesn't have to wait too long for the public endpoint to become available
			RequeueAfter: 5 * time.Second,
			Cause:        fmt.Errorf("bastion instance has no public/private endpoints yet"),
		}
	}

	patch := client.MergeFrom(bastion.DeepCopy())
	bastion.Status.Ingress = endpoints.public
	return a.client.Status().Patch(ctx, bastion, patch)
}

func getInfrastructureStatus(ctx context.Context, a *actuator, cluster *extensions.Cluster) (*azure.InfrastructureStatus, error) {
	var (
		infrastructureStatus *azure.InfrastructureStatus
		err                  error
	)

	worker := &extensionsv1alpha1.Worker{}
	if err = a.client.Get(ctx, client.ObjectKey{Namespace: cluster.ObjectMeta.Name, Name: cluster.Shoot.Name}, worker); err != nil {
		return nil, err
	}

	if worker == nil || worker.Spec.InfrastructureProviderStatus == nil {
		return nil, errors.New("infrastructure provider status must be not empty for worker")
	}

	if infrastructureStatus, err = helper.InfrastructureStatusFromRaw(worker.Spec.InfrastructureProviderStatus); err != nil {
		return nil, err
	}

	if infrastructureStatus.ResourceGroup.Name == "" {
		return nil, errors.New("resource group name must be not empty for infrastructure provider status")
	}

	if infrastructureStatus.Networks.VNet.Name == "" {
		return nil, errors.New("virtual network name must be not empty for infrastructure provider status")
	}

	if len(infrastructureStatus.Networks.Subnets) == 0 {
		return nil, errors.New("subnets name must be not empty for infrastructure provider status")
	}
	return infrastructureStatus, nil
}

func getPrivateIPv4Address(nic *network.Interface) (string, error) {
	if nic.IPConfigurations == nil {
		return "", fmt.Errorf("nic.IPConfigurations %s is nil", *nic.ID)
	}

	ipConfigurations := *nic.IPConfigurations
	for _, ipConfiguration := range ipConfigurations {
		if ipConfiguration.PrivateIPAddress != nil {
			ipv4 := net.ParseIP(*ipConfiguration.PrivateIPAddress).To4()
			if ipv4 != nil {
				return ipv4.String(), nil
			}
		}
	}

	return "", fmt.Errorf("failed to get IPv4 PrivateIPAddress on nic %s", *nic.ID)
}

func getPrivateIPv6Address(nic *network.Interface) (string, error) {
	if nic.IPConfigurations == nil {
		return "", fmt.Errorf("nic.IPConfigurations %s is nil", *nic.ID)
	}

	ipConfigurations := *nic.IPConfigurations
	for _, ipConfiguration := range ipConfigurations {
		if ipConfiguration.PrivateIPAddress != nil {
			ip := net.ParseIP(*ipConfiguration.PrivateIPAddress)
			if len(ip.To4()) == net.IPv4len {
				continue
			}
			if len(ip.To16()) == net.IPv6len {
				return ip.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no IPv6 PrivateIPAddress found on nic %s", *nic.ID)
}

func ensureNetworkSecurityGroups(ctx context.Context, log logr.Logger, factory azureclient.Factory, opt *Options) error {
	expectedNSGRuleList := prepareNSGRules(opt)
	networkSecGroupResp, err := getNetworkSecurityGroup(ctx, log, factory, opt)
	if err != nil {
		return err
	}

	if expectedNSGRulesPresentAndValid(networkSecGroupResp.SecurityRules, expectedNSGRuleList) {
		return nil
	}

	addOrReplaceNsgRulesDefinition(networkSecGroupResp.SecurityRules, expectedNSGRuleList)

	if err := createOrUpdateNetworkSecGroup(ctx, factory, opt, networkSecGroupResp); err != nil {
		return err
	}

	log.Info("created or updated bastion security rules of network security group",
		"nsg", opt.SecurityGroupName,
		"rules", networkSecGroupResp.SecurityRules,
	)

	return nil
}

func prepareNSGRules(opt *Options) *[]network.SecurityRule {
	res := make([]network.SecurityRule, 0)
	res = append(res, nsgEgressDenyAllIPv4(opt))
	res = append(res, nsgEgressAllowSSHToWorkerIPv4(opt))

	ipv4cidr := make([]string, 0)
	ipv6cidr := make([]string, 0)
	for _, cidr := range opt.CIDRs {
		ip, _, _ := net.ParseCIDR(cidr)
		if len(ip.To4()) == net.IPv4len {
			ipv4cidr = append(ipv4cidr, cidr)
		} else if len(ip.To16()) == net.IPv6len {
			ipv6cidr = append(ipv6cidr, cidr)
		}
	}

	ipv4Name := NSGIngressAllowSSHResourceNameIPv4(opt.BastionInstanceName)
	res = append(res, nsgIngressAllowSSH(ipv4Name, opt.PrivateIPAddressV4, ipv4cidr))

	if len(ipv6cidr) > 0 && opt.PrivateIPAddressV6 != "" {
		ipv6Name := NSGIngressAllowSSHResourceNameIPv6(opt.BastionInstanceName)
		res = append(res, nsgIngressAllowSSH(ipv6Name, opt.PrivateIPAddressV6, ipv6cidr))
	}

	return &res
}

func ensurePublicIPAddress(ctx context.Context, log logr.Logger, factory azureclient.Factory, opt *Options) (*network.PublicIPAddress, error) {
	publicIP, err := getPublicIP(ctx, log, factory, opt)
	if err != nil {
		return nil, err
	}
	if publicIP != nil {
		if publicIP.ProvisioningState != "Succeeded" {
			return nil, fmt.Errorf("public IP with name %v is not in \"Succeeded\" status: %s", publicIP.Name, publicIP.ProvisioningState)
		}
		return publicIP, nil
	}

	parameters := publicIPAddressDefine(opt)

	publicIP, err = createOrUpdatePublicIP(ctx, factory, opt, parameters)
	if err != nil {
		return nil, err
	}

	log.Info("bastion compute instance public ip address created", "publicIP", *publicIP.IPAddress)
	return publicIP, nil
}

func ensureComputeInstance(ctx context.Context, log logr.Logger, bastion *extensionsv1alpha1.Bastion, factory azureclient.Factory, opt *Options) error {
	instance, err := getBastionInstance(ctx, log, factory, opt)
	if err != nil {
		return err
	}

	if instance != nil {
		if instance.ProvisioningState == nil {
			return fmt.Errorf("instance not running, status: nil")
		}
		if *instance.ProvisioningState == "Succeeded" {
			return nil
		} else {
			return fmt.Errorf("instance not running, status: %v", *instance.ProvisioningState)
		}
	}

	log.Info("creating new bastion compute instance")

	publickey, err := createSSHPublicKey()
	if err != nil {
		return err
	}

	sku, err := getLatestSku(ctx, opt, factory)
	if err != nil {
		return err
	}

	parameters := computeInstanceDefine(opt, sku, bastion, publickey)

	_, err = createBastionInstance(ctx, factory, opt, parameters)
	if err != nil {
		return fmt.Errorf("failed to create bastion compute instance: %w", err)
	}
	return nil
}

func ensureNic(ctx context.Context, log logr.Logger, factory azureclient.Factory, infrastructureStatus *azure.InfrastructureStatus, opt *Options, publicIP *network.PublicIPAddress) (*network.Interface, error) {
	nic, err := getNic(ctx, log, factory, opt)
	if err != nil {
		return nil, err
	}
	if nic != nil {
		if nic.ProvisioningState != "Succeeded" {
			return nil, fmt.Errorf("network interface with name %v is not in \"Succeeded\" status: %s", nic.Name, nic.ProvisioningState)
		}
		return nic, nil
	}

	log.Info("create new bastion compute instance nic")

	subnet, err := getSubnet(ctx, log, factory, infrastructureStatus, opt)
	if err != nil {
		return nil, err
	}

	if subnet == nil || *subnet.ID == "" {
		return nil, errors.New("virtual network subnet must be not empty")
	}

	parameters := nicDefine(opt, publicIP, subnet)

	nicClient, err := factory.NetworkInterface(ctx, opt.SecretReference)
	if err != nil {
		return nil, err
	}

	nic, err = nicClient.CreateOrUpdate(ctx, opt.ResourceGroupName, opt.NicName, *parameters)
	if err != nil || nic == nil {
		return nil, fmt.Errorf("failed to create bastion compute nic: %w", err)
	}

	return nic, nil
}

func getInstanceEndpoints(nic *network.Interface, publicIP *network.PublicIPAddress) (*bastionEndpoints, error) {
	endpoints := &bastionEndpoints{}

	internalIP, err := getPrivateIPv4Address(nic)
	if err != nil {
		return nil, fmt.Errorf("no internal IP found: %v", err)
	}

	if ingress := addressToIngress(nil, &internalIP); ingress != nil {
		endpoints.private = ingress
	}

	// Azure does not automatically assign a public dns name to the instance (in contrast to e.g. AWS).
	// As we provide an externalIP to connect to the bastion, having a public dns name would just be an alternative way to connect to the bastion.
	// Out of this reason, we spare the effort to create a PTR record (see https://docs.microsoft.com/en-us/azure/dns/dns-reverse-dns-hosting) just for the sake of having it.
	externalIP := publicIP.IPAddress
	if ingress := addressToIngress(nil, externalIP); ingress != nil {
		endpoints.public = ingress
	}

	return endpoints, nil
}

// IngressReady returns true if either an IP or a hostname or both are set.
func IngressReady(ingress *corev1.LoadBalancerIngress) bool {
	return ingress != nil && (ingress.Hostname != "" || ingress.IP != "")
}

// addressToIngress converts the IP address into a
// corev1.LoadBalancerIngress resource. If both arguments are nil, then
// nil is returned.
func addressToIngress(dnsName *string, ipAddress *string) *corev1.LoadBalancerIngress {
	var ingress *corev1.LoadBalancerIngress

	if ipAddress != nil || dnsName != nil {
		ingress = &corev1.LoadBalancerIngress{}
		if dnsName != nil {
			ingress.Hostname = *dnsName
		}

		if ipAddress != nil {
			ingress.IP = *ipAddress
		}
	}

	return ingress
}

func expectedNSGRulesPresentAndValid(existingRules *[]network.SecurityRule, expectedRules *[]network.SecurityRule) bool {
	if existingRules == nil || expectedRules == nil {
		return false
	}

	for _, desRule := range *expectedRules {
		ruleExistAndValid := false
		for _, existingRule := range *existingRules {
			// compare firewall rules by its names because names here kind of "IDs"
			if equalNotNil(desRule.Name, existingRule.Name) {
				if notEqualNotNil(desRule.SourceAddressPrefix, existingRule.SourceAddressPrefix) {
					return false
				}
				if notEqualNotNil(desRule.DestinationAddressPrefix, existingRule.DestinationAddressPrefix) {
					return false
				}
				ruleExistAndValid = true
			}
		}
		if !ruleExistAndValid {
			return false
		}
	}
	return true
}

func addOrReplaceNsgRulesDefinition(existingRules *[]network.SecurityRule, desiredRules *[]network.SecurityRule) {
	if existingRules == nil || desiredRules == nil {
		return
	}

	result := make([]network.SecurityRule, 0, len(*existingRules)+len(*desiredRules))

	bookedPriorityIDs := make(map[int32]bool)
	for _, rule := range *existingRules {
		if rule.Priority == nil {
			continue
		}
		bookedPriorityIDs[*rule.Priority] = true
	}

	// filter rules intended to be replaced
	for _, existentRule := range *existingRules {
		if RuleExist(existentRule.Name, desiredRules) {
			continue
		}
		result = append(result, existentRule)
	}

	// ensure uniq priority numbers
	for _, desiredRule := range *desiredRules {
		desiredRule.Priority = findNextFreeNumber(bookedPriorityIDs, *desiredRule.Priority)
	}

	result = append(result, *desiredRules...)
	*existingRules = result
}

// RuleExist checks if the rule with the given name is present in the list of rules.
func RuleExist(ruleName *string, rules *[]network.SecurityRule) bool {
	if ruleName == nil {
		return false
	}

	for _, rule := range *rules {
		if rule.Name != nil && *rule.Name == *ruleName {
			return true
		}
	}
	return false
}

func findNextFreeNumber(set map[int32]bool, baseValue int32) *int32 {
	if set[baseValue] {
		incremented := baseValue + 1
		return findNextFreeNumber(set, incremented)
	}
	set[baseValue] = true
	return &baseValue
}

func getLatestSku(ctx context.Context, opt *Options, factory azureclient.Factory) (*compute.ImageReference, error) {
	vmImageclient, err := factory.VirtualMachineImage(ctx, opt.SecretReference)
	if err != nil {
		return nil, err
	}

	result, err := vmImageclient.ListSkus(ctx, opt.Location, IMAGE_PUBLISHER, IMAGE_OFFER)
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`^\d+_\d+-(?:lts|LTS)$`)
	var sku string
	for _, v := range *result.Value {
		if re.MatchString(*v.Name) {
			// images are sorted by the api and we only need to keep the latest image
			sku = *v.Name
		}
	}

	if sku == "" {
		return nil, errors.New("sku not found")
	}

	return &compute.ImageReference{
		Publisher: to.StringPtr(IMAGE_PUBLISHER),
		Offer:     to.StringPtr(IMAGE_OFFER),
		Sku:       to.StringPtr(sku),
		Version:   to.StringPtr("latest"),
	}, nil
}
