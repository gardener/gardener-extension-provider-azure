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

package infrastructure

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/Azure/go-autorest/autorest"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

type azureNotFoundErrorMatcher struct{}

func (a *azureNotFoundErrorMatcher) Match(actual interface{}) (success bool, err error) {
	if actual == nil {
		return false, nil
	}

	azError, ok := actual.(error)
	if !ok {
		return false, fmt.Errorf("expected type error, got %s", format.Object(actual, 1))
	}

	if IsNotFound(azError) {
		return true, nil
	}

	return false, nil
}

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

func (a *azureNotFoundErrorMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to be not found error")
}

func (a *azureNotFoundErrorMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to not be not found error")
}

// BeNotFoundError matches errors returned by azure-sdk-for-go remote calls when an object could not be found (HTTP Status code = 404).
func BeNotFoundError() types.GomegaMatcher {
	return &azureNotFoundErrorMatcher{}
}

type azureIDMatcher struct {
	expected string
}

func (a azureIDMatcher) Match(actual interface{}) (success bool, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			success = false
			err = fmt.Errorf("panicked while matching ID, got:\n%s", panicErr)
		}
	}()

	if actual == nil {
		return false, nil
	}

	val := reflect.ValueOf(actual)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return false, fmt.Errorf("expected struct or pointer, got %s", format.Object(actual, 1))
	}

	idField := val.FieldByName("ID")
	if !idField.IsValid() {
		return false, fmt.Errorf("ID field not found")
	}

	var id string
	if idField.Kind() == reflect.Ptr {
		id = idField.Elem().String()
	} else {
		id = idField.String()
	}

	return id == a.expected, nil
}

func (a azureIDMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to match ID")
}

func (a azureIDMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to not match ID")
}

// HaveEqualID succeeds if Azure object has the expected ID.
// Azure objects are identified by an ID, which in azure-sdk-for-go is mapped to an `ID *string` field.
// HaveEqualID will succeed if actual is a struct or a pointer to a struct containing a field with this specification and
// points to a string equal to expected.
func HaveEqualID(expected string) types.GomegaMatcher {
	return &azureIDMatcher{expected: expected}
}
