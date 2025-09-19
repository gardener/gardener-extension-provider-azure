// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
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
	infrastructureStatus, err := getInfrastructureStatus(ctx, a, cluster)
	if err != nil {
		return err
	}

	opts, err := NewOpts(bastion, cluster, infrastructureStatus.ResourceGroup.Name, log)
	if err != nil {
		return err
	}

	cloudProfile, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return err
	}

	var cloudConfiguration *azure.CloudConfiguration
	if cloudProfile != nil {
		cloudConfiguration = cloudProfile.CloudConfiguration
	}

	azCloudConfiguration, err := azureclient.AzureCloudConfiguration(cloudConfiguration, &opts.Location)
	if err != nil {
		return err
	}

	clientFactory, err := azureclient.NewAzureClientFactoryFromSecret(
		ctx,
		a.client,
		opts.SecretReference,
		false,
		azureclient.WithCloudConfiguration(azCloudConfiguration),
	)
	if err != nil {
		return err
	}

	publicIP, err := ensurePublicIPAddress(ctx, clientFactory, opts)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	nic, err := ensureNic(ctx, clientFactory, infrastructureStatus, opts, publicIP)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	opts.NicID = *nic.ID

	// assume it's not possible to not have an ipv4 address
	opts.PrivateIPAddressV4, err = getPrivateIPv4Address(nic)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	opts.PrivateIPAddressV6, err = getPrivateIPv6Address(nic)
	if err != nil {
		log.Info(err.Error())
	}

	err = ensureNetworkSecurityGroups(ctx, clientFactory, opts)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	err = ensureComputeInstance(ctx, bastion, clientFactory, opts)
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

	if worker.Spec.InfrastructureProviderStatus == nil {
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

func getPrivateIPv4Address(nic *armnetwork.Interface) (string, error) {
	if len(nic.Properties.IPConfigurations) == 0 {
		return "", fmt.Errorf("nic.IPConfigurations %s is empty", *nic.ID)
	}

	ipConfigurations := nic.Properties.IPConfigurations
	for _, ipConfiguration := range ipConfigurations {
		if ipConfiguration.Properties.PrivateIPAddress != nil {
			ipv4 := net.ParseIP(*ipConfiguration.Properties.PrivateIPAddress).To4()
			if ipv4 != nil {
				return ipv4.String(), nil
			}
		}
	}

	return "", fmt.Errorf("failed to get IPv4 PrivateIPAddress on nic %s", *nic.ID)
}

func getPrivateIPv6Address(nic *armnetwork.Interface) (string, error) {
	if len(nic.Properties.IPConfigurations) == 0 {
		return "", fmt.Errorf("nic.IPConfigurations %s is empty", *nic.ID)
	}

	ipConfigurations := nic.Properties.IPConfigurations
	for _, ipConfiguration := range ipConfigurations {
		if ipConfiguration.Properties.PrivateIPAddress != nil {
			ip := net.ParseIP(*ipConfiguration.Properties.PrivateIPAddress)
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

func ensureNetworkSecurityGroups(ctx context.Context, factory azureclient.Factory, opts Options) error {
	expectedNSGRuleList := prepareNSGRules(opts)
	networkSecGroupResp, err := getNetworkSecurityGroup(ctx, factory, opts.BaseOptions)
	if err != nil {
		return err
	}

	if expectedNSGRulesPresentAndValid(networkSecGroupResp.Properties.SecurityRules, expectedNSGRuleList) {
		return nil
	}

	networkSecGroupResp.Properties.SecurityRules = addOrReplaceNsgRulesDefinition(networkSecGroupResp.Properties.SecurityRules, expectedNSGRuleList)

	if err := createOrUpdateNetworkSecGroup(ctx, factory, opts.BaseOptions, networkSecGroupResp); err != nil {
		return err
	}

	opts.Logr.Info("created or updated bastion security rules of network security group",
		"nsg", opts.SecurityGroupName,
		"rules", networkSecGroupResp.Properties.SecurityRules,
	)

	return nil
}

func prepareNSGRules(opts Options) []*armnetwork.SecurityRule {
	res := make([]*armnetwork.SecurityRule, 0)
	res = append(res, nsgEgressDenyAllIPv4(opts))
	res = append(res, nsgEgressAllowSSHToWorkerIPv4(opts))

	ipv4cidr := make([]string, 0)
	ipv6cidr := make([]string, 0)
	for _, cidr := range opts.CIDRs {
		ip, _, _ := net.ParseCIDR(cidr)
		if len(ip.To4()) == net.IPv4len {
			ipv4cidr = append(ipv4cidr, cidr)
		} else if len(ip.To16()) == net.IPv6len {
			ipv6cidr = append(ipv6cidr, cidr)
		}
	}

	ipv4Name := NSGIngressAllowSSHResourceNameIPv4(opts.BastionInstanceName)
	res = append(res, nsgIngressAllowSSH(ipv4Name, opts.PrivateIPAddressV4, ipv4cidr))

	if len(ipv6cidr) > 0 && opts.PrivateIPAddressV6 != "" {
		ipv6Name := NSGIngressAllowSSHResourceNameIPv6(opts.BastionInstanceName)
		res = append(res, nsgIngressAllowSSH(ipv6Name, opts.PrivateIPAddressV6, ipv6cidr))
	}

	return res
}

func ensurePublicIPAddress(ctx context.Context, factory azureclient.Factory, opts Options) (*armnetwork.PublicIPAddress, error) {
	publicIP, err := getPublicIP(ctx, factory, opts.BaseOptions)
	if err != nil {
		return nil, err
	}
	if publicIP != nil {
		if *publicIP.Properties.ProvisioningState != "Succeeded" {
			return nil, fmt.Errorf("public IP with name %v is not in \"Succeeded\" status: %s", publicIP.Name, *publicIP.Properties.ProvisioningState)
		}
		return publicIP, nil
	}

	parameters := publicIPAddressDefine(opts)

	publicIP, err = createOrUpdatePublicIP(ctx, factory, opts.BaseOptions, parameters)
	if err != nil {
		return nil, err
	}

	opts.Logr.Info("bastion compute instance public ip address created", "publicIP", *publicIP.Properties.IPAddress)
	return publicIP, nil
}

func ensureComputeInstance(ctx context.Context, bastion *extensionsv1alpha1.Bastion, factory azureclient.Factory, opts Options) error {
	instance, err := getBastionInstance(ctx, factory, opts.BaseOptions)
	if err != nil {
		return err
	}

	if instance != nil {
		if instance.Properties.ProvisioningState == nil {
			return fmt.Errorf("instance not running, status: nil")
		}
		if *instance.Properties.ProvisioningState == "Succeeded" {
			return nil
		} else {
			return fmt.Errorf("instance not running, status: %v", *instance.Properties.ProvisioningState)
		}
	}

	opts.Logr.Info("creating new bastion compute instance")

	publickey, err := createSSHPublicKey()
	if err != nil {
		return err
	}

	parameters := computeInstanceDefine(opts, bastion, publickey)

	_, err = createBastionInstance(ctx, factory, opts, parameters)
	if err != nil {
		return fmt.Errorf("failed to create bastion compute instance: %w", err)
	}
	return nil
}

func ensureNic(ctx context.Context, factory azureclient.Factory, infrastructureStatus *azure.InfrastructureStatus, opts Options, publicIP *armnetwork.PublicIPAddress) (*armnetwork.Interface, error) {
	nic, err := getNic(ctx, factory, opts)
	if err != nil {
		return nil, err
	}
	if nic != nil {
		if *nic.Properties.ProvisioningState != "Succeeded" {
			return nil, fmt.Errorf("network interface with name %v is not in \"Succeeded\" status: %s", nic.Name, *nic.Properties.ProvisioningState)
		}
		return nic, nil
	}

	opts.Logr.Info("create new bastion compute instance nic")

	subnet, err := getSubnet(ctx, factory, infrastructureStatus, opts)
	if err != nil {
		return nil, err
	}

	if subnet == nil || *subnet.ID == "" {
		return nil, errors.New("virtual network subnet must be not empty")
	}

	parameters := nicDefine(opts, publicIP, subnet)

	nicClient, err := factory.NetworkInterface()
	if err != nil {
		return nil, err
	}

	nic, err = nicClient.CreateOrUpdate(ctx, opts.ResourceGroupName, opts.NicName, *parameters)
	if err != nil || nic == nil {
		return nil, fmt.Errorf("failed to create bastion compute nic: %w", err)
	}

	return nic, nil
}

func getInstanceEndpoints(nic *armnetwork.Interface, publicIP *armnetwork.PublicIPAddress) (*bastionEndpoints, error) {
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
	externalIP := publicIP.Properties.IPAddress
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

func expectedNSGRulesPresentAndValid(existingRules []*armnetwork.SecurityRule, expectedRules []*armnetwork.SecurityRule) bool {
	if existingRules == nil || expectedRules == nil {
		return false
	}

	for _, desRule := range expectedRules {
		ruleExistAndValid := false
		for _, existingRule := range existingRules {
			// compare firewall rules by its names because names here kind of "IDs"
			if equalNotNil(desRule.Name, existingRule.Name) {
				if notEqualNotNil(desRule.Properties.SourceAddressPrefix, existingRule.Properties.SourceAddressPrefix) {
					return false
				}
				if notEqualNotNil(desRule.Properties.DestinationAddressPrefix, existingRule.Properties.DestinationAddressPrefix) {
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

func addOrReplaceNsgRulesDefinition(existingRules []*armnetwork.SecurityRule, desiredRules []*armnetwork.SecurityRule) (newRules []*armnetwork.SecurityRule) {
	if existingRules == nil || desiredRules == nil {
		return
	}

	result := make([]*armnetwork.SecurityRule, 0, len(existingRules)+len(desiredRules))

	bookedPriorityIDs := make(map[int32]bool)
	for _, rule := range existingRules {
		if rule.Properties.Priority == nil {
			continue
		}
		bookedPriorityIDs[*rule.Properties.Priority] = true
	}

	// filter rules intended to be replaced
	for _, existentRule := range existingRules {
		if RuleExist(existentRule.Name, desiredRules) {
			continue
		}
		result = append(result, existentRule)
	}

	// ensure uniq priority numbers
	for _, desiredRule := range desiredRules {
		desiredRule.Properties.Priority = findNextFreeNumber(bookedPriorityIDs, *desiredRule.Properties.Priority)
	}

	result = append(result, desiredRules...)
	newRules = result
	return
}

// RuleExist checks if the rule with the given name is present in the list of rules.
func RuleExist(ruleName *string, rules []*armnetwork.SecurityRule) bool {
	if ruleName == nil {
		return false
	}

	for _, rule := range rules {
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
