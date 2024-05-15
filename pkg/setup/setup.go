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

// Package setup provides utility functions for setting up and tearing down tests.
package setup

import (
	"os"
	"testing"

	"github.com/gcb-catalog-testing-bot/catalog-infra/pkg/resourcemanager"
	"github.com/google/uuid"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// SetupTest creates a temporary namespace for testing and returns the namespace name and a cleanup function.
func SetupTest(t *testing.T, client *kubernetes.Clientset, tektonYAMLPath string) (string, func()) {
	t.Helper()
	t.Log("setting up tests ...")

	// Create a temporary namespace for testing
	namespace := uuid.New().String()
	if err := resourcemanager.CreateNamespace(namespace); err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}
	t.Logf("using namespace: %s", namespace)

	// Cleanup function
	cleanup := func() {
		t.Helper()
		t.Log("tearing down tests...")
		if err := resourcemanager.DeleteNamespace(namespace); err != nil {
			t.Fatalf("failed to delete namespace: %v", err)
		}
	}

	// Apply StepAction YAML
	if err := resourcemanager.ApplyStepActionYAML(tektonYAMLPath, namespace); err != nil {
		t.Fatalf("failed to apply Tekton YAML: %v", err)
	}

	return namespace, cleanup
}

// InitK8sClients initializes a k8s client and a Tekton client.
func InitK8sClients(t *testing.T) (*kubernetes.Clientset, *versioned.Clientset) {
	t.Helper()
	var kubeConfig = os.Getenv("KUBECONFIG")

	t.Logf("using kubeconfig: %s", kubeConfig)

	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		t.Fatalf("failed to create k8s config: %v", err)
	}

	k8sClientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("failed to create k8s client: %v", err)
	}

	tektonClient, err := versioned.NewForConfig(config)
	if err != nil {
		t.Fatalf("failed to create Tekton client: %v", err)
	}

	return k8sClientset, tektonClient
}
