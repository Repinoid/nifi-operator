#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

if [ ! -f "DOMAIN" ]; then
    echo "ERROR: DOMAIN file not found"
    exit 1
fi

DOMAIN=$(cat DOMAIN | tr -d '\n\r')

echo "Generating Registry manifests with DOMAIN=$DOMAIN..."

sed "s/\${DOMAIN}/$DOMAIN/g" 00-registry-operator.yaml.template > 00-registry-operator.yaml
echo "Generated 00-registry-operator.yaml"

sed "s/\${DOMAIN}/$DOMAIN/g" 02-nifi-registry.yaml.template > 02-nifi-registry.yaml
echo "Generated 02-nifi-registry.yaml"

echo "Done."
