#!/bin/bash
set -e

# NLI Auto Test Suite
# Создаёт, проверяет и удаляет Tubulus инстансы с AI инструкциями

export TF_CLI_CONFIG_FILE="/home/naeel/terra/dev_override.tfrc"
cd /home/naeel/terra/tests/modify_postgres

RESULTS_FILE="nli_test_results.json"
echo "[]" > $RESULTS_FILE

log_result() {
    local test_name="$1"
    local instruction="$2"
    local operation="$3"
    local duration_ms="$4"
    local body_message="$5"
    local where_fail="$6"
    local status="$7"
    local error="$8"
    
    # Escape quotes for JSON
    instruction=$(echo "$instruction" | sed 's/"/\\"/g')
    body_message=$(echo "$body_message" | sed 's/"/\\"/g')
    error=$(echo "$error" | sed 's/"/\\"/g' | head -c 200)
    
    jq ". += [{
        \"test\": \"$test_name\",
        \"instruction\": \"$instruction\",
        \"operation\": \"$operation\",
        \"duration_ms\": $duration_ms,
        \"body_message\": \"$body_message\",
        \"where_fail\": $where_fail,
        \"status\": \"$status\",
        \"error\": \"$error\",
        \"timestamp\": \"$(date -Iseconds)\"
    }]" $RESULTS_FILE > ${RESULTS_FILE}.tmp && mv ${RESULTS_FILE}.tmp $RESULTS_FILE
}

run_create_test() {
    local name="$1"
    local instruction="$2"
    
    echo "=== Testing: $name ==="
    echo "Instruction: $instruction"
    
    cat > test_auto_nli.tf <<EOF
resource "nubes_tubulus_instance" "auto_test" {
  display_name = "$name"
  description  = "NLI Auto Test"
  instruction  = "$instruction"
}

output "duration" {
  value = nubes_tubulus_instance.auto_test.duration_ms
}

output "body_msg" {
  value = nubes_tubulus_instance.auto_test.body_message
}

output "where_fail" {
  value = nubes_tubulus_instance.auto_test.where_fail
}

output "status" {
  value = nubes_tubulus_instance.auto_test.status
}
EOF
    
    # Plan
    if ! terraform plan -var-file=terraform.tfvars -target=nubes_tubulus_instance.auto_test -no-color > plan_output.txt 2>&1; then
        log_result "$name" "$instruction" "plan" "0" "null" "0" "failed" "$(cat plan_output.txt | tail -5)"
        return 1
    fi
    
    # Apply
    if ! terraform apply -auto-approve -var-file=terraform.tfvars -target=nubes_tubulus_instance.auto_test -no-color > apply_output.txt 2>&1; then
        log_result "$name" "$instruction" "apply" "0" "null" "0" "failed" "$(cat apply_output.txt | tail -5)"
        return 1
    fi
    
    # Extract outputs
    duration=$(terraform output -raw duration 2>/dev/null || echo "0")
    body=$(terraform output -raw body_msg 2>/dev/null || echo "null")
    fail=$(terraform output -raw where_fail 2>/dev/null || echo "0")
    status=$(terraform output -raw status 2>/dev/null || echo "unknown")
    
    log_result "$name" "$instruction" "create" "$duration" "$body" "$fail" "success" ""
    
    echo "✓ Created: duration=$duration, body=$body, fail=$fail, status=$status"
    
    # Cleanup
    terraform destroy -auto-approve -var-file=terraform.tfvars -target=nubes_tubulus_instance.auto_test -no-color > /dev/null 2>&1 || true
    rm -f test_auto_nli.tf
    
    return 0
}

run_modify_test() {
    local name="$1"
    local instruction1="$2"
    local instruction2="$3"
    
    echo "=== Modify Test: $name ==="
    echo "Initial: $instruction1"
    echo "Modified: $instruction2"
    
    # Create initial
    cat > test_auto_nli.tf <<EOF
resource "nubes_tubulus_instance" "auto_test" {
  display_name = "$name"
  description  = "NLI Modify Test"
  instruction  = "$instruction1"
}

output "duration" {
  value = nubes_tubulus_instance.auto_test.duration_ms
}
EOF
    
    terraform apply -auto-approve -var-file=terraform.tfvars -target=nubes_tubulus_instance.auto_test -no-color > /dev/null 2>&1 || return 1
    
    duration1=$(terraform output -raw duration 2>/dev/null || echo "0")
    log_result "$name-initial" "$instruction1" "create" "$duration1" "null" "0" "success" ""
    
    # Modify
    cat > test_auto_nli.tf <<EOF
resource "nubes_tubulus_instance" "auto_test" {
  display_name = "$name"
  description  = "NLI Modify Test"
  instruction  = "$instruction2"
}

output "duration" {
  value = nubes_tubulus_instance.auto_test.duration_ms
}
EOF
    
    if terraform apply -auto-approve -var-file=terraform.tfvars -target=nubes_tubulus_instance.auto_test -no-color > /dev/null 2>&1; then
        duration2=$(terraform output -raw duration 2>/dev/null || echo "0")
        log_result "$name-modified" "$instruction2" "modify" "$duration2" "null" "0" "success" ""
        echo "✓ Modified: $duration1 → $duration2"
    else
        log_result "$name-modified" "$instruction2" "modify" "0" "null" "0" "failed" "modify not supported"
        echo "✗ Modify failed (expected for immutable resource)"
    fi
    
    # Cleanup
    terraform destroy -auto-approve -var-file=terraform.tfvars -target=nubes_tubulus_instance.auto_test -no-color > /dev/null 2>&1 || true
    rm -f test_auto_nli.tf
    
    return 0
}

echo "🚀 Starting NLI Auto Test Suite"
echo "================================"
echo ""

# 10 CREATE tests
run_create_test "Test1-Fast" "сделай быстро"
sleep 2
run_create_test "Test2-Long" "пусть работает очень долго"
sleep 2
run_create_test "Test3-Message" "напиши hello world в вольт"
sleep 2
run_create_test "Test4-Fail" "сломай сразу при старте"
sleep 2
run_create_test "Test5-FailMid" "упади в процессе на втором этапе"
sleep 2
run_create_test "Test6-Complex" "поработай 15 секунд, положи пароль123 и упади на третьем"
sleep 2
run_create_test "Test7-Typo" "зделай нармална на 7 сикунд"
sleep 2
run_create_test "Test8-Minute" "создай на минуту без ошибок"
sleep 2
run_create_test "Test9-Vague" "просто что-нибудь сделай"
sleep 2
run_create_test "Test10-Technical" "duration 25000ms, fail at stage 1, message: SECRET"
sleep 2

# 5 MODIFY tests
run_modify_test "Modify1" "быстро" "медленно на 2 минуты"
sleep 2
run_modify_test "Modify2" "без ошибок" "сломай в середине"
sleep 2
run_modify_test "Modify3" "напиши hello" "напиши goodbye"
sleep 2
run_modify_test "Modify4" "на 10 секунд" "на 30 секунд"
sleep 2
run_modify_test "Modify5" "stage 1" "stage 3"

echo ""
echo "================================"
echo "✅ Test suite completed!"
echo "Results saved to: $RESULTS_FILE"
cat $RESULTS_FILE | jq '.'
