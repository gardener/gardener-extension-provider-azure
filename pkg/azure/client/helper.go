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

	"github.com/Azure/go-autorest/autorest"
)

// IsAzureAPINotFoundError tries to determine if an error is a resource not found error.
func IsAzureAPINotFoundError(err error) bool {
	switch e := err.(type) {
	case autorest.DetailedError:
		if e.Response != nil && e.Response.StatusCode == http.StatusNotFound {
			return true
		}
	}
	return false
}

// IsAzureAPIUnauthorized tries to determine if the API error is due to unauthorized access
func IsAzureAPIUnauthorized(err error) bool {
	switch e := err.(type) {
	case autorest.DetailedError:
		if e.Response.StatusCode == http.StatusUnauthorized {
			return true
		}
	}
	return false
}
