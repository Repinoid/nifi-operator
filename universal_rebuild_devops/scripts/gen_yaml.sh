#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SERVICE_ID="${1:-}"

if [[ -z "$SERVICE_ID" ]]; then
  echo "Usage: gen_yaml.sh <service_id>" >&2
  exit 1
fi

TOKEN_FILE="${TOKEN_FILE:-$ROOT_DIR/terra.token}"

if [[ -z "$TOKEN_FILE" || ! -f "$TOKEN_FILE" ]]; then
  echo "TOKEN_FILE is not set and terra.token was not found in project root." >&2
  exit 1
fi

export NUBES_API_TOKEN
NUBES_API_TOKEN="$(cat "$TOKEN_FILE")"
export NUBES_SERVICE_ID="$SERVICE_ID"

cd "$ROOT_DIR"
go run ./tools/service_params_gen/main.go
