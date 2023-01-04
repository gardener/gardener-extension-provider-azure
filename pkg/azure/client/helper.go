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

package client

import (
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/go-autorest/autorest"
)

func isAzureAPStatusError(err error, status int) bool {
	switch e := err.(type) {
	case autorest.DetailedError: // error from old azure client
		if e.Response != nil && e.Response.StatusCode == status {
			return true
		}
	case *azcore.ResponseError: // error from new azure SDK client
		if e.StatusCode == http.StatusNotFound {
			return true
		}
	}
	return false
}

// IsAzureAPINotFoundError tries to determine if an error is a resource not found error.
func IsAzureAPINotFoundError(err error) bool {
	return isAzureAPStatusError(err, http.StatusNotFound)
}

// IsAzureAPIUnauthorized tries to determine if the API error is due to unauthorized access
func IsAzureAPIUnauthorized(err error) bool {
	return isAzureAPStatusError(err, http.StatusUnauthorized)
}
