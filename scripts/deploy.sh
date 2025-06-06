#!/bin/bash

set -e

ENVIRONMENT=${1:-staging}
IMAGE_TAG=${2:-latest}

echo "Deploying to $ENVIRONMENT with image tag $IMAGE_TAG"

# Update the image tag in deployment
sed -i "s|image: gitlab-mr-conformity-bot:.*|image: gitlab-mr-conformity-bot:$IMAGE_TAG|g" deployments/k8s/deployment.yaml

# Apply Kubernetes manifests
kubectl apply -f deployments/k8s/deployment.yaml