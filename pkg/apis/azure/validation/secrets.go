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

package validation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"

	corev1 "k8s.io/api/core/v1"
)

var (
	guidRegex = regexp.MustCompile("^[0-9A-Fa-f]{8}-([0-9A-Fa-f]{4}-){3}[0-9A-Fa-f]{12}$")
)

// ValidateCloudProviderSecret checks whether the given secret contains a valid Azure client credentials.
func ValidateCloudProviderSecret(secret *corev1.Secret) error {
	secretKey := fmt.Sprintf("%s/%s", secret.Namespace, secret.Name)

	for _, key := range []string{azure.SubscriptionIDKey, azure.TenantIDKey, azure.ClientIDKey, azure.ClientSecretKey} {
		val, ok := secret.Data[key]
		if !ok {
			return fmt.Errorf("missing %q field in secret %s", key, secretKey)
		}
		if len(val) == 0 {
			return fmt.Errorf("field %q in secret %s cannot be empty", key, secretKey)
		}
		switch key {
		case azure.SubscriptionIDKey, azure.TenantIDKey, azure.ClientIDKey:
			// subscriptionID, tenantID, and clientID must be valid GUIDs,
			// see https://docs.microsoft.com/en-us/rest/api/securitycenter/locations/get
			if !guidRegex.Match(val) {
				return fmt.Errorf("field %q in secret %s must be a valid GUID", key, secretKey)
			}
		case azure.ClientSecretKey:
			// clientSecret must not contain leading or trailing new lines, as they are known to cause issues
			// Other whitespace characters such as spaces are intentionally not checked for,
			// since there is no documentation indicating that they would not be valid
			if strings.Trim(string(val), "\n\r") != string(val) {
				return fmt.Errorf("field %q in secret %s must not contain leading or traling new lines", key, secretKey)
			}
		}
	}

	return nil
}
