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

// Package setup provides utility functions for setting up and modifying stepaction and test files.
package setup

import (
	"fmt"
	"os"

	"github.com/gcb-catalog-testing-bot/catalog-infra/pkg/resourcemanager"
	"github.com/google/uuid"
)

// SetupTestEnvironment sets up a test environment and return the cleanup function to be called after the tests
func SetupTestEnvironment(tektonYAMLPath string) (string, func(), error) {
	fmt.Println("Setting up tests...")

	// Create a temporary namespace for testing
	namespace := uuid.New().String()
	output, err := resourcemanager.CreateTestNamespace(namespace)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create namespace: %v\n%s", err, output)
	}
	fmt.Printf("Using namespace: %s\n", namespace)

	// Cleanup function
	cleanup := func() {
		fmt.Println("Tearing down tests...")
		resourcemanager.DeleteNamespaceAndResources(namespace)
	}

	// Apply StepAction YAML
	if err = resourcemanager.ApplyStepActionYAML(tektonYAMLPath, namespace); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to apply Tekton YAML: %v\n%s", err, output)
	}

	return namespace, cleanup, nil
}

// ExitWithCleanup calls the cleanup function and exits the program with the given exit code.
func ExitWithCleanup(cleanup func(), exitCode int) {
	cleanup()
	os.Exit(exitCode)
}
