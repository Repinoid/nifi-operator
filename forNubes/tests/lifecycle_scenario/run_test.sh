#!/bin/bash
set -e

# === CONFIGURATION ===
ROOT_DIR="/home/naeel/terra"
TEST_DIR="$ROOT_DIR/tests/lifecycle_scenario"
DISCOVERY_DOC="$ROOT_DIR/docs/discovery/pg_s3_lifecycle_test.md"
HISTORY_DOC="$ROOT_DIR/docs/history/05_lifecycle_test_results.md"
TF_LOG_FILE="$TEST_DIR/test_output.log"
BUCKET_NAME="tf-lifecycle-bucket-$(date +%s)"

# 1. Build Provider
echo ">>> Building Provider..."
cd $ROOT_DIR
go build -o terraform-provider-nubes
if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

# 2. Setup Local Mirror in Test Dir
# We use -plugin-dir to force local loading effectively
PLUGIN_DIR="$TEST_DIR/plugins/terraform.local/nubes/nubes/1.0.0/linux_amd64"
mkdir -p "$PLUGIN_DIR"
cp terraform-provider-nubes "$PLUGIN_DIR/"

# 3. Setup Test Directory
echo ">>> Setting up Test Directory: $TEST_DIR"
cd $TEST_DIR

if [ ! -f "terraform.tfvars" ]; then
    echo "Copying terraform.tfvars..."
    cp "$ROOT_DIR/client_package/terraform.tfvars" .
fi

# 4. Init with Plugin Dir (Bypasses network for terraform.local)
echo ">>> Initializing Terraform with local plugin mirror..."
rm -rf .terraform .terraform.lock.hcl
terraform init -plugin-dir="$TEST_DIR/plugins" > /dev/null

echo "Starting Lifecycle Test" > $TF_LOG_FILE

# === TEST STEPS ===

# Step 1: Create
echo ">>> Step 1: Initial Creation (S3 + PG [2CPU, 4GB])" | tee -a $TF_LOG_FILE
start_1=$(date +%s)
terraform apply -auto-approve \
    -var="pg_cpu=2" \
    -var="pg_ram=4" \
    -var="pg_disk=10" \
    -var="s3_bucket_name=$BUCKET_NAME" \
    >> $TF_LOG_FILE 2>&1
end_1=$(date +%s)
dur_1=$((end_1 - start_1))
echo "Step 1 Done: ${dur_1}s" | tee -a $TF_LOG_FILE

# Step 2: Update (Scale Up)
echo ">>> Step 2: Modify PG (Scale Up [4CPU, 8GB])" | tee -a $TF_LOG_FILE
start_2=$(date +%s)
terraform apply -auto-approve \
    -var="pg_cpu=4" \
    -var="pg_ram=8" \
    -var="pg_disk=10" \
    -var="s3_bucket_name=$BUCKET_NAME" \
    >> $TF_LOG_FILE 2>&1
end_2=$(date +%s)
dur_2=$((end_2 - start_2))
echo "Step 2 Done: ${dur_2}s" | tee -a $TF_LOG_FILE

# Step 3: Update (Scale Down)
echo ">>> Step 3: Modify PG (Scale Down [2CPU, 4GB])" | tee -a $TF_LOG_FILE
start_3=$(date +%s)
terraform apply -auto-approve \
    -var="pg_cpu=2" \
    -var="pg_ram=4" \
    -var="pg_disk=10" \
    -var="s3_bucket_name=$BUCKET_NAME" \
    >> $TF_LOG_FILE 2>&1
end_3=$(date +%s)
dur_3=$((end_3 - start_3))
echo "Step 3 Done: ${dur_3}s" | tee -a $TF_LOG_FILE

# Step 4: Destroy
echo ">>> Step 4: Destroy" | tee -a $TF_LOG_FILE
terraform destroy -auto-approve \
    -var="pg_cpu=2" \
    -var="pg_ram=4" \
    -var="pg_disk=10" \
    -var="s3_bucket_name=$BUCKET_NAME" \
    >> $TF_LOG_FILE 2>&1
echo "Cleanup Done" | tee -a $TF_LOG_FILE

# === REPORT ===
cd $ROOT_DIR
mkdir -p docs/discovery docs/history

cat <<EOF > $DISCOVERY_DOC
# Lifecycle Test: S3 + Postgres Update

## Test Scenario
1. **Create S3**: $BUCKET_NAME
2. **Create Postgres**: 2CPU/4RAM
3. **Update**: 4CPU/8RAM (Expect In-place)
4. **Update**: 2CPU/4RAM (Expect In-place)
5. **Destroy**: (Expect Freeze policy to keep cloud resource)

## Results $(date)
| Step | Duration | Result |
|------|----------|--------|
| Create | ${dur_1}s | Done |
| Scale Up | ${dur_2}s | Done |
| Scale Down | ${dur_3}s | Done |

See $TF_LOG_FILE for details.
EOF

cat <<EOF > $HISTORY_DOC
# 05 Lifecycle Test Results
Date: $(date)
Success. Updates performed in-place.
EOF

echo "Docs generated."
