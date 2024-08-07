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

// Package assert provides utility functions for asserting conditions in tests.
package assert

import (
	"context"
	"strings"
	"testing"

	"github.com/gcb-catalog-testing-bot/catalog-infra/pkg/resourcemanager"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AssertStepResultNotEmpty asserts that a step result in the Tekton TaskRun is not empty
func AssertStepResultNotEmpty(t *testing.T, client resourcemanager.Clients, stepName, resultName, namespace string) {
	t.Helper()
	//Skip if runnign of V2
	if (client.GcbV2){
		t.Log("Can't assert result in V2, skipping...")
		return
	}
	var steps []v1.StepState

	switch strings.ToLower(client.TKN.Kind) {
	case "taskrun":
		taskRun, err := client.TKN.TektonClient.TektonV1().TaskRuns(namespace).Get(context.TODO(), client.TKN.Name, metav1.GetOptions{})
		if err != nil {
			t.Errorf("failed to get TaskRun: %v", err)
		}
		steps = taskRun.Status.Steps
	case "pipelinerun":
		t.Error("PipelineRun not supported for verifying step-level results")
	default:
		t.Errorf("unsupported Tekton Run kind: %s", client.TKN.Kind)
	}

	checkStepResults(t, steps, stepName, resultName)
}

// checkStepResults checks that a step result in the Tekton TaskRun is not empty
func checkStepResults(t *testing.T, steps []v1.StepState, stepName, resultName string) {
	t.Helper()
	for _, step := range steps {
		if step.Name != stepName {
			continue
		}
		for _, result := range step.Results {
			if result.Name != resultName {
				continue
			}
			switch result.Type {
			case v1.ResultsTypeString:
				if result.Value.StringVal != "" {
					return
				}
			case v1.ResultsTypeArray:
				if len(result.Value.ArrayVal) > 0 {
					return
				}
			case v1.ResultsTypeObject:
				if result.Value.ObjectVal != nil && len(result.Value.ObjectVal) > 0 {
					return
				}
			default:
				t.Errorf("unsupported result type for '%s': %v", resultName, result.Type)
			}

			t.Errorf("Step result '%s' in step '%s' is empty", resultName, step.Name)
		}
	}
	t.Errorf("Step result '%s' not found in Step '%s'", resultName, stepName)
}
