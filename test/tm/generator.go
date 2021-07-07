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

// package contains the generators for provider specific shoot configuration
package main

import (
	"flag"
	"os"
	"reflect"
	"strconv"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"

	"github.com/gardener/gardener/extensions/test/tm/generator"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	defaultNetworkVnetCIDR   = "10.250.0.0/16"
	defaultNetworkWorkerCidr = "10.250.0.0/19"
)

type GeneratorConfig struct {
	networkWorkerCidr                string
	networkVnetCidr                  string
	infrastructureProviderConfigPath string
	controlplaneProviderConfigPath   string
	zonedFlag                        string
	zoned                            bool
}

var (
	cfg    *GeneratorConfig
	logger logr.Logger
)

func addFlags() {
	cfg = &GeneratorConfig{}
	flag.StringVar(&cfg.infrastructureProviderConfigPath, "infrastructure-provider-config-filepath", "", "filepath to the provider specific infrastructure config")
	flag.StringVar(&cfg.controlplaneProviderConfigPath, "controlplane-provider-config-filepath", "", "filepath to the provider specific controlplane config")

	flag.StringVar(&cfg.networkVnetCidr, "network-vnet-cidr", "", "vnet network cidr")
	flag.StringVar(&cfg.networkWorkerCidr, "network-worker-cidr", "", "worker network cidr")

	flag.StringVar(&cfg.zonedFlag, "zoned", "", "shoot uses multiple zones")
}

func main() {
	addFlags()
	flag.Parse()
	log.SetLogger(zap.New(zap.UseDevMode(false)))
	logger = log.Log.WithName("azure-generator")
	if err := validate(); err != nil {
		logger.Error(err, "error validating input flags")
		os.Exit(1)
	}

	infra := v1alpha1.InfrastructureConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       reflect.TypeOf(v1alpha1.InfrastructureConfig{}).Name(),
		},
		Networks: v1alpha1.NetworkConfig{
			VNet: v1alpha1.VNet{
				CIDR: &cfg.networkVnetCidr,
			},
			Workers: &cfg.networkWorkerCidr,
		},
		Zoned: cfg.zoned,
	}

	cp := v1alpha1.ControlPlaneConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       reflect.TypeOf(v1alpha1.ControlPlaneConfig{}).Name(),
		},
	}

	if err := generator.MarshalAndWriteConfig(cfg.infrastructureProviderConfigPath, infra); err != nil {
		logger.Error(err, "unable to write infrastructure config")
		os.Exit(1)
	}
	if err := generator.MarshalAndWriteConfig(cfg.controlplaneProviderConfigPath, cp); err != nil {
		logger.Error(err, "unable to write infrastructure config")
		os.Exit(1)
	}
	logger.Info("successfully written azure provider configuration", "infra", cfg.infrastructureProviderConfigPath, "controlplane", cfg.controlplaneProviderConfigPath)
}

func validate() error {
	if err := generator.ValidateString(&cfg.infrastructureProviderConfigPath); err != nil {
		return errors.Wrap(err, "error validating infrastructure provider config path")
	}
	if err := generator.ValidateString(&cfg.controlplaneProviderConfigPath); err != nil {
		return errors.Wrap(err, "error validating controlplane provider config path")
	}
	//Optional Parameters
	if err := generator.ValidateString(&cfg.networkVnetCidr); err != nil {
		logger.Info("Parameter network-vnet-cidr is not set, using default.", "value", defaultNetworkVnetCIDR)
		cfg.networkVnetCidr = defaultNetworkVnetCIDR
	}
	if err := generator.ValidateString(&cfg.networkWorkerCidr); err != nil {
		logger.Info("Parameter network-worker-cidr is not set, using default.", "value", defaultNetworkWorkerCidr)
		cfg.networkWorkerCidr = defaultNetworkWorkerCidr
	}
	if err := generator.ValidateString(&cfg.zonedFlag); err == nil {
		parsedBool, err := strconv.ParseBool(cfg.zonedFlag)
		if err != nil {
			return errors.Wrap(err, "zoned is not a boolean value")
		}
		cfg.zoned = parsedBool
	}
	return nil
}
