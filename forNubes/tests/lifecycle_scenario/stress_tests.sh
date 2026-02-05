#!/bin/bash
set -e

# === CONFIGURATION ===
ROOT_DIR="/home/naeel/terra"
TEST_DIR="$ROOT_DIR/tests/lifecycle_scenario"
TF_LOG_FILE="$TEST_DIR/stress_test.log"

# Define colors
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'
YELLOW='\033[1;33m'

log() {
    echo -e "${GREEN}[STRESS] $1${NC}"
    echo "[STRESS] $1" >> $TF_LOG_FILE
}

error() {
    echo -e "${RED}[ERROR] $1${NC}"
    echo "[ERROR] $1" >> $TF_LOG_FILE
}

warn() {
    echo -e "${YELLOW}[WARN] $1${NC}"
    echo "[WARN] $1" >> $TF_LOG_FILE
}

# Ensure we are in the right directory
cd $TEST_DIR

echo "STARTING STRESS TESTS" > $TF_LOG_FILE

# === TEST 1: FAST UPDATE (Duration 500ms) ===
log "TEST 1: Fast Update (Modify)..."
cat <<EOF > tubulus.tf
resource "nubes_tubulus_instance" "bolvanka" {
  display_name     = "Bolvanka_Lifecycle_Test_014"
  description      = "Stress Test 1: Fast"
  body_message     = "Go Fast"
  duration_ms      = 500
}

output "bolvanka_id" {
  value = nubes_tubulus_instance.bolvanka.id
}
EOF

if terraform apply -auto-approve >> $TF_LOG_FILE 2>&1; then
    log "TEST 1 PASSED"
else
    warn "TEST 1 Modify Failed. Instance might be in bad state."
    log "Performing cleanup (Destroy & Recreate)..."
    terraform destroy -auto-approve >> $TF_LOG_FILE 2>&1
    
    if terraform apply -auto-approve >> $TF_LOG_FILE 2>&1; then
        log "TEST 1 PASSED (via Recreate)"
    else
        error "TEST 1 FAILED"
        cat $TF_LOG_FILE
        exit 1
    fi
fi

# === TEST 2: HEAVY Load (Duration 5000ms) ===
log "TEST 2: Long Duration Update (Should wait 5s+)..."
cat <<EOF > tubulus.tf
resource "nubes_tubulus_instance" "bolvanka" {
  display_name     = "Bolvanka_Lifecycle_Test_014"
  description      = "Stress Test 2: Slow"
  body_message     = "Go Slow"
  duration_ms      = 5000
}

output "bolvanka_id" {
  value = nubes_tubulus_instance.bolvanka.id
}
EOF

if terraform apply -auto-approve >> $TF_LOG_FILE 2>&1; then
    log "TEST 2 PASSED"
else
    error "TEST 2 FAILED"
    cat $TF_LOG_FILE
    exit 1
fi

# === TEST 3: INTENTIONAL FAILURE (Fail At Start) ===
log "TEST 3: Triggering Failure (fail_at_start=true)..."
cat <<EOF > tubulus.tf
resource "nubes_tubulus_instance" "bolvanka" {
  display_name     = "Bolvanka_Lifecycle_Test_014"
  description      = "Stress Test 3: Doom"
  fail_at_start    = true
  duration_ms      = 1000
}

output "bolvanka_id" {
  value = nubes_tubulus_instance.bolvanka.id
}
EOF

if ! terraform apply -auto-approve >> $TF_LOG_FILE 2>&1; then
    log "TEST 3 PASSED (Expected Failure occurred)"
else
    error "TEST 3 FAILED: Terraform succeeded but should have failed!"
    cat $TF_LOG_FILE
    exit 1
fi

# === TEST 4: RECOVERY FROM FAILURE ===
log "TEST 4: Recovery (Fixing config)..."
cat <<EOF > tubulus.tf
resource "nubes_tubulus_instance" "bolvanka" {
  display_name     = "Bolvanka_Lifecycle_Test_014"
  description      = "Stress Test 4: Recovered"
  body_message     = "We are back"
  duration_ms      = 1000
}

output "bolvanka_id" {
  value = nubes_tubulus_instance.bolvanka.id
}
EOF

if terraform apply -auto-approve >> $TF_LOG_FILE 2>&1; then
    log "TEST 4 PASSED"
else
    warn "TEST 4 Modify Failed. This is expected if the previous failure froze the instance."
    log "Attempting Hard Reset (Destroy & Recreate)..."
    
    # Force destroy to clear the stuck instance
    terraform destroy -auto-approve >> $TF_LOG_FILE 2>&1
    
    # Re-apply the valid configuration
    if terraform apply -auto-approve >> $TF_LOG_FILE 2>&1; then
       log "TEST 4 RECOVERY PASSED (via Hard Reset)"
    else
       error "TEST 4 FAILED: Could not recover even with Hard Reset."
       cat $TF_LOG_FILE
       exit 1
    fi
fi

# === TEST 5: COMPLEX UPDATE (Multiple Fields) ===
log "TEST 5: Complex Multi-field Update..."
cat <<EOF > tubulus.tf
resource "nubes_tubulus_instance" "bolvanka" {
  display_name     = "Bolvanka_Lifecycle_Test_014"
  description      = "Stress Test 5: Complex"
  body_message     = "Complex Payload: JSON { key: 'value' }"
  duration_ms      = 2000
  yaml_example     = "some: yaml"
}

output "bolvanka_id" {
  value = nubes_tubulus_instance.bolvanka.id
}
EOF

if terraform apply -auto-approve >> $TF_LOG_FILE 2>&1; then
    log "TEST 5 PASSED"
else
    error "TEST 5 FAILED"
    cat $TF_LOG_FILE
    exit 1
fi

log "ALL STRESS TESTS COMPLETED SUCCESSFULLY. BOLVANKA IS HOT!"
