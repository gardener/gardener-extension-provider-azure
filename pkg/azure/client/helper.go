// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	azerrors "github.com/AzureAD/microsoft-authentication-library-for-go/apps/errors"
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	azuretypes "github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

// FilterNotFoundError returns nil for NotFound errors.
func FilterNotFoundError(err error) error {
	if err == nil {
		return nil
	}
	if IsAzureAPINotFoundError(err) {
		return nil
	}
	return err
}

func isAzureAPIStatusError(err error, status int) bool {
	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) {
		return respErr.StatusCode == status
	}

	cerr := azerrors.CallErr{}
	if errors.As(err, &cerr) {
		return cerr.Resp != nil && cerr.Resp.StatusCode == status
	}

	return false
}

// IsAzureAPINotFoundError tries to determine if an error is a resource not found error.
func IsAzureAPINotFoundError(err error) bool {
	return isAzureAPIStatusError(err, http.StatusNotFound)
}

// IsAzureAPIUnauthorized tries to determine if the API error is due to unauthorized access
func IsAzureAPIUnauthorized(err error) bool {
	if isAzureAPIStatusError(err, http.StatusUnauthorized) {
		return true
	}

	inErr := &azidentity.AuthenticationFailedError{}
	return errors.As(err, &inErr)
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	lString := strings.ToLower(s)
	for _, p := range prefixes {
		if strings.HasPrefix(lString, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// CloudConfiguration returns a CloudConfiguration for the given input, prioritising the given CloudConfiguration if both inputs are not nil. In essence
// this function unifies both ways to configure the instance to connect to into a single type - our CloudConfiguration.
func CloudConfiguration(cloudConfiguration *azure.CloudConfiguration, region *string) (*azure.CloudConfiguration, error) {
	if cloudConfiguration != nil {
		return cloudConfiguration, nil
	} else if region != nil {
		return cloudConfigurationFromRegion(*region), nil
	}
	return nil, fmt.Errorf("either CloudConfiguration or region must not be nil to determine Azure Cloud configuration")
}

// AzureCloudConfiguration is a convenience function to get the corresponding Azure Cloud configuration (from the Azure SDK) to the given input,
// preferring the cloudConfiguration if both values are not nil.
func AzureCloudConfiguration(cloudConfiguration *azure.CloudConfiguration, region *string) (cloud.Configuration, error) {
	cloudConf, err := CloudConfiguration(cloudConfiguration, region)
	if err != nil {
		return cloud.Configuration{}, err
	}
	return AzureCloudConfigurationFromCloudConfiguration(cloudConf)
}

// cloudConfigurationFromRegion returns a matching cloudConfiguration corresponding to a well known cloud instance for the given region
func cloudConfigurationFromSecret(secret *corev1.Secret) (cloud.Configuration, error) {
	if v, ok := secret.Data[azuretypes.AzureCloud]; ok {
		return AzureCloudConfigurationFromCloudConfiguration(&azure.CloudConfiguration{Name: string(v)})
	}
	if v, ok := secret.Data[azuretypes.DNSAzureCloud]; ok {
		return AzureCloudConfigurationFromCloudConfiguration(&azure.CloudConfiguration{Name: string(v)})
	}
	return AzureCloudConfigurationFromCloudConfiguration(nil)
}

// cloudConfigurationFromRegion returns a matching cloudConfiguration corresponding to a well known cloud instance for the given region
func cloudConfigurationFromRegion(region string) *azure.CloudConfiguration {
	switch {
	case hasAnyPrefix(region, azure.AzureGovRegionPrefixes...):
		return &azure.CloudConfiguration{Name: azure.AzureGovCloudName}
	case hasAnyPrefix(region, azure.AzureChinaRegionPrefixes...):
		return &azure.CloudConfiguration{Name: azure.AzureChinaCloudName}
	default:
		return &azure.CloudConfiguration{Name: azure.AzurePublicCloudName}
	}
}

// AzureCloudConfigurationFromCloudConfiguration returns the cloud.Configuration corresponding to the given cloud configuration name (as part of our CloudConfiguration).
func AzureCloudConfigurationFromCloudConfiguration(cloudConfiguration *azure.CloudConfiguration) (cloud.Configuration, error) {
	if cloudConfiguration == nil {
		return cloud.AzurePublic, nil
	}
	cloudConfigurationName := cloudConfiguration.Name
	switch {
	case strings.EqualFold(cloudConfigurationName, azure.AzurePublicCloudName):
		return cloud.AzurePublic, nil
	case strings.EqualFold(cloudConfigurationName, azure.AzureGovCloudName):
		return cloud.AzureGovernment, nil
	case strings.EqualFold(cloudConfigurationName, azure.AzureChinaCloudName):
		return cloud.AzureChina, nil

	default:
		return cloud.Configuration{}, fmt.Errorf("unknown cloud configuration name '%s'", cloudConfigurationName)
	}
}
