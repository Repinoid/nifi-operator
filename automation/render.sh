#!/bin/bash
# render.sh - Generates Kubernetes manifests from templates using deploy.env

set -e

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Source configuration
if [ ! -f "deploy.env" ]; then
    echo "❌ ERROR: deploy.env not found!"
    exit 1
fi
source ./deploy.env
# Export variables for envsubst
export $(grep -E '^[A-Z0-9_]+=' deploy.env | cut -d= -f1)

# Create output directory
OUTPUT_DIR="./dist"
mkdir -p "$OUTPUT_DIR"

echo "🚀 Generating manifests in $OUTPUT_DIR..."

# List of templates to process
TEMPLATES=(
    "templates/nifi-operator.yaml.template"
    "templates/mtls-certs.yaml.template"
    "templates/nifi-cluster.yaml.template"
    "templates/registry-operator.yaml.template"
    "templates/registry-cr.yaml.template"
)

# Helper function to render templates
render_template() {
    local template=$1
    local output="$OUTPUT_DIR/$(basename "${template%.template}")"
    
    # We use perl for precise variable substitution without breaking shell syntax or complex strings
    # But since we use simple ${VAR} syntax, envsubst is the standard devops tool.
    # To avoid replacing k8s variables like $1, we only replace variables from deploy.env
    
    # 1. Get list of variables starting with NIFI_, REGISTRY_, KC_, SYNC_, OPERATOR_
    VARS_TO_SUBST=$(grep -E '^[A-Z0-9_]+=' deploy.env | cut -d= -f1 | sed 's/^/$/g' | tr '\n' ',' | sed 's/,$//')
    
    envsubst "$VARS_TO_SUBST" < "$template" > "$output"
    echo "  ✅ Generated: $output"
}

for t in "${TEMPLATES[@]}"; do
    if [ -f "$t" ]; then
        render_template "$t"
    else
        echo "  ⚠️ Warning: Template $t not found, skipping."
    fi
done

echo "------------------------------------------------"
echo "Done. You can now compare files in $OUTPUT_DIR with the originals."
echo "Example: diff dist/nifi.yaml ../nifi/nifi.yaml"
