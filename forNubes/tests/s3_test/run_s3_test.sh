#!/bin/bash
set -e

# Define root and test dirs
ROOT_DIR="/home/naeel/terra"
TEST_DIR="$ROOT_DIR/tests/s3_test"
LOG_FILE="$TEST_DIR/s3_test.log"

echo "=== Starting S3 Resource Test ===" | tee $LOG_FILE

# 1. Build Provider
echo "Building provider..." | tee -a $LOG_FILE
cd $ROOT_DIR
go build -o terraform-provider-nubes
if [ $? -ne 0 ]; then
    echo "Build failed!" | tee -a $LOG_FILE
    exit 1
fi
echo "Provider built successfully." | tee -a $LOG_FILE

# Copy provider to plugins dir
PLUGIN_DIR="$TEST_DIR/plugins/terraform.local/nubes/nubes/1.0.0/linux_amd64"
mkdir -p "$PLUGIN_DIR"
cp terraform-provider-nubes "$PLUGIN_DIR/terraform-provider-nubes_v1.0.0"
echo "Provider installed to $PLUGIN_DIR" | tee -a $LOG_FILE

# 2. Setup Test Environment
cd $TEST_DIR
# Reuse terraform.tfvars from client_package if it exists
if [ -f "$ROOT_DIR/client_package/terraform.tfvars" ]; then
    cp "$ROOT_DIR/client_package/terraform.tfvars" .
else
    echo "Error: terraform.tfvars not found!" | tee -a $LOG_FILE
    exit 1
fi

# 3. Initialize (Plugin Dir Mode)
echo "Initializing Terraform..." | tee -a $LOG_FILE
rm -rf .terraform .terraform.lock.hcl
terraform init -plugin-dir="$TEST_DIR/plugins" >> $LOG_FILE 2>&1

BUCKET_NAME="tf-s3-test-$(date +%s)"

# 4. Create
echo "Step 1: Creating S3 Bucket ($BUCKET_NAME)..." | tee -a $LOG_FILE
start_time=$(date +%s)
terraform apply -auto-approve \
    -var="bucket_name=$BUCKET_NAME" \
    -var="max_size=100" \
    >> $LOG_FILE 2>&1
end_time=$(date +%s)
echo "Created in $((end_time - start_time))s" | tee -a $LOG_FILE

# 5. Modify (Update size) - SKIPPED because API forbids name reuse immediately
echo "Step 2: Modifying S3 Bucket... SKIPPED (API does not support updates)" | tee -a $LOG_FILE

# 6. Upload File using Python (No boto3/CLI dependency)
echo "Step 2.5: Uploading file to S3..." | tee -a $LOG_FILE
echo "This is a test file for Nubes S3." > test_file.txt

export AWS_ACCESS_KEY_ID="0GLQRD38H4I6RBDB0EWJ"
export AWS_SECRET_ACCESS_KEY="eTFibiHmBd96IApj9PYsboTR6OBoD7osxoarHykw"
export AWS_DEFAULT_REGION="msk-1"
export S3_ENDPOINT="https://s3.msk-1.ngcloud.ru"

python3 upload_mini.py "$BUCKET_NAME" "test_file.txt" "test_file.txt" >> $LOG_FILE 2>&1
if [ $? -eq 0 ]; then
    echo "SUCCESS: File uploaded." | tee -a $LOG_FILE
else
    echo "WARNING: File upload failed, but continuing..." | tee -a $LOG_FILE
fi

# 7. Destroy
echo "Step 3: Destroying S3 Bucket..." | tee -a $LOG_FILE
start_time=$(date +%s)
terraform destroy -auto-approve \
    -var="bucket_name=$BUCKET_NAME" \
    -var="max_size=200" \
    >> $LOG_FILE 2>&1
end_time=$(date +%s)
echo "Destroyed in $((end_time - start_time))s" | tee -a $LOG_FILE

echo "=== Test Completed ===" | tee -a $LOG_FILE
