// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

// IsNotFound ignore Azure Not Found Error
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}

	actual, ok := err.(*azcore.ResponseError)
	if !ok {
		return false
	}

	if actual.StatusCode != http.StatusNotFound {
		return false
	}

	return true
}
