// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"net/http"

	"github.com/Azure/go-autorest/autorest"
)

// IsNotFound ignore Azure Not Found Error
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}

	actual, ok := err.(autorest.DetailedError)
	if !ok {
		return false
	}

	if code, ok := actual.StatusCode.(int); !ok || code != http.StatusNotFound {
		return false
	}

	return true
}
