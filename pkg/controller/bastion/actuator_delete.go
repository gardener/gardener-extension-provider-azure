// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	ctrlerror "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/go-logr/logr"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

func (a *actuator) Delete(ctx context.Context, log logr.Logger, bastion *extensionsv1alpha1.Bastion, cluster *controller.Cluster) error {
	infrastructureStatus, err := getInfrastructureStatus(ctx, a, cluster)
	if err != nil {
		return err
	}

	opts, err := NewBaseOpts(bastion, cluster, infrastructureStatus.ResourceGroup.Name, log)
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

	azCloudConfiguration, err := azureclient.AzureCloudConfiguration(cloudConfiguration, &cluster.Shoot.Spec.Region)

	if err != nil {
		return err
	}

	factory, err := azureclient.NewAzureClientFactoryFromSecret(
		ctx,
		a.client,
		opts.SecretReference,
		false,
		azureclient.WithCloudConfiguration(azCloudConfiguration),
	)
	if err != nil {
		return err
	}

	err = removeBastionInstance(ctx, factory, opts)
	if err != nil {
		return util.DetermineError(fmt.Errorf("failed to remove bastion instance: %w", err), helper.KnownCodes)
	}

	deleted, err := isInstanceDeleted(ctx, factory, opts)
	if err != nil {
		return util.DetermineError(fmt.Errorf("failed to check for bastion instance: %w", err), helper.KnownCodes)
	}

	if !deleted {
		return &ctrlerror.RequeueAfterError{
			RequeueAfter: 10 * time.Second,
			Cause:        errors.New("bastion instance is still deleting"),
		}
	}

	err = removeNic(ctx, factory, opts)
	if err != nil {
		return util.DetermineError(fmt.Errorf("failed to remove nic: %w", err), helper.KnownCodes)
	}

	err = removePublicIP(ctx, factory, opts)
	if err != nil {
		return util.DetermineError(fmt.Errorf("failed to remove public ip: %w", err), helper.KnownCodes)
	}

	err = removeDisk(ctx, factory, opts)
	if err != nil {
		return util.DetermineError(fmt.Errorf("failed to remove disk: %w", err), helper.KnownCodes)
	}

	err = removeNSGRule(ctx, factory, opts)
	if err != nil {
		return util.DetermineError(fmt.Errorf("failed to remove nsg rules: %w", err), helper.KnownCodes)
	}

	return nil
}

func (a *actuator) ForceDelete(_ context.Context, _ logr.Logger, _ *extensionsv1alpha1.Bastion, _ *controller.Cluster) error {
	return nil
}

func removeNSGRule(ctx context.Context, factory azureclient.Factory, opts BaseOptions) error {
	securityGroupResp, err := getNetworkSecurityGroup(ctx, factory, opts)
	if err != nil {
		return err
	}

	rules := []string{
		NSGIngressAllowSSHResourceNameIPv4(opts.BastionInstanceName),
		NSGIngressAllowSSHResourceNameIPv6(opts.BastionInstanceName),
		NSGEgressDenyAllResourceName(opts.BastionInstanceName),
		NSGEgressAllowOnlyResourceName(opts.BastionInstanceName),
	}

	modifiedRules, rulesWereDeleted := deleteSecurityRuleDefinitionsByName(securityGroupResp.Properties.SecurityRules, rules...)
	securityGroupResp.Properties.SecurityRules = modifiedRules
	if !rulesWereDeleted {
		return nil
	}

	err = createOrUpdateNetworkSecGroup(ctx, factory, opts, securityGroupResp)
	if err != nil {
		return err
	}

	opts.Logr.Info("bastion network security group rules removed: ", "rules", rules)
	return nil
}

func removePublicIP(ctx context.Context, factory azureclient.Factory, opts BaseOptions) error {
	publicClient, err := factory.PublicIP()
	if err != nil {
		return err
	}

	err = publicClient.Delete(ctx, opts.ResourceGroupName, opts.PublicIPName)
	if err != nil {
		return fmt.Errorf("failed to delete Public IP: %w", err)
	}

	opts.Logr.Info("Public IP removed", "ip", opts.PublicIPName)
	return nil
}

func removeNic(ctx context.Context, factory azureclient.Factory, opts BaseOptions) error {
	nicClient, err := factory.NetworkInterface()
	if err != nil {
		return err
	}

	err = nicClient.Delete(ctx, opts.ResourceGroupName, opts.NicName)
	if err != nil {
		return fmt.Errorf("failed to delete Nic: %w", err)
	}

	opts.Logr.Info("Nic removed", "nic", opts.NicName)
	return nil
}

func removeDisk(ctx context.Context, factory azureclient.Factory, opts BaseOptions) error {
	diskClient, err := factory.Disk()
	if err != nil {
		return err
	}
	err = diskClient.Delete(ctx, opts.ResourceGroupName, opts.DiskName)
	if err != nil {
		return fmt.Errorf("failed to delete disk: %w", err)
	}

	opts.Logr.Info("Disk removed", "disk", opts.DiskName)
	return nil
}

func removeBastionInstance(ctx context.Context, factory azureclient.Factory, opts BaseOptions) error {
	vmClient, err := factory.VirtualMachine()
	if err != nil {
		return err
	}

	if err = vmClient.Delete(ctx, opts.ResourceGroupName, opts.BastionInstanceName, ptr.To(false)); err != nil {
		return fmt.Errorf("failed to terminate bastion instance: %w", err)
	}
	opts.Logr.Info("Instance removed", "instance", opts.BastionInstanceName)
	return nil
}

func isInstanceDeleted(ctx context.Context, factory azureclient.Factory, opts BaseOptions) (bool, error) {
	instance, err := getBastionInstance(ctx, factory, opts)
	if err != nil {
		return false, err
	}

	return instance == nil, nil
}
