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
	"testing"
	"time"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"knative.dev/pkg/apis"
)

const (
	tektonRunPattern = `(?m)^(taskrun|pipelinerun)\.tekton\.dev/(\S+)\s+created$`
)

// TektonRun represents a Tekton TaskRun or PipelineRun
type TektonRun struct {
	Name string
	Kind string
}

// ApplyStepActionYAML applies the Tekton StepAction YAML file to the kubernetes cluster
func ApplyStepActionYAML(stepActionFilePath, namespace string) error {
	cmd := exec.Command("kubectl", "apply", "-f", stepActionFilePath, "-n", namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to apply Tekton YAML file: %v\n%s", err, output)
	}
	return nil
}

// ApplyTestYAML applies the Test YAML file to the kubernetes cluster and returns the Tekton TaskRun or PipelineRun
func ApplyTestYAML(t *testing.T, testFilePath, namespace string) TektonRun {
	t.Helper()
	cmd := exec.Command("kubectl", "apply", "-f", testFilePath, "-n", namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to apply Test YAML file: %v\n%s", err, output)
	}
	tektonRun, err := getTektonRun(string(output))
	if err != nil {
		t.Fatalf("failed to get Tekton Run: %v", err)
	}
	return tektonRun
}

// WaitForTektonRunCompletion waits for the Tekton TaskRun or PipelineRun to complete with the expected condition within the timeout
func WaitForTektonRunCompletion(t *testing.T, tektonClient *versioned.Clientset, tektonRun TektonRun, watchTimeout time.Duration, expectedCondition, namespace string) {
	t.Helper()
	var watcher watch.Interface
	var err error

	// Calculate timeout in seconds
	timeoutSeconds := int64(watchTimeout.Seconds())

	switch strings.ToLower(tektonRun.Kind) {
	case "taskrun":
		watcher, err = tektonClient.TektonV1().TaskRuns(namespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  fmt.Sprintf("metadata.name=%s", tektonRun.Name),
			TimeoutSeconds: &timeoutSeconds,
		})
		if err != nil {
			t.Fatalf("failed to start watch for TaskRun: %v", err)
		}
	case "pipelinerun":
		watcher, err = tektonClient.TektonV1().PipelineRuns(namespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  fmt.Sprintf("metadata.name=%s", tektonRun.Name),
			TimeoutSeconds: &timeoutSeconds,
		})
		if err != nil {
			t.Fatalf("failed to start watch for PipelineRun: %v", err)
		}
	default:
		t.Fatalf("unsupported Tekton Run kind: %s", tektonRun.Kind)
	}
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		switch event.Type {
		case watch.Error:
			t.Fatalf("watch error: %v", event.Object)
		case watch.Modified, watch.Added:
			switch run := event.Object.(type) {
			case *v1.TaskRun:
				if run.IsDone() && meetExpectedCondition(run.Status.Conditions, expectedCondition) {
					return
				}
			case *v1.PipelineRun:
				if run.IsDone() && meetExpectedCondition(run.Status.Conditions, expectedCondition) {
					return
				}
			}
		}
	}

	t.Fatalf("watch timed out after %v", watchTimeout)
}

// meetExpectedCondition checks if the Tekton TaskRun or PipelineRun meets the expected condition
func meetExpectedCondition(conditions []apis.Condition, expectedCondition string) bool {
	for _, cond := range conditions {
		if string(cond.Type) == expectedCondition && cond.Status == "True" {
			return true
		}
	}
	return false
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

// CreateNamespace creates a namespace for testing in the kubernetes cluster
func CreateNamespace(namespace string) error {
	cmd := exec.Command("kubectl", "create", "namespace", namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create namespace: %v\n%s", err, output)
	}
	return nil
}

// DeleteNamespace deletes the namespace and all resources in it
func DeleteNamespace(namespace string) error {
	cmd := exec.Command("kubectl", "delete", "namespace", namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete namespace: %v\n%s", err, output)
	}
	return nil
}
