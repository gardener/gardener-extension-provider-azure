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
	"os"
	"strings"
	"time"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func makeCluster(pods, services string, region string, countFaultDomain, countUpdateDomain int32) *controller.Cluster {
	var (
		shoot = gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Networking: gardencorev1beta1.Networking{
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

func getNamedCluster(name string) *controller.Cluster {
	return &controller.Cluster{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

type cmYaml struct {
	APIVersion string `yaml:"apiVersion"`
	Data       struct {
		TerraformTfstate string `yaml:"terraform.tfstate"`
	} `yaml:"data"`
	Kind     string `yaml:"kind"`
	Metadata struct {
		CreationTimestamp time.Time `yaml:"creationTimestamp"`
		Finalizers        []string  `yaml:"finalizers"`
		Name              string    `yaml:"name"`
		Namespace         string    `yaml:"namespace"`
		OwnerReferences   []struct {
			APIVersion         string `yaml:"apiVersion"`
			BlockOwnerDeletion bool   `yaml:"blockOwnerDeletion"`
			Controller         bool   `yaml:"controller"`
			Kind               string `yaml:"kind"`
			Name               string `yaml:"name"`
			UID                string `yaml:"uid"`
		} `yaml:"ownerReferences"`
		ResourceVersion string `yaml:"resourceVersion"`
		UID             string `yaml:"uid"`
	} `yaml:"metadata"`
}

func readTfRawStateFromFile(yamlFile string) (terraformer.RawState, error) {
	bytes, err := os.ReadFile(yamlFile)
	if err != nil {
		return terraformer.RawState{}, err
	}

	cfg := cmYaml{}
	err = yaml.Unmarshal(bytes, &cfg)
	if err != nil {
		return terraformer.RawState{}, err
	}
	rawState := terraformer.RawState{
		Data:     cfg.Data.TerraformTfstate,
		Encoding: terraformer.NoneEncoding,
	}
	return rawState, err
}

// IsShootResourceGroupAvailable determines if the managed resource group exists on Azure.
func IsShootResourceGroupAvailable(ctx context.Context, factory azureclient.Factory, infra *extensionsv1alpha1.Infrastructure, infraConfig *api.InfrastructureConfig) (bool, error) {
	if infraConfig.ResourceGroup != nil {
		return true, nil
	}

	groupClient, err := factory.Group(ctx, infra.Spec.SecretRef)
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

	subnetClient, err := factory.Subnet(ctx, infra.Spec.SecretRef)
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
