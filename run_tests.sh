#!/bin/bash
set -ux

project=gcb-catalog-testing
zone=us-central1-a
spoofed_sa=gcb-catalog-e2e-testing@gcb-catalog-testing.iam.gserviceaccount.com
vm_name=minikube-test-vm-prow

gcloud config set project ${project}
gcloud config set compute/zone ${zone}

# Port forward minikube (k8) from the VM locally on 8443, and setup corresponding kubectl entry
export CLOUDSDK_CORE_DISABLE_PROMPTS=1
gcloud compute ssh --internal-ip minikube@${vm_name} -- -4 -f -NL 8443:192.168.10.2:8443
gcloud compute scp --internal-ip --recurse minikube@${vm_name}:~/.minikube/profiles/cloudbuild /tmp/cloudbuild
kubectl config set-cluster minikube --server="https://127.0.0.1:8443" --insecure-skip-tls-verify
kubectl config set-credentials cloudbuild --client-certificate=/tmp/cloudbuild/client.crt --client-key=/tmp/cloudbuild/client.key
kubectl config set-context minikube --cluster=minikube --user=cloudbuild
kubectl config use-context minikube

# Run tests
go test -v --timeout 30m ./...
# Capture the exit code
exit_code=$?

# Kill ssh sessions
pkill ssh

# TODO delete VM

# Check if the process exited successfully (exit code 0)
if [[ $exit_code -eq 0 ]]; then
    echo "All tests passed!"
else
    echo "Some tests failed (see logs for details)."
    exit 1
fi