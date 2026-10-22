// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package apitypecheck_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	. "github.com/gardener/gardener-extension-provider-azure/hack/tools/apitypecheck/pkg/apitypecheck"
)

func TestAPITypeCheck(t *testing.T) {
	t.Run("complete validation — no diagnostics expected", func(t *testing.T) {
		analysistest.Run(t, analysistest.TestData(), Analyzer, "apis/azure/validation/complete")
	})

	t.Run("incomplete validation — missing field.Child reference reported", func(t *testing.T) {
		analysistest.Run(t, analysistest.TestData(), Analyzer, "apis/azure/validation/incomplete")
	})
}
