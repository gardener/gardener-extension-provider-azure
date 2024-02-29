// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// package contains the generators for provider specific shoot configuration
package main

import (
	"flag"
	"fmt"
	"os"
	"reflect"
	"strconv"

	"github.com/gardener/gardener/test/testmachinery/extensions/generator"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
)

const (
	defaultNetworkVnetCIDR   = "10.250.0.0/16"
	defaultNetworkWorkerCidr = "10.250.0.0/19"
)

type generatorConfig struct {
	networkWorkerCidr                string
	networkVnetCidr                  string
	infrastructureProviderConfigPath string
	controlplaneProviderConfigPath   string
	zonedFlag                        string
	zoned                            bool
}

var (
	cfg    *generatorConfig
	logger logr.Logger
)

func addFlags() {
	cfg = &generatorConfig{}
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
		return fmt.Errorf("error validating infrastructure provider config path: %w", err)
	}
	if err := generator.ValidateString(&cfg.controlplaneProviderConfigPath); err != nil {
		return fmt.Errorf("error validating controlplane provider config path: %w", err)
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
			return fmt.Errorf("zoned is not a boolean value: %w", err)
		}
		cfg.zoned = parsedBool
	}
	return nil
}
