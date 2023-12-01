// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

// MakeCluster returns a cluster object used for testing.
func MakeCluster(pods, services string, region string, countFaultDomain, countUpdateDomain int32) *controller.Cluster {
	var (
		shoot = gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Networking: &gardencorev1beta1.Networking{
					Pods:     &pods,
					Services: &services,
				},
			},
		}
		cloudProfileConfig = apiv1alpha1.CloudProfileConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
				Kind:       "CloudProfileConfig",
			},
			CountFaultDomains: []apiv1alpha1.DomainCount{
				{Region: region, Count: countFaultDomain},
			},
			CountUpdateDomains: []apiv1alpha1.DomainCount{
				{Region: region, Count: countUpdateDomain},
			},
		}
		cloudProfileConfigJSON, _ = json.Marshal(cloudProfileConfig)
		cloudProfile              = gardencorev1beta1.CloudProfile{
			Spec: gardencorev1beta1.CloudProfileSpec{
				ProviderConfig: &runtime.RawExtension{
					Raw: cloudProfileConfigJSON,
				},
			},
		}
	)

	return &controller.Cluster{
		Shoot:        &shoot,
		CloudProfile: &cloudProfile,
	}
}

// IsShootResourceGroupAvailable determines if the managed resource group exists on Azure.
func IsShootResourceGroupAvailable(ctx context.Context, factory azureclient.Factory, infra *extensionsv1alpha1.Infrastructure, infraConfig *api.InfrastructureConfig) (bool, error) {
	if infraConfig.ResourceGroup != nil {
		return true, nil
	}

	groupClient, err := factory.Group()
	if err != nil {
		return false, err
	}

	resourceGroup, err := groupClient.Get(ctx, infra.Namespace)
	if err != nil {
		return false, err
	}

	if resourceGroup == nil {
		return false, nil
	}

	return true, nil
}

// DeleteNodeSubnetIfExists will delete the nodes subnet(s) if exists.
func DeleteNodeSubnetIfExists(ctx context.Context, factory azureclient.Factory, infra *extensionsv1alpha1.Infrastructure, infraConfig *api.InfrastructureConfig) error {
	if infraConfig.Networks.VNet.ResourceGroup == nil || infraConfig.Networks.VNet.Name == nil {
		return nil
	}

	subnetClient, err := factory.Subnet()
	if err != nil {
		return err
	}

	subnets, err := subnetClient.List(ctx, *infraConfig.Networks.VNet.ResourceGroup, *infraConfig.Networks.VNet.Name)
	if err != nil {
		return err
	}

	subnetNamePrefix := fmt.Sprintf("%s-nodes", infra.Namespace)
	for _, subnet := range subnets {
		if !strings.HasPrefix(*subnet.Name, subnetNamePrefix) {
			continue
		}

		if err := subnetClient.Delete(ctx, *infraConfig.Networks.VNet.ResourceGroup, *infraConfig.Networks.VNet.Name, *subnet.Name); err != nil {
			return err
		}
	}

	return nil
}
