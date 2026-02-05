#!/usr/bin/env bash
set -euo pipefail

# Build and optionally push the registry-server image with docs handler
# Usage: ./scripts/build-and-push-registry-server.sh <image-repo> <tag>
IMAGE_REPO=${1:-naeel}
TAG=${2:-docs-dev}

cd operator
make build-registry IMAGE_REPO=${IMAGE_REPO} TAG=${TAG}

echo "Built image: ${IMAGE_REPO}/terraform-registry-server:${TAG}"

echo "To push: docker push ${IMAGE_REPO}/terraform-registry-server:${TAG}"

echo "To deploy to cluster (example):"
echo "  kubectl apply -f operator/manifests/06-registry-server-docs.yaml" 

echo "and edit the file to replace IMAGE_REPO/TAG with your registry and tag before applying."
