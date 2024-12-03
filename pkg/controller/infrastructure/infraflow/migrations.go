// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"
	"errors"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"

	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow/shared"
)

// BackupPIPsForBasicLBMigration saves the Public IPs attached to the basic load balancer of the shoot to the state.
func (fctx *FlowContext) BackupPIPsForBasicLBMigration(ctx context.Context) error {
	loadbalancerClient, err := fctx.factory.LoadBalancer()
	if err != nil {
		return err
	}
	lb, err := loadbalancerClient.Get(ctx, fctx.adapter.ResourceGroupName(), fctx.infra.Namespace)
	if err != nil {
		return err
	}
	// if we don't find the loadbalancer or some properties are missing, exit early
	if lb == nil ||
		lb.SKU == nil ||
		lb.SKU.Name == nil ||
		*lb.SKU.Name != armnetwork.LoadBalancerSKUNameBasic ||
		lb.Properties == nil ||
		len(lb.Properties.FrontendIPConfigurations) == 0 {
		return nil
	}
	for _, fipc := range lb.Properties.FrontendIPConfigurations {
		if fipc.Properties == nil {
			continue
		}
		if fipc.Properties.PublicIPAddress != nil {
			resourceID, err := arm.ParseResourceID(*fipc.Properties.PublicIPAddress.ID)
			if err != nil {
				return err
			}
			fctx.whiteboard.GetChild("migration").GetChild("basic-lb").GetChild(resourceID.ResourceGroupName).Set(resourceID.Name, "true")
		}
	}
	return fctx.PersistState(ctx)
}

// UpdatePublicIPs updates the public IPs saved in the state from basic to standard SKU.
func (fctx *FlowContext) UpdatePublicIPs(ctx context.Context) error {
	log := shared.LogFromContext(ctx)
	ipc, err := fctx.factory.PublicIP()
	if err != nil {
		return err
	}
	var (
		wg   = sync.WaitGroup{}
		errs error
	)
	for _, resourceGroup := range fctx.whiteboard.GetChild("migration").GetChild("basic-lb").GetChildrenKeys() {
		for _, pipName := range fctx.whiteboard.GetChild("migration").GetChild("basic-lb").GetChild(resourceGroup).Keys() {
			wg.Add(1)
			go func() {
				defer wg.Done()
				pip, err := ipc.Get(ctx, resourceGroup, pipName, nil)
				if err != nil {
					errs = errors.Join(err)
					return
				}
				if pip == nil || pip.SKU == nil || pip.SKU.Name == nil || *pip.SKU.Name != armnetwork.PublicIPAddressSKUNameBasic {
					return
				}
				pip.SKU = &armnetwork.PublicIPAddressSKU{
					Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard),
					Tier: to.Ptr(armnetwork.PublicIPAddressSKUTierRegional),
				}
				log.Info("upgrading basic PIP", "ResourceGroup", resourceGroup, "Name", pipName)
				_, err = ipc.CreateOrUpdate(ctx, resourceGroup, pipName, *pip)
				if err != nil {
					errs = errors.Join(err)
					return
				}
				// removing upgraded PIP from state.
				fctx.whiteboard.GetChild("migration").GetChild("basic-lb").GetChild(resourceGroup).Delete(pipName)
			}()
		}
	}
	wg.Wait()
	return errs
}
