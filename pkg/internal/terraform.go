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

package internal

import (
	"time"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/imagevector"
	"github.com/gardener/gardener-extensions/pkg/terraformer"

	"github.com/gardener/gardener/pkg/logger"
	"k8s.io/client-go/rest"
)

const (
	// TerraformVarClientID is the name of the terraform client id environment variable.
	TerraformVarClientID = "TF_VAR_CLIENT_ID"
	//TerraformVarClientSecret is the name of the client secret environment variable.
	TerraformVarClientSecret = "TF_VAR_CLIENT_SECRET"
)

// TerraformVariablesEnvironmentFromClientAuth computes the Terraformer variables environment from the
// given ServiceAccount.
func TerraformVariablesEnvironmentFromClientAuth(auth *ClientAuth) (map[string]string, error) {
	return map[string]string{
		TerraformVarClientID:     auth.ClientID,
		TerraformVarClientSecret: auth.ClientSecret,
	}, nil
}

// NewTerraformer initializes a new Terraformer that has the azure auth credentials.
func NewTerraformer(
	restConfig *rest.Config,
	clientAuth *ClientAuth,
	purpose,
	namespace,
	name string,
) (terraformer.Terraformer, error) {
	tf, err := terraformer.NewForConfig(logger.NewLogger("info"), restConfig, purpose, namespace, name, imagevector.TerraformerImage())
	if err != nil {
		return nil, err
	}

	variables, err := TerraformVariablesEnvironmentFromClientAuth(clientAuth)
	if err != nil {
		return nil, err
	}

	return tf.
		SetVariablesEnvironment(variables).
		SetActiveDeadlineSeconds(630).
		SetDeadlineCleaning(5 * time.Minute).
		SetDeadlinePod(15 * time.Minute), nil
}
