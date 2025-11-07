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
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

var _ = Describe("Helper", func() {
	DescribeTable("#IsAzureAPINotFoundError",
		func(err error, expectIsNotFoundError bool) {
			Expect(IsAzureAPINotFoundError(err)).To(Equal(expectIsNotFoundError))
		},
		Entry("should return false as error is not a detailed azure error", errors.New("error"), false),
		Entry("should return true as error is a NotFound error",
			&azcore.ResponseError{StatusCode: http.StatusNotFound}, true,
		),
		Entry("should return true as error is a NotFound error",
			azerrors.CallErr{Resp: &http.Response{StatusCode: http.StatusNotFound}}, true,
		),
	)

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

	DescribeTable("#CloudConfiguration",
		func(cc *azure.CloudConfiguration, region *string, expectedCloud string, expectedError error) {
			cloud, err := CloudConfiguration(cc, region)
			if expectedError != nil {
				Expect(err).To(MatchError(expectedError))
				Expect(cloud).To(BeNil())
			} else {
				Expect(err).To(BeNil())
				Expect(cloud).ToNot(BeNil())
				Expect(cloud.Name).To(Equal(expectedCloud))
			}
		},
		Entry("should resolve cloud configuration from  cloudConfiguration=AzurePublic", &azure.CloudConfiguration{Name: "AzurePublic"}, nil, "AzurePublic", nil),
		Entry("should resolve cloud configuration from  cloudConfiguration=AzureChina", &azure.CloudConfiguration{Name: "AzureChina"}, nil, "AzureChina", nil),
		Entry("should resolve cloud configuration from  cloudConfiguration=AzureGovernment", &azure.CloudConfiguration{Name: "AzureGovernment"}, nil, "AzureGovernment", nil),
		Entry("should resolve cloud configuration from  region=AzurePublic", nil, ptr.To("eastus"), "AzurePublic", nil),
		Entry("should resolve cloud configuration from  region=AzurePublic", nil, ptr.To("westeurope"), "AzurePublic", nil),
		Entry("should resolve cloud configuration from  region=AzurePublic", nil, ptr.To("uksouth"), "AzurePublic", nil),
		Entry("should resolve cloud configuration from  region=AzureChina", nil, ptr.To("chinanorth3"), "AzureChina", nil),
		Entry("should resolve cloud configuration from  region=AzureChina", nil, ptr.To("chinaeast"), "AzureChina", nil),
		Entry("should resolve cloud configuration from  region=AzureGovernment", nil, ptr.To("USGovTexas"), "AzureGovernment", nil),
		Entry("should resolve cloud configuration from  region=AzureGovernment", nil, ptr.To("USDoDCentral"), "AzureGovernment", nil),
		Entry("should resolve cloud configuration from  region=AzureGovernment", nil, ptr.To("USSecEast"), "AzureGovernment", nil),
		Entry("should fail to resolve cloud configuration", nil, nil, "", errors.New("either CloudConfiguration or region must not be nil to determine Azure Cloud configuration")),
	)
})
