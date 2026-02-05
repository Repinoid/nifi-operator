#!/bin/bash
set -e

# === CONFIGURATION ===
ROOT_DIR="/home/naeel/terra"
TEST_DIR="$ROOT_DIR/tests/lifecycle_scenario"
TF_LOG_FILE="$TEST_DIR/test_output_tubulus.log"

# 1. Build Provider
echo ">>> Building Provider..."
cd $ROOT_DIR
go build -o terraform-provider-nubes
if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

# 2. Setup Local Mirror in Test Dir
PLUGIN_DIR="$TEST_DIR/plugins/terraform.local/nubes/nubes/1.0.0/linux_amd64"
mkdir -p "$PLUGIN_DIR"
cp terraform-provider-nubes "$PLUGIN_DIR/"

# 3. Setup Test Directory
echo ">>> Setting up Test Directory: $TEST_DIR"
cd $TEST_DIR

# 4. Init with Plugin Dir
echo ">>> Initializing Terraform with local plugin mirror..."
rm -rf .terraform .terraform.lock.hcl
terraform init -plugin-dir="$TEST_DIR/plugins" > /dev/null

echo "Starting Tubulus Lifecycle Test" > $TF_LOG_FILE

# Step 1: Create
echo ">>> Step 1: Initial Creation" | tee -a $TF_LOG_FILE
terraform apply -auto-approve | tee -a $TF_LOG_FILE
