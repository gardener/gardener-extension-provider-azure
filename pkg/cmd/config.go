// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	apisconfigv1alpha1 "github.com/gardener/gardener/extensions/pkg/apis/config/v1alpha1"
	"github.com/spf13/pflag"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/config"
	configloader "github.com/gardener/gardener-extension-provider-azure/pkg/apis/config/loader"
)

// ConfigOptions are command line options that can be set for config.ControllerConfiguration.
type ConfigOptions struct {
	// Kubeconfig is the path to a kubeconfig.
	ConfigFilePath string

	config *Config
}

// Config is a completed controller configuration.
type Config struct {
	// Config is the controller configuration.
	Config *config.ControllerConfiguration
}

func (c *ConfigOptions) buildConfig() (*config.ControllerConfiguration, error) {
	if len(c.ConfigFilePath) == 0 {
		return nil, fmt.Errorf("config file path not set")
	}
	return configloader.LoadFromFile(c.ConfigFilePath)
}

// Complete implements RESTCompleter.Complete.
func (c *ConfigOptions) Complete() error {
	config, err := c.buildConfig()
	if err != nil {
		return err
	}

	c.config = &Config{config}
	return nil
}

// Completed returns the completed Config. Only call this if `Complete` was successful.
func (c *ConfigOptions) Completed() *Config {
	return c.config
}

// AddFlags implements Flagger.AddFlags.
func (c *ConfigOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.ConfigFilePath, "config-file", "", "path to the controller manager configuration file")
}

// Apply sets the values of this Config in the given config.ControllerConfiguration.
func (c *Config) Apply(cfg *config.ControllerConfiguration) {
	*cfg = *c.Config
}

// ApplyETCDStorage sets the given etcd storage configuration to that of this Config.
func (c *Config) ApplyETCDStorage(etcdStorage *config.ETCDStorage) {
	*etcdStorage = c.Config.ETCD.Storage
}

// ApplyETCDBackup sets the given etcd backup configuration to that of this Config.
func (c *Config) ApplyETCDBackup(etcdBackup *config.ETCDBackup) {
	*etcdBackup = c.Config.ETCD.Backup
}

// Options initializes empty config.ControllerConfiguration, applies the set values and returns it.
func (c *Config) Options() config.ControllerConfiguration {
	var cfg config.ControllerConfiguration
	c.Apply(&cfg)
	return cfg
}

// ApplyHealthCheckConfig applies the HealthCheckConfig to the config
func (c *Config) ApplyHealthCheckConfig(config *apisconfigv1alpha1.HealthCheckConfig) {
	if c.Config.HealthCheckConfig != nil {
		*config = *c.Config.HealthCheckConfig
	}
}

// SeedConfig is a completed configuration for the topology webhook.
type SeedConfig struct {
	Region   string
	Provider string
}

// SeedConfigOptions are command line options for the topology webhook.
type SeedConfigOptions struct {
	Region   string
	Provider string

	config *SeedConfig
}

// AddFlags implements Flagger.AddFlags.
func (s *SeedConfigOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.Region, "seed-region", "", "the region of the seed")
	fs.StringVar(&s.Provider, "seed-provider", "", "the provider of the seed")
}

// Complete implements RESTCompleter.Complete.
func (s *SeedConfigOptions) Complete() error {
	s.config = &SeedConfig{
		Region:   s.Region,
		Provider: s.Provider,
	}
	return nil
}

// Completed returns the completed SeedConfig. Only call this if `Complete` was successful.
func (s *SeedConfigOptions) Completed() *SeedConfig {
	return s.config
}
