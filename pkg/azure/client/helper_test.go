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
	"errors"
	"net/http"

	. "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"

	"github.com/Azure/go-autorest/autorest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Helper", func() {
	DescribeTable("#IsAzureAPINotFoundError",
		func(isDetailedError, hasResponse bool, statusCode int, expectIsNotFoundError bool) {
			var err = errors.New("error")
			if !isDetailedError {
				Expect(IsAzureAPINotFoundError(err)).To(Equal(expectIsNotFoundError))
				return
			}
			var detailedError = autorest.DetailedError{
				Original:   err,
				StatusCode: statusCode,
			}
			if hasResponse {
				detailedError.Response = &http.Response{
					StatusCode: statusCode,
				}
			}
			Expect(IsAzureAPINotFoundError(detailedError)).To(Equal(expectIsNotFoundError))
		},
		Entry("should return false as error is not a detailed azure error", false, false, 999, false),
		Entry("should return false as error is not a NotFound", true, false, http.StatusInternalServerError, false),
		Entry("should return true as error is a NotFound", true, true, http.StatusNotFound, true),
	)
})
