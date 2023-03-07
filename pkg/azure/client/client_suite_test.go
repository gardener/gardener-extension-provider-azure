// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package client_test

import (
	"context"
	"encoding/base64"
	"flag"
	"os"
	"testing"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

func TestWorker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Client Suite")
}

// HOW TO RUN: ginkgo --  --secret-path=""
var (
	secretYamlPath = flag.String("secret-path", "", "Yaml file with secret including Azure credentials")
)

var _ = Describe("client", func() {
	It("does not error when resource group not found", func() {
		auth := setConfigVariablesFromFlags()
		rclient, err := client.NewResourceGroupsClient(auth)
		Expect(err).NotTo(HaveOccurred())
		Expect(rclient.Delete(context.TODO(), "random-2342341")).To(Succeed())
	})
	It("does not error when resource group not found", func() {
		auth := setConfigVariablesFromFlags()
		natClient, err := client.NewNatGatewaysClient(auth)
		Expect(err).NotTo(HaveOccurred())
		Expect(natClient.Delete(context.TODO(), "random-2342341", "nat2134")).To(Succeed())
	})
})

func setConfigVariablesFromFlags() internal.ClientAuth {
	flag.Parse()
	if *secretYamlPath != "" {
		return readAuthFromFile(*secretYamlPath)
	} else {
		Skip("No secret yaml path specified to test the Azure client")
		return internal.ClientAuth{}
	}
}

type ProviderSecret struct {
	Data internal.ClientAuth `yaml:"data"`
}

func readAuthFromFile(fileName string) internal.ClientAuth {
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
