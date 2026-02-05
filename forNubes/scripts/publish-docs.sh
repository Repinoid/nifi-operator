#!/usr/bin/env bash
set -euo pipefail

# Usage: publish-docs.sh <site-dir> <registry-host> <namespace> <name> <version>
SITE_DIR=${1:-site}
REGISTRY_HOST=${2:-terra.k8c.ru}
NAMESPACE=${3:-nubes}
NAME=${4:-nubes}
VERSION=${5:-dev}

# Support both S3_* (New Standard) and MINIO_* (Legacy) variables
ENDPOINT=${S3_ENDPOINT:-${MINIO_ENDPOINT:-}}
ACCESS_KEY=${S3_ACCESS_KEY:-${MINIO_ACCESS_KEY:-}}
SECRET_KEY=${S3_SECRET_KEY:-${MINIO_SECRET_KEY:-}}

if [ -z "$ENDPOINT" ] || [ -z "$ACCESS_KEY" ] || [ -z "$SECRET_KEY" ]; then
  echo "Error: S3_ENDPOINT/S3_ACCESS_KEY/S3_SECRET_KEY must be set"
  exit 2
fi

MC_ALIAS=registry
mc alias set $MC_ALIAS "$ENDPOINT" "$ACCESS_KEY" "$SECRET_KEY" --api S3v4
TARGET="${MC_ALIAS}/terraform-registry/docs/${NAMESPACE}/${NAME}/${VERSION}/"

# Create target bucket path if needed (mc will create directories implicitly when copying)
mc cp --recursive "$SITE_DIR/" "$TARGET"
# Optionally set public policy
mc policy set public "$TARGET" || true

echo "Published docs to: https://${REGISTRY_HOST}/docs/${NAMESPACE}/${NAME}/${VERSION}/"
