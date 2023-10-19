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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/go-autorest/autorest"
	azerrors "github.com/AzureAD/microsoft-authentication-library-for-go/apps/errors"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
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
		Entry("should return true as error if it is an NotFound error", true, true, http.StatusNotFound, true))
	DescribeTable("#IsAzureAPIUnauthorized",
		func(errorType int, statusCode int, expectIsUnauthorizedError bool) {
			err := errors.New("error")
			switch errorType {
			case 0:
				err = &azcore.ResponseError{
					StatusCode: statusCode,
				}
			case 1:
				err = autorest.DetailedError{
					Original:   err,
					StatusCode: statusCode,
				}
			case 2:
				err = azerrors.CallErr{
					Resp: &http.Response{
						StatusCode: statusCode,
					},
					Err: err,
				}

			}
			Expect(IsAzureAPIUnauthorized(err)).To(Equal(expectIsUnauthorizedError))
		},
		Entry("should return true as error if it is an Unauthorized response error", 0, http.StatusUnauthorized, true),
		Entry("should return false as error if it is an NotFound response error", 0, http.StatusNotFound, false),
		Entry("should return true as error if it is an Unauthorized detailed error", 1, http.StatusUnauthorized, true),
		Entry("should return false as error if it is an NotFound detailed error", 1, http.StatusNotFound, false),
		Entry("should return true as error if it is an Unauthorized call error", 2, http.StatusUnauthorized, true),
		Entry("should return false as error if it is an NotFound call error", 2, http.StatusNotFound, false),
		Entry("should return false as error if it is an unknown error", -1, http.StatusUnauthorized, false),
	)
})
