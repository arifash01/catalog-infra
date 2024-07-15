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
	"path/filepath"
	"testing"

	"github.com/gcb-catalog-testing-bot/catalog-infra/pkg/resourcemanager"
	"github.com/google/uuid"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// SetupTest creates a temporary namespace for testing and returns the namespace name.
func SetupTest(t *testing.T, client resourcemanager.Clients, tektonYAMLPath string) string {
	t.Helper()
	t.Log("setting up tests ...")

	// Create a temporary namespace and id for testing (will be a tekton ns or v2 Id)
	namespace := uuid.New().String()
	id := namespace
	
	if err := resourcemanager.CreateBundle(tektonYAMLPath,id); err != nil {
		t.Fatalf("failed to create bundle: %v", err)
	}
	// Cleanup function
	t.Cleanup(func() {
		if err := client.GCB.GcloudDeleteBundle(namespace); err != nil {
			t.Fatalf("failed to delete bundle: %v", err)
		}
	})

	//Skip if running on gcbV2
	if (client.GcbV2){
		return id
	}

	if err := client.TKN.CreateNamespace(namespace); err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}
	t.Logf("using namespace: %s", namespace)

	// Cleanup function
	t.Cleanup(func() {
		t.Log("tearing down tests...")
		if err := client.TKN.DeleteNamespace(namespace); err != nil {
			t.Fatalf("failed to delete namespace: %v", err)
		}
	})

	return namespace
}

// InitClients initializes a clients.
func InitClients(t *testing.T) (resourcemanager.Clients) {
	t.Helper()

	// If running on v2
	if resourcemanager.GcbV2(t){
		return resourcemanager.Clients{
			GCB: resourcemanager.MyCloudBuildClient{},
			GcbV2: resourcemanager.GcbV2(t),
		}
	}
		
	kubeConfig := os.Getenv("KUBECONFIG")

	if kubeConfig == "" {
		kubeConfig = filepath.Join(homedir.HomeDir(), ".kube", "config")
	}

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
	return resourcemanager.Clients{
		TKN: resourcemanager.MyTektonClient{
			K8sClientset: k8sClientset,
			TektonClient: tektonClient,
		},
		GcbV2: resourcemanager.GcbV2(t),
	}
}