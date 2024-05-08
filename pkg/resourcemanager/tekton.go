// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package resourcemanager provides utility functions for managing resources in the testing cluster.
package resourcemanager

import (
	"fmt"
	"os/exec"
	"strings"
)

// TektonRun represents a Tekton TaskRun or PipelineRun
type TektonRun struct {
	Name string
	Kind string
}

// ExtractFieldFromYAML extracts the field from the Tekton YAML using the yq query expression
func ExtractFieldFromYAML(tektonYaml, yqQueryExpression string) (string, error) {
	cmd := exec.Command(
		"yq",
		"eval",
		yqQueryExpression,
		"-",
	)
	cmd.Stdin = strings.NewReader(tektonYaml)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to extract field from Tekton YAML: %v\n%s", err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

// ExtractFieldFromTektonRun extracts the field from the Tekton TaskRun or PipelineRun using the yq query expression
func ExtractFieldFromTektonRun(tektonRunName, tektonRunKind, yqQueryExpression, namespace string) (string, error) {
	tektonYaml, err := GetTektonRunYAML(tektonRunName, tektonRunKind, namespace)
	if err != nil {
		return "", err
	}
	return ExtractFieldFromYAML(tektonYaml, yqQueryExpression)
}
