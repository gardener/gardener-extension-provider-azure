// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client_test

import (
	"errors"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	azerrors "github.com/AzureAD/microsoft-authentication-library-for-go/apps/errors"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

var _ = Describe("Helper", func() {
	DescribeTable("#IsAzureAPINotFoundError",
		func(err error, expectIsNotFoundError bool) {
			Expect(IsAzureAPINotFoundError(err)).To(Equal(expectIsNotFoundError))
		},
		Entry("should return false as error is not a detailed azure error", errors.New("error"), false),
		Entry("should return true as error is a NotFound error",
			&azcore.ResponseError{StatusCode: http.StatusNotFound}, true),
		Entry("should return true as error is a NotFound error",
			azerrors.CallErr{Resp: &http.Response{StatusCode: http.StatusNotFound}}, true))
	DescribeTable("#IsAzureAPIUnauthorized",
		func(errorType int, statusCode int, expectIsUnauthorizedError bool) {
			err := errors.New("error")
			switch errorType {
			case 0:
				err = &azcore.ResponseError{
					StatusCode: statusCode,
				}
			case 1:
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
		Entry("should return true as error if it is an Unauthorized call error", 1, http.StatusUnauthorized, true),
		Entry("should return false as error if it is an NotFound call error", 1, http.StatusNotFound, false),
		Entry("should return false as error if it is an unknown error", -1, http.StatusUnauthorized, false),
	)
})
