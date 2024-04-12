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
	"fmt"

	"github.com/gcb-catalog-testing-bot/catalog-infra/pkg/tekton"
)

// AssertTektonRunFieldNotEmpty asserts that a field in the Tekton TaskRun or PipelineRun is not empty
func AssertTektonRunFieldNotEmpty(tektonRunName, tektonRunKind, yqQueryExpression, namespace string) error {
	field, err := tekton.ExtractFieldFromTektonRun(tektonRunName, tektonRunKind, yqQueryExpression, namespace)
	if err != nil {
		return err
	}
	if field == "" {
		return fmt.Errorf("field '%s' is empty", yqQueryExpression)
	}
	return nil
}

// AssertTektonRunFieldEquals asserts that a field in the Tekton TaskRun or PipelineRun equals the expected value
func AssertTektonRunFieldEquals(tektonRunName, tektonRunKind, yqQueryExpression, expected, namespace string) error {
	field, err := tekton.ExtractFieldFromTektonRun(tektonRunName, tektonRunKind, yqQueryExpression, namespace)
	if err != nil {
		return err
	}
	if field != expected {
		return fmt.Errorf("field '%s' does not equal '%s'", yqQueryExpression, expected)
	}
	return nil
}
