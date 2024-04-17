// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"crypto/tls"
	"fmt"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
)

const (
	// DefaultMaxRetries is the default value for max retries on retryable operations.
	DefaultMaxRetries = 3
	// DefaultMaxRetryDelay is the default maximum value for delay on retryable operations.
	DefaultMaxRetryDelay = math.MaxInt64
	// DefaultRetryDelay is the default value for the initial delay on retry for retryable operations.
	DefaultRetryDelay = 5 * time.Second
)

var (
	// DefaultAzureClientOpts generates clientOptions for the azure clients.
	DefaultAzureClientOpts func() *arm.ClientOptions
	once                   sync.Once
)

func init() {
	once.Do(func() {
		DefaultAzureClientOpts = getAzureClientOpts
	})
}

func getTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        100,
		MaxConnsPerHost:     100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
}
func getAzureClientOpts() *arm.ClientOptions {
	return &arm.ClientOptions{
		ClientOptions: policy.ClientOptions{
			Retry: policy.RetryOptions{
				RetryDelay:    DefaultRetryDelay,
				MaxRetryDelay: DefaultMaxRetryDelay,
				MaxRetries:    DefaultMaxRetries,
				StatusCodes:   getRetriableStatusCode(),
			},
			Transport: &http.Client{
				Transport: getTransport(),
			},
			Cloud: cloud.AzurePublic,
		},
	}
}

// AzureCloudConfigurationFromCloudConfiguration returns the cloud.Configuration corresponding to the given cloud configuration name (as part of our CloudConfiguration)
func AzureCloudConfigurationFromCloudConfiguration(cloudConfiguration *azure.CloudConfiguration) (cloud.Configuration, error) {
	if cloudConfiguration == nil {
		return cloud.AzurePublic, nil
	}

	cloudConfigurationName := cloudConfiguration.Name
	switch {
	case strings.EqualFold(cloudConfigurationName, "AzurePublic"):
		return cloud.AzurePublic, nil
	case strings.EqualFold(cloudConfigurationName, "AzureGovernment"):
		return cloud.AzureGovernment, nil
	case strings.EqualFold(cloudConfigurationName, "AzureChina"):
		return cloud.AzureChina, nil

	default:
		return cloud.Configuration{}, fmt.Errorf("unknown cloud configuration name '%s'", cloudConfigurationName)
	}
}

// CloudEnvVarNameFromCloudConfiguration returns the names of env-vars used by the upstream-controllers corresponding to the given cloud configuration name (as part of our CloudConfiguration)
func CloudEnvVarNameFromCloudConfiguration(cloudConfiguration *azure.CloudConfiguration) (string, error) {
	if cloudConfiguration == nil {
		return "AZUREPUBLICCLOUD", nil
	}

	cloudConfigurationName := cloudConfiguration.Name
	switch {
	case strings.EqualFold(cloudConfigurationName, "AzurePublic"):
		return "AZUREPUBLICCLOUD", nil
	case strings.EqualFold(cloudConfigurationName, "AzureGovernment"):
		return "AZUREUSGOVERNMENT", nil
	case strings.EqualFold(cloudConfigurationName, "AzureChina"):
		return "AZURECHINACLOUD", nil
	default:
		return "", fmt.Errorf("unknown cloud configuration name '%s'", cloudConfigurationName)
	}
}

func getRetriableStatusCode() []int {
	return []int{
		http.StatusRequestTimeout,      // 408
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout,      // 504
	}
}
