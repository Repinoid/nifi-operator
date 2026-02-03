#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SERVICE_ID="${1:-}"

if [[ -z "$SERVICE_ID" ]]; then
  echo "Usage: build_provider.sh <service_id>" >&2
  exit 1
fi

cd "$ROOT_DIR"

YAML_FILE=$(grep -l "service_id: ${SERVICE_ID}" "$ROOT_DIR"/resources_yaml/*.yaml | head -n 1 || true)
if [[ -z "$YAML_FILE" ]]; then
  echo "YAML for service_id ${SERVICE_ID} not found. Run gen_yaml.sh first." >&2
  exit 1
fi
SERVICE_NAME="$(basename "$YAML_FILE" .yaml)"

go run ./tools/gen/main.go

go build -o "$ROOT_DIR/terraform-provider-nubes" .

# Append minimal resource block to test_dummy/main.tf
go run ./tools/min_tf/main.go "$SERVICE_NAME" >> "$ROOT_DIR/test_dummy/main.tf"
