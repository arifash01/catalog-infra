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

// Package tekton provides utility functions for working with Tekton YAML files.
package tekton

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/gcb-catalog-testing-bot/catalog-infra/pkg/k8s"
)

// AddSuffixToFiles adds a suffix to the metadata.name field in the YAML files
func UpdateMetadataName(filePath, suffix string) error {
	cmd := exec.Command(
		"yq",
		"eval",
		fmt.Sprintf(`(.metadata.name) += "-%s"`, suffix),
		"-i",
		filePath,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add a suffix to the metadata.name field: %w", err)
	}

	return nil
}

// UpdateTestFile adds a suffix to the ref.name field for stepaction and the taskRef.name field for task
func UpdateTestFile(filePath, stepActionName, suffix string) error {
	if err := updateStepActionRefName(filePath, stepActionName, suffix); err != nil {
		return err
	}
	if err := updateTaskRefName(filePath, suffix); err != nil {
		return err
	}
	if err := UpdateMetadataName(filePath, suffix); err != nil {
		return err
	}
	return nil
}

func updateStepActionRefName(filePath, stepActionName, suffix string) error {
	cmd := exec.Command(
		"yq",
		"eval",
		fmt.Sprintf(`(.. | select(has("ref")) | select(.ref.name == "%s") | .ref.name) += "-%s"`, stepActionName, suffix),
		"-i",
		filePath,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update the ref.name field for the stepaction: %w", err)
	}

	return nil
}

func updateTaskRefName(filePath, suffix string) error {
	taskName, err := getMetadataName(filePath)
	if err != nil {
		return err
	}

	cmd := exec.Command(
		"yq",
		"eval",
		fmt.Sprintf(`(.. | select(has("taskRef")) | select(.taskRef.name == "%s") | .taskRef.name) += "-%s"`, taskName, suffix),
		"-i",
		filePath,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update the name field in the taskRef: %w", err)
	}

	return nil
}

func getMetadataName(filePath string) (string, error) {
	cmd := exec.Command(
		"yq",
		"eval",
		`select(.kind == "Task" or .kind == "Pipeline") | .metadata.name`,
		filePath,
	)

	taskName, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get metadata.name field for the task: %w", err)
	}

	return strings.TrimSpace(string(taskName)), nil
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
func ExtractFieldFromTektonRun(tektonRunName, tektonRunKind, yqQueryExpression string) (string, error) {
	tektonYaml, err := k8s.GetTektonRunYAML(tektonRunName, tektonRunKind)
	if err != nil {
		return "", err
	}
	return ExtractFieldFromYAML(tektonYaml, yqQueryExpression)
}
