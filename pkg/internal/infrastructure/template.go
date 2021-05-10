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
	_ "embed"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/pkg/errors"
)

var (
	//go:embed templates/main.tpl.tf
	mainFile string
	//go:embed templates/terraform.tfvars
	terraformTFVars []byte
	//go:embed templates/variables.tf
	variablesTF string

	mainTemplate *template.Template
)

func init() {
	var err error
	mainTemplate, err = template.
		New("main.tf").
		Funcs(sprig.TxtFuncMap()).
		Funcs(
			map[string]interface{}{
				"required": required,
			}).
		Parse(mainFile)

	if err != nil {
		panic(err)
	}
}

func required(message string, val interface{}) (interface{}, error) {
	switch val := val.(type) {
	case string:
		if val == "" {
			return nil, errors.Errorf("missing required value %s", message)
		}
	default:
		if val == nil {
			return nil, errors.Errorf("missing required value %s", message)
		}
	}
	return val, nil
}

// RenderTerraformFiles renders the templates using data as the input for the template execution and
// returns the contents of the files used for the Terraformer job.
func RenderTerraformFiles(data interface{}) (*TerraformFiles, error) {
	var mainTF bytes.Buffer

	if err := mainTemplate.Execute(&mainTF, data); err != nil {
		return nil, fmt.Errorf("could not render Terraform template: %+v", err)
	}

	return &TerraformFiles{
		Main:      mainTF.String(),
		Variables: variablesTF,
		TFVars:    terraformTFVars,
	}, nil
}
