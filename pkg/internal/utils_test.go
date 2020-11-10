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

package internal_test

import (
	"errors"
	"net/http"

	. "github.com/gardener/gardener-extension-provider-azure/pkg/internal"

	"github.com/Azure/go-autorest/autorest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Utils", func() {
	Describe("#AzureAPIErrorNotFound", func() {
		var err error
		BeforeEach(func() {
			err = errors.New("error")
		})

		It("should return false as error is no detailed azure error", func() {
			Expect(AzureAPIErrorNotFound(err)).To(BeFalse())
		})

		It("should return false as error is not a NotFound", func() {
			detailedErr := autorest.DetailedError{
				Original:   err,
				StatusCode: http.StatusInternalServerError,
			}
			Expect(AzureAPIErrorNotFound(detailedErr)).To(BeFalse())
		})

		It("should return true as error is a NotFound", func() {
			detailedErr := autorest.DetailedError{
				Original: err,
				Response: &http.Response{
					StatusCode: http.StatusNotFound,
				},
				StatusCode: http.StatusNotFound,
			}
			Expect(AzureAPIErrorNotFound(detailedErr)).To(BeTrue())
		})
	})
})
