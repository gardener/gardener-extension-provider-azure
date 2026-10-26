// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/gardener/gardener-extension-provider-azure/hack/tools/apitypecheck/pkg/apitypecheck"
)

func main() {
	singlechecker.Main(apitypecheck.Analyzer)
}
