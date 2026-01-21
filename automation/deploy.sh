#!/bin/bash
# deploy.sh - Renders manifests and applies them to Kubernetes cluster

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "🛠 Step 1: Generating manifests..."
./render.sh

echo "📦 Step 2: Creating namespaces if needed..."
kubectl create namespace nifi --dry-run=client -o yaml | kubectl apply -f -
kubectl create namespace registry --dry-run=client -o yaml | kubectl apply -f -

echo "🔐 Step 3: applying manifests..."

# Apply Operators (CRDs first, included in files)
kubectl apply -f dist/nifi-operator.yaml
kubectl apply -f dist/registry-operator.yaml

echo "🔐 Step 3.1: Applying MTLS certificates..."
kubectl apply -f dist/mtls-certs.yaml

echo "⏳ Waiting for operators to be ready..."
kubectl rollout status deployment/oper-controller-manager -n nifi --timeout=90s
kubectl rollout status deployment/oper-registry-controller-manager -n registry --timeout=90s

# Apply Cluster Resources
kubectl apply -f dist/nifi-cluster.yaml
kubectl apply -f dist/registry-cr.yaml

echo "🚀 DONE! NiFi and Registry are deploying."
echo "Check status: kubectl get nificluster -n nifi && kubectl get nifiregistry -n registry"
