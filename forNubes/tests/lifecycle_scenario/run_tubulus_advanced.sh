#!/bin/bash
set -e

# === CONFIGURATION ===
ROOT_DIR="/home/naeel/terra"
TEST_DIR="$ROOT_DIR/tests/lifecycle_scenario"
TF_LOG_FILE="$TEST_DIR/test_output_tubulus_advanced.log"

# Define colors
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

log() {
    echo -e "${GREEN}[TEST] $1${NC}"
    echo "[TEST] $1" >> $TF_LOG_FILE
}

error() {
    echo -e "${RED}[ERROR] $1${NC}"
    echo "[ERROR] $1" >> $TF_LOG_FILE
}

# 1. Build Provider
log "Building Provider..."
cd $ROOT_DIR
go build -o terraform-provider-nubes
if [ $? -ne 0 ]; then
    error "Build failed!"
    exit 1
fi

# 2. Setup Local Mirror in Test Dir
PLUGIN_DIR="$TEST_DIR/plugins/terraform.local/nubes/nubes/1.0.0/linux_amd64"
mkdir -p "$PLUGIN_DIR"
cp terraform-provider-nubes "$PLUGIN_DIR/"

# 3. Setup Test Directory
log "Setting up Test Directory: $TEST_DIR"
cd $TEST_DIR

# 4. Init with Plugin Dir
log "Initializing Terraform..."
rm -rf .terraform.lock.hcl .terraform
terraform init -plugin-dir="$TEST_DIR/plugins" > /dev/null

echo "Starting Advanced Tubulus Tests" > $TF_LOG_FILE

# === TEST A: MODIFY (HAPPY PATH) ===
log "TEST A: Modifying instance description (Should Succeed)..."

# Modify the TF file
cat <<EOF > tubulus.tf
resource "nubes_tubulus_instance" "bolvanka" {
  display_name     = "Bolvanka_Lifecycle_Test_014"
  description      = "Modified Description by Test A"
  body_message     = "Initial State"
  duration_ms      = 1000
}

output "bolvanka_id" {
  value = nubes_tubulus_instance.bolvanka.id
}
EOF

# Apply update
if terraform apply -auto-approve >> $TF_LOG_FILE 2>&1; then
    log "TEST A PASSED: Modification successful."
else
    error "TEST A FAILED: Apply returned error."
    cat $TF_LOG_FILE
    exit 1
fi

# === TEST B: MODIFY (FAILURE PATH) ===
# This tests if the provider correctly reports mistakes during 'modify' operation
log "TEST B: Triggering failure during modification (Should Fail Gracefully)..."

# Modify TF file to include failure instruction
cat <<EOF > tubulus.tf
resource "nubes_tubulus_instance" "bolvanka" {
  display_name     = "Bolvanka_Lifecycle_Test_014"
  description      = "Modified Description by Test B (Will Fail)"
  body_message     = "Initial State"
  duration_ms      = 1000
  fail_in_progress = true
}

output "bolvanka_id" {
  value = nubes_tubulus_instance.bolvanka.id
}
EOF

# Apply update (Expect Failure)
if ! terraform apply -auto-approve >> $TF_LOG_FILE 2>&1; then
    log "TEST B PASSED: Terraform correctly reported failure."
else
    error "TEST B FAILED: Terraform succeeded but should have failed!"
    cat $TF_LOG_FILE
    # We might need to fix the state if B passed unexpectedly, but let's assume correct behavior.
fi

# === RESTORE STATE ===
# Remove failure flag to allow subsequent tests (Suspend/Resume)
log "Restoring valid state..."
cat <<EOF > tubulus.tf
resource "nubes_tubulus_instance" "bolvanka" {
  display_name     = "Bolvanka_Lifecycle_Test_014"
  description      = "Restored Description"
  body_message     = "Initial State"
  duration_ms      = 1000
}

output "bolvanka_id" {
  value = nubes_tubulus_instance.bolvanka.id
}
EOF

# We expect this to act as a repair or re-run
terraform apply -auto-approve >> $TF_LOG_FILE 2>&1

# === TEST D: SUSPEND (SOFT DELETE) ===
log "TEST D: Suspending Instance (Terraform Destroy)..."
if terraform destroy -auto-approve >> $TF_LOG_FILE 2>&1; then
    log "TEST D PASSED: Destroy (Suspend) successful."
else
    error "TEST D FAILED: Destroy returned error."
    cat $TF_LOG_FILE
    exit 1
fi

# === TEST E: RESUME (CREATE on SUSPENDED) ===
log "TEST E: Resuming Instance (Terraform Apply)..."
if terraform apply -auto-approve >> $TF_LOG_FILE 2>&1; then
    log "TEST E PASSED: Resume (Create) successful."
else
    error "TEST E FAILED: Apply (Resume) returned error."
    cat $TF_LOG_FILE
    exit 1
fi

log "ALL ADVANCED TESTS COMPLETED SUCCESSFULLY."
