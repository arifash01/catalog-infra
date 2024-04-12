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
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gcb-catalog-testing-bot/catalog-infra/pkg/tekton"
)

// CopyStepActionFiles copies the action and test files from the source directory to the destination directory
func CopyStepActionFiles(srcDir, dstDir string) error {
	srcStepActionFile, err := GetStepActionFile(srcDir)
	if err != nil {
		return err
	}
	dstStepActionFile := filepath.Join(dstDir, filepath.Base(srcStepActionFile))
	if err := copyFile(srcStepActionFile, dstStepActionFile); err != nil {
		return err
	}

	srcTestsDir := filepath.Join(srcDir, "tests")
	if _, err := os.Stat(srcTestsDir); !os.IsNotExist(err) {
		dstTestsDir := filepath.Join(dstDir, "tests")
		if err := os.MkdirAll(dstTestsDir, 0755); err != nil {
			return err
		}
		srcTestFiles, err := filepath.Glob(filepath.Join(srcTestsDir, "*.yaml"))
		if err != nil {
			return err
		}
		for _, srcTestFile := range srcTestFiles {
			dstTestFile := filepath.Join(dstTestsDir, filepath.Base(srcTestFile))
			if err := copyFile(srcTestFile, dstTestFile); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetStepActionFile returns the path to the step action file in the source directory
func GetStepActionFile(srcDir string) (string, error) {
	srcStepActionFiles, err := filepath.Glob(filepath.Join(srcDir, "*.yaml"))
	if err != nil {
		return "", err
	}
	if len(srcStepActionFiles) == 0 {
		return "", fmt.Errorf("no YAML file found in %s", srcDir)
	}
	if len(srcStepActionFiles) > 1 {
		return "", fmt.Errorf("multiple YAML files found in %s", srcDir)
	}
	return srcStepActionFiles[0], nil
}

func copyFile(src, dst string) error {
	srcData, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	if err = os.WriteFile(dst, srcData, 0644); err != nil {
		return err
	}

	return nil
}

// GenerateRandomSuffix generates a random suffix, e.g. "abc12"
func GenerateRandomSuffix() string {
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	suffix := make([]byte, 5)
	for i := range suffix {
		suffix[i] = charset[rand.Intn(len(charset))]
	}
	return string(suffix)
}

// AddSuffixToFiles adds a suffix to the stepaction and test files in the given directory
func AddSuffixToFiles(srcDir, suffix string) error {
	stepActionFile, err := GetStepActionFile(srcDir)
	if err != nil {
		return err
	}
	stepActionName := strings.TrimSuffix(filepath.Base(stepActionFile), ".yaml")
	if err := tekton.UpdateMetadataName(stepActionFile, suffix); err != nil {
		return err
	}

	testFilePaths, err := filepath.Glob(filepath.Join(srcDir, "tests", "*.yaml"))
	if err != nil {
		return err
	}
	for _, testFilePath := range testFilePaths {
		if err := tekton.UpdateTestFile(testFilePath, stepActionName, suffix); err != nil {
			return err
		}
	}

	return nil
}

// SetupKubectlConfig sets up the kubectl configuration for the specified GKE cluster
func SetupKubectlConfig(projectID, clusterName, region string) error {
	// Set the project ID in gcloud config
	cmd := exec.Command("gcloud", "config", "set", "project", projectID)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set project ID in gcloud config: %v, output: %s", err, output)
	}

	// Get the credentials for the GKE cluster
	cmd = exec.Command("gcloud", "container", "clusters", "get-credentials", clusterName, "--region", region)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to get credentials for the GKE cluster: %v, output: %s", err, output)
	}

	return nil
}
