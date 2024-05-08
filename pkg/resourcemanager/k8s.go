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
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const (
	watchTimeoutMinutes = 10
	tektonRunPattern    = `(?m)^(taskrun|pipelinerun)\.tekton\.dev/(\S+)\s+created$`
)

// ApplyStepActionYAML applies the Tekton StepAction YAML file to the kubernetes cluster
func ApplyStepActionYAML(stepActionFilePath, namespace string) error {
	cmd := exec.Command("kubectl", "apply", "-f", stepActionFilePath, "-n", namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to apply Tekton YAML file: %v\n%s", err, output)
	}
	return nil
}

// ApplyTestYAML applies the Test YAML file to the kubernetes cluster
func ApplyTestYAML(testFilePath, namespace string) (TektonRun, error) {
	cmd := exec.Command("kubectl", "apply", "-f", testFilePath, "-n", namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return TektonRun{}, fmt.Errorf("failed to apply Test YAML file: %v\n%s", err, output)
	}
	return getTektonRun(string(output))
}

// WaitForTektonRunCompletion waits for the Tekton TaskRun or PipelineRun to complete with the expected condition
func WaitForTektonRunCompletion(ctx context.Context, tektonRunName, tektonRunKind, expectedCondition, namespace string) error {
	resourceType := strings.ToLower(tektonRunKind) + "s"

	timeout := watchTimeoutMinutes * time.Minute
	cmd := exec.CommandContext(ctx, "kubectl", "wait", "--for=condition="+expectedCondition, fmt.Sprintf("--timeout=%s", timeout.String()), fmt.Sprintf("%s/%s", resourceType, tektonRunName), "-n", namespace)
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("waiting for %s %s to reach condition %s: %v", tektonRunKind, tektonRunName, expectedCondition, err)
	}

	return nil
}

// getTektonRun extracts a single Tekton TaskRun or PipelineRun from the output
func getTektonRun(output string) (TektonRun, error) {
	re := regexp.MustCompile(tektonRunPattern)
	matches := re.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return TektonRun{}, fmt.Errorf("no TaskRun or PipelineRun found in the output")
	}
	if len(matches[0]) > 2 {
		return TektonRun{
			Name: matches[0][2],
			Kind: matches[0][1],
		}, nil

	}
	return TektonRun{}, fmt.Errorf("no TaskRun or PipelineRun found in the output")
}

// GetTektonRunYAML gets the YAML for the Tekton TaskRun or PipelineRun
func GetTektonRunYAML(tektonRunName, tektonRunKind, namespace string) (string, error) {
	resourceType := strings.ToLower(tektonRunKind) + "s"
	cmd := exec.Command("kubectl", "get", fmt.Sprintf("%s/%s", resourceType, tektonRunName), "-n", namespace, "-o", "yaml")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("failed to get Tekton Run YAML: %v\n%s", err, output)
	}
	return string(output), nil
}

// DeleteTektonYAML deletes the Tekton YAML file from the kubernetes cluster
func DeleteTektonYAML(taskFilePath, namespace string) (string, error) {
	cmd := exec.Command("kubectl", "delete", "-f", taskFilePath, "-n", namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("failed to delete Tekton YAML file: %v\n%s", err, output)
	}
	return string(output), nil
}

// CreateTestNamespace creates a namespace for testing in the kubernetes cluster
func CreateTestNamespace(namespace string) (string, error) {
	cmd := exec.Command("kubectl", "create", "namespace", namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("failed to create namespace: %v\n%s", err, output)
	}
	return string(output), nil
}

// DeleteNamespaceAndResources deletes the namespace and all resources in it
func DeleteNamespaceAndResources(namespace string) (string, error) {
	cmd := exec.Command("kubectl", "delete", "namespace", namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("failed to delete namespace: %v\n%s", err, output)
	}
	return string(output), nil
}
