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

// Package k8s provides utility functions for working with kubectl commands.
package k8s

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

// TektonRun represents a Tekton TaskRun or PipelineRun
type TektonRun struct {
	Name string
	Kind string
}

// ApplyTektonYAML applies the Tekton YAML file to the kubernetes cluster
func ApplyTektonYAML(taskFilePath string) (string, error) {
	cmd := exec.Command("kubectl", "apply", "-f", taskFilePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("failed to apply Tekton YAML file: %v\n%s", err, output)
	}
	return string(output), nil
}

// WaitForTektonRunCompletion waits for the Tekton TaskRun or PipelineRun to complete with the expected condition
func WaitForTektonRunCompletion(ctx context.Context, tektonRunName, tektonRunKind, expectedCondition string) error {
	resourceType := strings.ToLower(tektonRunKind) + "s"

	timeout := watchTimeoutMinutes * time.Minute
	cmd := exec.CommandContext(ctx, "kubectl", "wait", "--for=condition="+expectedCondition, fmt.Sprintf("--timeout=%s", timeout.String()), fmt.Sprintf("%s/%s", resourceType, tektonRunName))
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("waiting for %s %s to reach condition %s: %v", tektonRunKind, tektonRunName, expectedCondition, err)
	}

	return nil
}

// GetTektonRun extracts the Tekton TaskRun or PipelineRun from the output
func GetTektonRun(output string) (TektonRun, error) {
	re := regexp.MustCompile(tektonRunPattern)
	match := re.FindStringSubmatch(output)
	if len(match) > 2 {
		kind := match[1]
		name := match[2]
		return TektonRun{
			Name: name,
			Kind: kind,
		}, nil
	}
	return TektonRun{}, fmt.Errorf("no Tekton TaskRun or PipelineRun found in the output")
}
