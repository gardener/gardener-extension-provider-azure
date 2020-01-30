// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package charts

import (
	"encoding/json"
	"fmt"

	calicov1alpha1 "github.com/gardener/gardener-extension-networking-calico/pkg/apis/calico/v1alpha1"
	"github.com/gardener/gardener-extension-networking-calico/pkg/calico"
	"github.com/gardener/gardener-extension-networking-calico/pkg/imagevector"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

const (
	hostLocal  = "host-local"
	usePodCIDR = "usePodCidr"
	defaultMTU = "1440"
)

type calicoConfig struct {
	Backend calicov1alpha1.Backend `json:"backend"`
	Felix   felix                  `json:"felix"`
	IPv4    ipv4                   `json:"ipv4"`
	IPAM    ipam                   `json:"ipam"`
	Typha   typha                  `json:"typha"`
	VethMTU string                 `json:"veth_mtu"`
}

type felix struct {
	IPInIP felixIPinIP `json:"ipinip"`
}

type felixIPinIP struct {
	Enabled bool `json:"enabled"`
}

type ipv4 struct {
	Pool                calicov1alpha1.IPv4Pool     `json:"pool"`
	Mode                calicov1alpha1.IPv4PoolMode `json:"mode"`
	AutoDetectionMethod *string                     `json:"autoDetectionMethod"`
}

type ipam struct {
	IPAMType string `json:"type"`
	Subnet   string `json:"subnet"`
}

type typha struct {
	Enabled bool `json:"enabled"`
}

var defaultCalicoConfig = calicoConfig{
	Backend: calicov1alpha1.Bird,
	Felix: felix{
		IPInIP: felixIPinIP{
			Enabled: true,
		},
	},
	IPv4: ipv4{
		Pool:                calicov1alpha1.PoolIPIP,
		Mode:                calicov1alpha1.Always,
		AutoDetectionMethod: nil,
	},
	IPAM: ipam{
		IPAMType: hostLocal,
		Subnet:   usePodCIDR,
	},
	Typha: typha{
		Enabled: true,
	},
	VethMTU: defaultMTU,
}

func newCalicoConfig() calicoConfig {
	return defaultCalicoConfig
}

func (c *calicoConfig) toMap() (map[string]interface{}, error) {
	bytes, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("could not marshal calico config: %v", err)
	}
	var configMap map[string]interface{}
	err = json.Unmarshal(bytes, &configMap)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal calico config: %v", err)
	}
	return configMap, nil
}

// ComputeCalicoChartValues computes the values for the calico chart.
func ComputeCalicoChartValues(network *extensionsv1alpha1.Network, config *calicov1alpha1.NetworkConfig) (map[string]interface{}, error) {
	typedConfig, err := generateChartValues(config)
	if err != nil {
		return nil, fmt.Errorf("error when generating calico config: %v", err)
	}
	calicoConfig, err := typedConfig.toMap()
	if err != nil {
		return nil, fmt.Errorf("could not convert calico config: %v", err)
	}
	calicoChartValues := map[string]interface{}{
		"images": map[string]interface{}{
			calico.CNIImageName:                                        imagevector.CalicoCNIImage(),
			calico.TyphaImageName:                                      imagevector.CalicoTyphaImage(),
			calico.KubeControllersImageName:                            imagevector.CalicoKubeControllersImage(),
			calico.NodeImageName:                                       imagevector.CalicoNodeImage(),
			calico.PodToDaemonFlexVolumeDriverImageName:                imagevector.CalicoFlexVolumeDriverImage(),
			calico.TyphaClusterProportionalAutoscalerImageName:         imagevector.TyphaClusterProportionalAutoscalerImage(),
			calico.TyphaClusterProportionalVerticalAutoscalerImageName: imagevector.TyphaClusterProportionalVerticalAutoscalerImage(),
		},
		"global": map[string]string{
			"podCIDR": network.Spec.PodCIDR,
		},
		"config": calicoConfig,
	}
	return calicoChartValues, nil
}

func generateChartValues(config *calicov1alpha1.NetworkConfig) (*calicoConfig, error) {
	c := newCalicoConfig()
	if config == nil {
		return &c, nil
	}

	if config.Backend != nil {
		switch *config.Backend {
		case calicov1alpha1.Bird, calicov1alpha1.VXLan, calicov1alpha1.None:
			c.Backend = *config.Backend
		default:
			return nil, fmt.Errorf("unsupported value for backend: %s", *config.Backend)
		}
	}
	if c.Backend == calicov1alpha1.None {
		c.Felix.IPInIP.Enabled = false
		c.IPv4.Mode = calicov1alpha1.Never
	}

	if config.IPAM != nil {
		if config.IPAM.Type != "" {
			c.IPAM.IPAMType = config.IPAM.Type
		}
		if config.IPAM.Type == hostLocal && config.IPAM.CIDR != nil {
			c.IPAM.Subnet = string(*config.IPAM.CIDR)
		}
	}

	if config.IPv4 != nil {
		if config.IPv4.Pool != nil {
			switch *config.IPv4.Pool {
			case calicov1alpha1.PoolIPIP, calicov1alpha1.PoolVXLan:
				c.IPv4.Pool = *config.IPv4.Pool
			default:
				return nil, fmt.Errorf("unsupported value for ipv4 pool: %s", *config.IPv4.Pool)
			}
		}
		if config.IPv4.Mode != nil {
			switch *config.IPv4.Mode {
			case calicov1alpha1.Always, calicov1alpha1.Never, calicov1alpha1.Off, calicov1alpha1.CrossSubnet:
				c.IPv4.Mode = *config.IPv4.Mode
			default:
				return nil, fmt.Errorf("unsupported value for ipv4 mode: %s", *config.IPv4.Mode)
			}
		}
		if config.IPv4.AutoDetectionMethod != nil {
			c.IPv4.AutoDetectionMethod = config.IPv4.AutoDetectionMethod
		}
	} else {
		// fallback to deprecated configuration fields
		// will be removed in a future Gardener release
		if config.IPIP != nil {
			switch *config.IPIP {
			case calicov1alpha1.Always, calicov1alpha1.Never, calicov1alpha1.Off, calicov1alpha1.CrossSubnet:
				c.IPv4.Mode = *config.IPIP
			default:
				return nil, fmt.Errorf("unsupported value for ipip: %s", *config.IPIP)
			}
		}
		if config.IPAutoDetectionMethod != nil {
			c.IPv4.AutoDetectionMethod = config.IPAutoDetectionMethod
		}
	}

	if config.Typha != nil {
		c.Typha.Enabled = config.Typha.Enabled
	}

	if config.VethMTU != nil {
		c.VethMTU = *config.VethMTU
	}

	return &c, nil
}
