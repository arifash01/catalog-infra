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
	"encoding/json"
	"os"
	"io/ioutil"
	"path/filepath"
	"gopkg.in/yaml.v2"
	"context"
	"fmt"
	"os/exec"
	"io"
	"regexp"
	"strings"
	"path"
	"testing"
	"time"
	"flag"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"knative.dev/pkg/apis"
)
var v2Ptr = flag.Bool("gcbV2", false, "Run on V2") // Define v2 Flag
const (
	tektonRunPattern = `(?m)^(taskrun|pipelinerun)\.tekton\.dev/(\S+)\s+created$`
)

// TektonRun represents a Tekton TaskRun or PipelineRun
type TektonRun struct {
	Name string
	Kind string
}

func gcbV2(t *testing.T)bool{
	t.Helper()
	flag.Parse()
	return *v2Ptr
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
	flag.Parse()

	populateUniqueID(t,testFilePath,namespace)

	//Run on v2 if True
	if (*v2Ptr){
		// Create a temporary directory in the system's default temp location  (change to empty)
		tempDir, err := ioutil.TempDir("/usr/local/google/home/arifash/catalog-infra/pkg/setup", "temp") 
		if err != nil {
			t.Fatalf("Error creating temp directory:", err)
		}
		// Clean up (delete the temporary directory when you're done)
		defer os.RemoveAll(tempDir)

		defaultPath := copyYaml(t,testFilePath,"default.yaml",tempDir)
		_ = copyYaml(t,"./kustomization.yaml","kustomization.yaml",tempDir)
		modifyMetadataName(defaultPath)

		tempDirName := filepath.Base(filepath.Dir(defaultPath))

		tempFile,err := runKustomize(tempDirName)
		if err != nil {
			t.Fatalf("failed to apply Kustomization: ",err)
		}

		populateUniqueID(t,tempDir,namespace) //makes metadata names unique

		region := "us-central1"
		project := "gcb-catalog-testing"
		output, err := runGcloudBuildsApply(tempFile, region, project)
		if err != nil {
			t.Fatalf("Error:", err)
		}
		t.Log(output)
		return TektonRun{}
	}

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

func runGcloudBuildsApply(filePath, region, project string) (string, error) {
    cmd := exec.Command("gcloud", "alpha", "builds", "runs", "apply",
        "--file="+filePath,
        "--region="+region,
        "--project="+project,
    )

    output, err := cmd.CombinedOutput()
    if err != nil {
        return "", fmt.Errorf("error running gcloud command: %w\nOutput: %s", err, string(output))
    }

    return string(output), nil
}
func runKustomize(fileName string) (string, error) {
    cmd := exec.Command("kubectl", "kustomize", fileName)

    // Capture stdout and stderr
    output, err := cmd.CombinedOutput()
    if err != nil {
        // Handle the error, including potential stderr output
        return "",fmt.Errorf("kustomize failed: %w: %s", err, string(output))
    }
	newFileName := fileName+"/gcbV2.yaml"
    // Write the output to gcbV2.yaml
    err = os.WriteFile(newFileName, output, 0644)
    if err != nil {
        return "",fmt.Errorf("failed to write gcbV2.yaml: %w", err)
    }

    return newFileName, nil
}
// Replace UNIQUEID with ID
func populateUniqueID(t *testing.T, yamlPath string,id string){
	t.Helper()
	// yamlPath:=tempDir+"/gcbV2.yaml"

	yamlData, err := ioutil.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("Error reading file:", err)
		return
	}
 
	// Create a regular expression to match "UNIQUE_ID"
	re := regexp.MustCompile("UNIQUE_ID")
 
	// Replace all occurrences of "UNIQUE_ID" with the value from the environment variable
	newYamlData := re.ReplaceAll(yamlData, []byte(id))
 
	// Write the modified content back to yaml file
	err = ioutil.WriteFile(yamlPath, newYamlData, 0644)
	if err != nil {
		t.Fatalf("Error writing file:", err)
		return
	}
 
	t.Log("Successfully replaced UNIQUE_ID")
}

func copyYaml(t *testing.T,existingPath string, newFileName string,tempDir string)(destinationPath string){
	t.Helper()
	//Open the source YAML file
    sourceFile, err := os.Open(existingPath)
    if err != nil {
        t.Fatalf("Error opening YAML file:", err)
        return
    }
    defer sourceFile.Close()

	// Create a new file in the temporary directory with the same name
   	destinationFile, err := os.Create(path.Join(tempDir, newFileName))
   	if err != nil {
		t.Fatalf("Error creating destination file:", err)
	   return
   	}
   	defer destinationFile.Close()

	// Copy the contents of the source file to the destination file
    _, err = io.Copy(destinationFile, sourceFile)
    if err != nil {
        t.Fatalf("Error copying file:", err)
        return
    }
    err = destinationFile.Sync()
    if err != nil {
        t.Fatalf("Error syncing file:", err)
        return
    }

	return destinationFile.Name()
}

// WaitForTektonRunCompletion waits for the Tekton TaskRun or PipelineRun to complete with the expected condition within the timeout
func WaitForTektonRunCompletion(t *testing.T, tektonClient *versioned.Clientset, tektonRun TektonRun, watchTimeout time.Duration, expectedCondition, namespace string) {
	t.Helper()
	flag.Parse()

	//Run on v2 if True
	if (*v2Ptr){
		bundlePath:= "us-docker.pkg.dev/gcb-catalog-testing/bundles/"+namespace
		monitorBuildStatusWithGcloud(t,"gcb-catalog-testing","us-central1", "test-"+namespace)
		DeleteBundle(bundlePath)
		return
	}
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
			fetchTektonRunLogs(t, tektonRun, namespace)
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

	fetchTektonRunLogs(t, tektonRun, namespace)
	t.Fatalf("watch timed out after %v", watchTimeout)
}

func monitorBuildStatusWithGcloud(t *testing.T,projectID,region, buildID string) {
    t.Helper()
    cmd := exec.Command("gcloud","alpha", "builds", "runs", "describe", buildID, "--project="+projectID, "--region="+region,"--format=json")

    // Poll the build status periodically
    for {
        output, err := cmd.CombinedOutput()
        if err != nil {
            t.Fatalf("error getting build status: %w\nOutput: %s", err, string(output))
        }

         // Parse the JSON output to get the build status
		 var build struct {
            State string `json:"state"` 
        }
        if err := json.Unmarshal(output, &build); err != nil {
            t.Fatalf("error parsing build status JSON: %w", err)
        }

        // Check the build status (using v2 State strings)
        switch build.State {
        case "SUCCEEDED":
            t.Log("Build completed successfully!")
        case "FAILED", "INTERNAL_ERROR", "TIMEOUT", "CANCELLED":
            t.Logf("Build encountered an error: %s\n", build.State)
            t.Fatalf("build error: %s", build.State)
        default: // QUEUED, WORKING, etc.
		t.Log("Build is still running...")
        }

        time.Sleep(5 * time.Second) // Poll every 5 seconds (adjust as needed)
    }
}

// fetchTektonRunLogs fetches the logs for the Tekton TaskRun or PipelineRun
func fetchTektonRunLogs(t *testing.T, tektonRun TektonRun, namespace string) {
	t.Helper()
	podName := tektonRun.Name + "-pod"
	cmd := exec.Command("kubectl", "logs", podName, "-n", namespace, "--all-containers")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get logs for Tekton Run: %v\n%s", err, output)
	}
	t.Logf("Tekton Run logs:\n%s", output)
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

//CreateBundle pushes the StepAction to AR as an OCI bundle
func CreateBundle(stepAction string, id string) error {
	bundlePath:= "us-docker.pkg.dev/gcb-catalog-testing/bundles/"+id+":latest"
	cmd := exec.Command("tkn", "bundle", "push", bundlePath,"-f",stepAction)
	fmt.Println(string(cmd))
	output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("error pushing bundle: %w\nCommand output TESTTT: %s", err, string(output))
    }
    fmt.Println("StepAction OCI bundle pushed successfully!")
    return nil

}

// DeleteBundle delete OCI bundle that was previously created
func DeleteBundle(bundlePath string) error {
    cmd := exec.Command(
        "gcloud",
        "artifacts", 
        "docker", 
        "images",
        "delete",
        bundlePath,
        "--delete-tags",
        "--quiet", 
    )
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("error deleting bundle: %w\nCommand output: %s", err, string(output))
    }
    return nil
}

// modifies metadata names to allow kustomize locate files
func modifyMetadataName(yamlPath string){
	data := make(map[interface{}]interface{})
	yamlFile, err := ioutil.ReadFile(yamlPath)
	if err != nil {
		fmt.Println("Error reading YAML file:", err)
		return
	}
	err = yaml.Unmarshal(yamlFile, &data)
	if err != nil {
		fmt.Println("Error unmarshaling YAML:", err)
		return
	}

	if metadata, ok := data["metadata"].(map[interface{}]interface{}); ok {
		metadata["name"] = "default"
	} else {
		fmt.Println("Error: metadata field not found or not a map")
		return
	}

	modifiedYaml, err := yaml.Marshal(data)
	if err != nil {
		fmt.Println("Error marshaling YAML:", err)
		return
	}
	err = ioutil.WriteFile(yamlPath, modifiedYaml, 0644)
	if err != nil {
		fmt.Println("Error writing YAML file:", err)
		return
	}

   fmt.Println("Modified YAML file:", yamlPath)
}
