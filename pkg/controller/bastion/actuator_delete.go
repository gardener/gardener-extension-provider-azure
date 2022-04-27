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
	"time"

	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	ctrlerror "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (a *actuator) Delete(ctx context.Context, bastion *extensionsv1alpha1.Bastion, cluster *controller.Cluster) error {
	logger := a.logger.WithValues("bastion", client.ObjectKeyFromObject(bastion), "operation", "delete")

	var factory = azureclient.NewAzureClientFactory(a.client)

	infrastructureStatus, err := getInfrastructureStatus(ctx, a, cluster)
	if err != nil {
		return err
	}

	opt, err := DetermineOptions(bastion, cluster, infrastructureStatus.ResourceGroup.Name)
	if err != nil {
		return err
	}

	err = removeBastionInstance(ctx, factory, logger, opt)
	if err != nil {
		return fmt.Errorf("failed to remove bastion instance: %w", err)
	}

	deleted, err := isInstanceDeleted(ctx, factory, opt)
	if err != nil {
		return fmt.Errorf("failed to check for bastion instance: %w", err)
	}

	if !deleted {
		return &ctrlerror.RequeueAfterError{
			RequeueAfter: 10 * time.Second,
			Cause:        errors.New("bastion instance is still deleting"),
		}
	}

	err = removeNic(ctx, factory, logger, opt)
	if err != nil {
		return fmt.Errorf("failed to remove nic: %w", err)
	}

	err = removePublicIP(ctx, factory, logger, opt)
	if err != nil {
		return fmt.Errorf("failed to remove public ip: %w", err)
	}

	err = removeDisk(ctx, factory, logger, opt)
	if err != nil {
		return fmt.Errorf("failed to remove disk: %w", err)
	}

	err = removeNSGRule(ctx, factory, logger, opt)
	if err != nil {
		return fmt.Errorf("failed to remove nsg rules: %w", err)
	}

	return nil
}

func removeNSGRule(ctx context.Context, factory azureclient.Factory, logger logr.Logger, opt *Options) error {
	securityGroupResp, err := getNetworkSecurityGroup(ctx, factory, opt)
	if err != nil {
		return err
	}

	rules := []string{
		NSGIngressAllowSSHResourceNameIPv4(opt.BastionInstanceName),
		NSGIngressAllowSSHResourceNameIPv6(opt.BastionInstanceName),
		NSGEgressDenyAllResourceName(opt.BastionInstanceName),
		NSGEgressAllowOnlyResourceName(opt.BastionInstanceName),
	}

	rulesWereDeleted := deleteSecurityRuleDefinitionsByName(securityGroupResp.SecurityRules, rules...)
	if !rulesWereDeleted {
		return nil
	}

	err = createOrUpdateNetworkSecGroup(ctx, factory, opt, securityGroupResp)
	if err != nil {
		return err
	}

	logger.Info("bastion network security group rules removed: ", "rules", rules)
	return nil
}

func removePublicIP(ctx context.Context, factory azureclient.Factory, logger logr.Logger, opt *Options) error {
	publicClient, err := factory.PublicIP(ctx, opt.SecretReference)
	if err != nil {
		return err
	}

	err = publicClient.Delete(ctx, opt.ResourceGroupName, opt.BastionPublicIPName)
	if err != nil {
		return fmt.Errorf("failed to delete Public IP: %w", err)
	}

	logger.Info("Public IP removed", "ip", opt.BastionPublicIPName)
	return nil
}

func removeNic(ctx context.Context, factory azureclient.Factory, logger logr.Logger, opt *Options) error {
	nicClient, err := factory.NetworkInterface(ctx, opt.SecretReference)
	if err != nil {
		return err
	}

	err = nicClient.Delete(ctx, opt.ResourceGroupName, opt.NicName)
	if err != nil {
		return fmt.Errorf("failed to delete Nic: %w", err)
	}

	logger.Info("Nic removed", "nic", opt.NicName)
	return nil
}

func removeDisk(ctx context.Context, factory azureclient.Factory, logger logr.Logger, opt *Options) error {
	diskClient, err := factory.Disk(ctx, opt.SecretReference)
	if err != nil {
		return err
	}
	err = diskClient.Delete(ctx, opt.ResourceGroupName, opt.DiskName)
	if err != nil {
		return fmt.Errorf("failed to delete disk: %w", err)
	}

	logger.Info("Disk removed", "disk", opt.DiskName)
	return nil
}

func removeBastionInstance(ctx context.Context, factory azureclient.Factory, logger logr.Logger, opt *Options) error {
	vmClient, err := factory.VirtualMachine(ctx, opt.SecretReference)
	if err != nil {
		return err
	}

	if err = vmClient.Delete(ctx, opt.ResourceGroupName, opt.BastionInstanceName, to.BoolPtr(false)); err != nil {
		return fmt.Errorf("failed to terminate bastion instance: %w", err)
	}
	logger.Info("Instance removed", "instance", opt.BastionInstanceName)
	return nil
}

func isInstanceDeleted(ctx context.Context, factory azureclient.Factory, opt *Options) (bool, error) {
	instance, err := getBastionInstance(ctx, factory, opt)
	if err != nil {
		return false, err
	}

	return instance == nil, nil
}
