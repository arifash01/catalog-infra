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
	"bytes"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"knative.dev/pkg/apis"
)
var v2Ptr = flag.Bool("gcbV2", false, "Run on V2") // Define v2 Flag
const (
	tektonRunPattern = `(?m)^(taskrun|pipelinerun)\.tekton\.dev/(\S+)\s+created$`
	serviceAccount = "projects/gcb-catalog-testing/serviceAccounts/gcb-catalog-e2e-testing@gcb-catalog-testing.iam.gserviceaccount.com"
	bundlePath = "us-docker.pkg.dev/gcb-catalog-testing/bundles/"
	project = "gcb-catalog-testing"
	region = "us-central1"
	prefix = "integration-tests-"
	bundlePlaceholder = "BUNDLE_ID"
)

type Clients struct {
	TKN MyTektonClient
	GCB MyCloudBuildClient
	GcbV2 bool
}

func GcbV2(t *testing.T)bool{
	t.Helper()
	flag.Parse()
	return *v2Ptr
}

type MyTektonClient struct {
	Name string
	Kind string
    K8sClientset *kubernetes.Clientset
    TektonClient *versioned.Clientset
}

// getTektonRun extracts a single Tekton TaskRun or PipelineRun from the output
func (mtc *MyTektonClient)getTektonRun(output string)  error {
	re := regexp.MustCompile(tektonRunPattern)
	matches := re.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return fmt.Errorf("no TaskRun or PipelineRun found in the output")
	}
	if len(matches[0]) > 2 {
		mtc.Name = matches[0][2]
		mtc.Kind = matches[0][1]
		return nil
	}
	return fmt.Errorf("no TaskRun or PipelineRun found in the output")
}

// CreateNamespace creates a namespace for testing in the kubernetes cluster
func (mtc *MyTektonClient)CreateNamespace(namespace string) error {
	cmd := exec.Command("kubectl", "create", "namespace", namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create namespace: %v\n%s", err, output)
	}
	return nil
}

// DeleteNamespace deletes the namespace and all resources in it
func (mtc *MyTektonClient)DeleteNamespace(namespace string) error {
	cmd := exec.Command("kubectl", "delete", "namespace", namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete namespace: %v\n%s", err, output)
	}
	return nil
}

// fetchTektonRunLogs fetches the logs for the Tekton TaskRun or PipelineRun
func (mtc *MyTektonClient)fetchTektonRunLogs(t *testing.T, namespace string) {
	t.Helper()
	podName := mtc.Name + "-pod"
	cmd := exec.Command("kubectl", "logs", podName, "-n", namespace, "--all-containers")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get logs for Tekton Run: %v\n%s", err, output)
	}
	t.Logf("Tekton Run logs:\n%s", output)
}


type MyCloudBuildClient struct {
	kind string
	workspaceName string
}

// Apply v2 yaml file
func (cbc *MyCloudBuildClient)runGcloudBuildsApply(filePath string) (string, error) {
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

// Edit existing tr/pr to v2 syntax
func (cbc *MyCloudBuildClient)modifyYamlToV2(t *testing.T,yamlFilePath string, id string) {
	t.Helper()
	yamlData, err := os.ReadFile(yamlFilePath)
	if err != nil {
		t.Fatalf("error reading YAML file: %w", err)
	}

	var data map[interface{}]interface{}
	err = yaml.Unmarshal(yamlData, &data)
	if err != nil {
		t.Fatalf("error unmarshaling YAML: %w", err)
	}

	if spec, ok := data["spec"].(map[interface{}]interface{}); ok {
		// Add service account
		if security, ok := spec["security"].(map[interface{}]interface{}); ok {
			security["serviceAccount"] = serviceAccount
		} else {
			// If 'security' doesn't exist, create it
			spec["security"] = map[interface{}]interface{}{
				"serviceAccount": serviceAccount,
			}
		}
		// Remove everything after workspace name
		if workspaces, ok := spec["workspaces"].([]interface{}); ok {
            if len(workspaces) > 0 {
                if workspace, ok := workspaces[0].(map[interface{}]interface{}); ok {
                    newWorkspace := map[interface{}]interface{}{
                        "name": workspace["name"],
                    }
                    spec["workspaces"] = []interface{}{newWorkspace} 
                } else {
                    t.Fatalf("error: first element in workspaces is not a map")
                }
            }
        } else {
            t.Log("error: 'workspaces' field not found or not a list")
        }
	} else {
		t.Fatalf("error: 'spec' field not found or not a map")
	}

	// To ensure unique tests
	if metadata, ok := data["metadata"].(map[interface{}]interface{}); ok {
		metadata["name"] = prefix+id
	} else {
		t.Fatalf("Error: metadata field not found or not a map")
	}
	// Sets Kind
	if trOrPr, ok :=data["kind"].(string); ok {
	 	cbc.kind = strings.ToLower(trOrPr)
   	} else{
		t.Fatalf("Error: kind not found or not a map")
   	}
	// Save modified yaml file and overwrite existing one
	modifiedYaml, err := yaml.Marshal(data)
	if err != nil {
		t.Fatalf("error marshaling YAML: %w", err)
	}
	err = os.WriteFile(yamlFilePath, modifiedYaml, 0644)
	if err != nil {
		t.Fatalf("error writing YAML file: %w", err)
	}
	t.Log("YAML file modified successfully")
}

// Monitor build status with gcloud
func (cbc *MyCloudBuildClient)monitorBuildStatusWithGcloud(t *testing.T,buildID string) {
    t.Helper()
    for {
		output := cbc.getGcloudBuildStatus(t, buildID)
		type BuildRun struct {
			Conditions []struct {
				Status string `json:"status"`;
				Type string `json:"type"`;
				Reason string `json:"reason"`
			} `json:"conditions"`
		}
		var buildRun BuildRun
		if err := json.Unmarshal(output, &buildRun); err != nil {
			t.Fatalf("Error unmarshaling JSON:", err)
		}

		index:=-1
		for index == -1 {
			if len(buildRun.Conditions) > 0 && buildRun.Conditions[0].Type == "Succeeded" {
				index = 0
			} else if len(buildRun.Conditions) > 1 && buildRun.Conditions[1].Type == "Succeeded" {
				index = 1
			} else {
				time.Sleep(2 * time.Second) // Wait for 2 seconds before checking again
				fmt.Println("Waiting to fetch build status...")
				if err := json.Unmarshal(output, &buildRun); err != nil {
					t.Fatalf("Error unmarshaling JSON:", err)
				}
			}
		}
		
        switch buildRun.Conditions[index].Status {
        case "TRUE":
            t.Log("Build completed successfully!")
			return
        case "FALSE":
            t.Logf("Build encountered an error: %s\n", buildRun.Conditions[index].Reason)
			return
        default:
		t.Logf("Build is running...")

        }
        time.Sleep(10 * time.Second)
    }
}

// Gets current build status with gcloud
func (cbc *MyCloudBuildClient)getGcloudBuildStatus(t *testing.T,buildID string) []byte {
	cmd := exec.Command("gcloud","alpha", "builds", "runs", "describe", prefix+buildID, "--project="+project, "--type="+cbc.kind, "--region="+region,"--format=json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("error getting build status: %w\nOutput: %s", err, string(output))
	}
	return output
}

// DeleteBundle delete OCI bundle that was previously created
func (cbc *MyCloudBuildClient)GcloudDeleteBundle(id string) error {
	path:=bundlePath+id
    cmd := exec.Command(
        "gcloud",
        "artifacts", 
        "docker", 
        "images",
        "delete",
        path,
        "--delete-tags",
        "--quiet", 
    )
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("error deleting bundle: %w\nCommand output: %s", err, string(output))
    }
	fmt.Println("StepAction OCI bundle deleted successfully")
    return nil
}

// ApplyTestYAML applies the Test YAML file to the kubernetes cluster and returns the Tekton TaskRun or PipelineRun
func ApplyTestYAML(t *testing.T, testFilePath, namespace string, client Clients) Clients {
	t.Helper()
	
	// Create a temporary directory in the system's default temp location  (change to empty)
	tempDir, err := ioutil.TempDir("", "temp") 
	if err != nil {
		t.Fatalf("Error creating temp directory:", err)
	}

	defaultPath := copyYaml(t,testFilePath,"default.yaml",tempDir)
	substituteBundleId(t,defaultPath,namespace) //substitutes bundle name in

	//Run on v2 if True
	if (client.GcbV2){
		client.GCB.modifyYamlToV2(t,defaultPath, namespace)
		output, err := client.GCB.runGcloudBuildsApply(defaultPath)
		if err != nil {
			t.Fatalf("Error:", err)
		}
		t.Log(output)
		return client
	}

	cmd := exec.Command("kubectl", "apply", "-f", defaultPath, "-n", namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to apply Test YAML file: %v\n%s", err, output)
	}
	client.TKN.getTektonRun(string(output))
	if err != nil {
		t.Fatalf("failed to get Tekton Run: %v", err)
	}
	return client
}

// Replace ID
func substituteBundleId(t *testing.T, yamlPath string,id string){
	t.Helper()

	yamlData, err := ioutil.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("Error reading file:", err)
		return
	}
 	re := regexp.MustCompile(bundlePlaceholder)
	newYamlData := re.ReplaceAll(yamlData, []byte(id))

	if bytes.Equal(yamlData, newYamlData) {
		t.Fatalf("Could not replace %s, no occurrences were found.",bundlePlaceholder)
	}	
 
	// Write the modified content back to yaml file
	err = ioutil.WriteFile(yamlPath, newYamlData, 0644)
	if err != nil {
		t.Fatalf("Error writing file:", err)
		return
	}
 
	t.Log("Successfully replaced "+bundlePlaceholder)
}

// copies a yaml to a new file and returns the new path
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
func WaitForRunCompletion(t *testing.T, client Clients, watchTimeout time.Duration, expectedCondition, namespace string) {
	t.Helper()
	flag.Parse()

	//Run on v2 if True
	if (client.GcbV2){
		client.GCB.monitorBuildStatusWithGcloud(t,namespace)
		return
	}
	var watcher watch.Interface
	var err error

	// Calculate timeout in seconds
	timeoutSeconds := int64(watchTimeout.Seconds())

	switch strings.ToLower(client.TKN.Kind) {
	case "taskrun":
		watcher, err = client.TKN.TektonClient.TektonV1().TaskRuns(namespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  fmt.Sprintf("metadata.name=%s", client.TKN.Name),
			TimeoutSeconds: &timeoutSeconds,
		})
		if err != nil {
			t.Fatalf("failed to start watch for TaskRun: %v", err)
		}
	case "pipelinerun":
		watcher, err = client.TKN.TektonClient.TektonV1().PipelineRuns(namespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  fmt.Sprintf("metadata.name=%s", client.TKN.Name),
			TimeoutSeconds: &timeoutSeconds,
		})
		if err != nil {
			t.Fatalf("failed to start watch for PipelineRun: %v", err)
		}
	default:
		t.Fatalf("unsupported Tekton Run kind: %s", client.TKN.Kind)
	}
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		switch event.Type {
		case watch.Error:
			client.TKN.fetchTektonRunLogs(t, namespace)
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

	client.TKN.fetchTektonRunLogs(t, namespace)
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

//CreateBundle pushes the StepAction to AR as an OCI bundle
func CreateBundle(stepAction string, id string) error {
	path:= bundlePath+id+":latest"
	cmd := exec.Command("tkn", "bundle", "push", path,"-f",stepAction)
	output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("error pushing bundle: %w\nCommand output: %s", err, string(output))
    }
    fmt.Println("StepAction OCI bundle pushed successfully!")
    return nil

}

