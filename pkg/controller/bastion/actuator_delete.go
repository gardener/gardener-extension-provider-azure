// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	ctrlerror "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/go-logr/logr"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

func (a *actuator) Delete(ctx context.Context, log logr.Logger, bastion *extensionsv1alpha1.Bastion, cluster *controller.Cluster) error {

	infrastructureStatus, err := getInfrastructureStatus(ctx, a, cluster)
	if err != nil {
		return err
	}

	opt, err := DetermineOptions(bastion, cluster, infrastructureStatus.ResourceGroup.Name)
	if err != nil {
		return err
	}

	cloudProfile, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return err
	}

	azCloudConfiguration, err := azureclient.AzureCloudConfiguration(cloudProfile.CloudConfiguration, &cluster.Shoot.Spec.Region)

	if err != nil {
		return err
	}

	factory, err := azureclient.NewAzureClientFactoryFromSecret(
		ctx,
		a.client,
		opt.SecretReference,
		false,
		azureclient.WithCloudConfiguration(azCloudConfiguration),
	)
	if err != nil {
		return err
	}

	err = removeBastionInstance(ctx, log, factory, opt)
	if err != nil {
		return util.DetermineError(fmt.Errorf("failed to remove bastion instance: %w", err), helper.KnownCodes)
	}

	deleted, err := isInstanceDeleted(ctx, log, factory, opt)
	if err != nil {
		return util.DetermineError(fmt.Errorf("failed to check for bastion instance: %w", err), helper.KnownCodes)
	}

	if !deleted {
		return &ctrlerror.RequeueAfterError{
			RequeueAfter: 10 * time.Second,
			Cause:        errors.New("bastion instance is still deleting"),
		}
	}

	err = removeNic(ctx, log, factory, opt)
	if err != nil {
		return util.DetermineError(fmt.Errorf("failed to remove nic: %w", err), helper.KnownCodes)
	}

	err = removePublicIP(ctx, log, factory, opt)
	if err != nil {
		return util.DetermineError(fmt.Errorf("failed to remove public ip: %w", err), helper.KnownCodes)
	}

	err = removeDisk(ctx, log, factory, opt)
	if err != nil {
		return util.DetermineError(fmt.Errorf("failed to remove disk: %w", err), helper.KnownCodes)
	}

	err = removeNSGRule(ctx, log, factory, opt)
	if err != nil {
		return util.DetermineError(fmt.Errorf("failed to remove nsg rules: %w", err), helper.KnownCodes)
	}

	return nil
}

func (a *actuator) ForceDelete(_ context.Context, _ logr.Logger, _ *extensionsv1alpha1.Bastion, _ *controller.Cluster) error {
	return nil
}

func removeNSGRule(ctx context.Context, log logr.Logger, factory azureclient.Factory, opt *Options) error {
	securityGroupResp, err := getNetworkSecurityGroup(ctx, log, factory, opt)
	if err != nil {
		return err
	}

	rules := []string{
		NSGIngressAllowSSHResourceNameIPv4(opt.BastionInstanceName),
		NSGIngressAllowSSHResourceNameIPv6(opt.BastionInstanceName),
		NSGEgressDenyAllResourceName(opt.BastionInstanceName),
		NSGEgressAllowOnlyResourceName(opt.BastionInstanceName),
	}

	modifiedRules, rulesWereDeleted := deleteSecurityRuleDefinitionsByName(securityGroupResp.Properties.SecurityRules, rules...)
	securityGroupResp.Properties.SecurityRules = modifiedRules
	if !rulesWereDeleted {
		return nil
	}

	err = createOrUpdateNetworkSecGroup(ctx, factory, opt, securityGroupResp)
	if err != nil {
		return err
	}

	log.Info("bastion network security group rules removed: ", "rules", rules)
	return nil
}

func removePublicIP(ctx context.Context, log logr.Logger, factory azureclient.Factory, opt *Options) error {
	publicClient, err := factory.PublicIP()
	if err != nil {
		return err
	}

	err = publicClient.Delete(ctx, opt.ResourceGroupName, opt.BastionPublicIPName)
	if err != nil {
		return fmt.Errorf("failed to delete Public IP: %w", err)
	}

	log.Info("Public IP removed", "ip", opt.BastionPublicIPName)
	return nil
}

func removeNic(ctx context.Context, log logr.Logger, factory azureclient.Factory, opt *Options) error {
	nicClient, err := factory.NetworkInterface()
	if err != nil {
		return err
	}

	err = nicClient.Delete(ctx, opt.ResourceGroupName, opt.NicName)
	if err != nil {
		return fmt.Errorf("failed to delete Nic: %w", err)
	}

	log.Info("Nic removed", "nic", opt.NicName)
	return nil
}

func removeDisk(ctx context.Context, log logr.Logger, factory azureclient.Factory, opt *Options) error {
	diskClient, err := factory.Disk()
	if err != nil {
		return err
	}
	err = diskClient.Delete(ctx, opt.ResourceGroupName, opt.DiskName)
	if err != nil {
		return fmt.Errorf("failed to delete disk: %w", err)
	}

	log.Info("Disk removed", "disk", opt.DiskName)
	return nil
}

func removeBastionInstance(ctx context.Context, log logr.Logger, factory azureclient.Factory, opt *Options) error {
	vmClient, err := factory.VirtualMachine()
	if err != nil {
		return err
	}

	if err = vmClient.Delete(ctx, opt.ResourceGroupName, opt.BastionInstanceName, to.BoolPtr(false)); err != nil {
		return fmt.Errorf("failed to terminate bastion instance: %w", err)
	}
	log.Info("Instance removed", "instance", opt.BastionInstanceName)
	return nil
}

func isInstanceDeleted(ctx context.Context, log logr.Logger, factory azureclient.Factory, opt *Options) (bool, error) {
	instance, err := getBastionInstance(ctx, log, factory, opt)
	if err != nil {
		return false, err
	}

	return instance == nil, nil
}
