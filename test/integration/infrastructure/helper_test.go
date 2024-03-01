// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure_test

import (
	"encoding/base64"
	"flag"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"gopkg.in/yaml.v2"
)

func setConfigVariablesFromFlags() {
	flag.Parse()
	if *secretYamlPath != "" {
		auth := readAuthFromFile(*secretYamlPath)
		clientId = &auth.ClientID
		clientSecret = &auth.ClientSecret
		subscriptionId = &auth.SubscriptionID
		tenantId = &auth.TenantID
	} else {
		validateFlags()
	}
}

func validateFlags() {
	if len(*clientId) == 0 {
		panic("client-id flag is not specified")
	}
	if len(*clientSecret) == 0 {
		panic("client-secret flag is not specified")
	}
	if len(*subscriptionId) == 0 {
		panic("subscription-id flag is not specified")
	}
	if len(*tenantId) == 0 {
		panic("tenant-id flag is not specified")
	}
	if len(*region) == 0 {
		panic("region flag is not specified")
	}
	if len(*reconciler) == 0 {
		reconciler = to.Ptr(reconcilerUseTF)
	}
}

// ClientAuth represents a Azure Client Auth credentials.
type ClientAuth struct {
	// SubscriptionID is the Azure subscription ID.
	SubscriptionID string `yaml:"subscriptionID"`
	// TenantID is the Azure tenant ID.
	TenantID string `yaml:"tenantID"`
	// ClientID is the Azure client ID.
	ClientID string `yaml:"clientID"`
	// ClientSecret is the Azure client secret.
	ClientSecret string `yaml:"clientSecret"`
}

func (clientAuth ClientAuth) GetAzClientCredentials() (*azidentity.ClientSecretCredential, error) {
	return azidentity.NewClientSecretCredential(clientAuth.TenantID, clientAuth.ClientID, clientAuth.ClientSecret, nil)
}

type ProviderSecret struct {
	Data ClientAuth `yaml:"data"`
}

func readAuthFromFile(fileName string) ClientAuth {
	secret := ProviderSecret{}
	data, err := os.ReadFile(fileName)
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(data, &secret)
	if err != nil {
		panic(err)
	}
	secret.Data.ClientID = decodeString(secret.Data.ClientID)
	secret.Data.ClientSecret = decodeString(secret.Data.ClientSecret)
	secret.Data.SubscriptionID = decodeString(secret.Data.SubscriptionID)
	secret.Data.TenantID = decodeString(secret.Data.TenantID)
	return secret.Data
}

func decodeString(s string) string {
	res, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return string(res)
}
