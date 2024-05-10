#!/bin/bash

set -eux

show_help() {
  echo "Usage: $0 [-p project] [-s service_account]" 
}

while getopts "hp:s:" opt; do
  case $opt in
    h)
      show_help
      exit 0
      ;;
    p)
      project="$OPTARG"
      ;;
    s)
      service_account="$OPTARG"
      ;;
    \?)
      echo "Invalid option: -$OPTARG"
      exit 1
      ;;
  esac
done
shift $((OPTIND-1)) # Shift arguments past the options

echo "Retrieving token for service account ${service_account} in project ${project}"
project_num=$(gcloud projects describe "${project}" --format="value(projectNumber)")
token=$(gcloud auth print-access-token --impersonate-service-account "${service_account}")
if [ -z ${token} ]; then
    echo "Failed to generate token!"
    return 1
fi

# Post Project Info and Token to Spoofed Metadata Server
echo "Attempting to impersonate service account: ${service_account} to obtain access token"
expiry=$(date -Is --date "3600 seconds")
token_data="{\"access_token\": \"${token}\", \"email\": \"${service_account}\", \"expiry\": \"${expiry}\", \"scopes\":[]}"
build_data="{\"project_id\": \"${project}\",\"project_num\": ${project_num}}"
curl -sH "Content-Type: application/json" -X POST -d "${build_data}" http://192.168.10.5/build
curl -sH "Content-Type: application/json" -X POST -d "${token_data}" http://192.168.10.5/token

# Attach the secret for pulling images from AR to the default KSA
echo "Refreshing K8 image pull secret"
kubectl create secret docker-registry regcred \
    --docker-server="*-docker.pkg.dev" \
    --docker-username=oauth2accesstoken \
    --docker-password=${token} \
    --docker-email=${service_account} \
    -o yaml --dry-run=client | kubectl apply -f -
kubectl patch serviceaccount default -p '{"imagePullSecrets": [{"name": "regcred"}]}'