#!/usr/bin/env bash
set -euo pipefail

# Create namespace if missing and apply kustomize overlay for dev
kubectl create namespace terra-dev --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -k k8s/overlays/dev

echo "Applied kustomize overlay for terra-dev."
echo "To init terraform backend for dev: terraform init -reconfigure -backend-config=terraform/backend-dev.hcl"
