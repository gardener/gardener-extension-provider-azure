// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package infrastructure

import (
	"bytes"
	"text/template"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Template", func() {

	DescribeTable("required", func(values interface{}, out string, hasError bool) {
		testTpl := `Hello, {{ required "name" .name}}`
		parsedTpl, err := template.New("test").Funcs(
			map[string]interface{}{
				"required": required,
			}).
			Parse(testTpl)
		Expect(err).NotTo(HaveOccurred())

		var buffer bytes.Buffer
		err = parsedTpl.Execute(&buffer, values)

		if !hasError {
			Expect(err).NotTo(HaveOccurred())
			Expect(buffer.String()).To(Equal(out))
		} else {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("missing required value"))
		}
	},
		Entry("should print 'Hello, World'", map[string]interface{}{"name": "World"}, "Hello, World", false),
		Entry("should return an error if param is missing", map[string]interface{}{}, "", true),
		Entry("should return an error if values are nil", nil, "", true),
		Entry("should return an error if values are present but missing the correct param", map[string]interface{}{"foo": "bar"}, "", true),
		Entry("should return an error if param is present but its empty", map[string]interface{}{"name": ""}, "", true),
	)
})
