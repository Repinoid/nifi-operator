#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

if [ ! -f "DOMAIN" ]; then
    echo "ERROR: DOMAIN file not found"
    exit 1
fi

DOMAIN=$(cat DOMAIN | tr -d '\n\r')

echo "Generating NiFi manifests with DOMAIN=$DOMAIN..."

sed "s/\${DOMAIN}/$DOMAIN/g" 00-nifi-operator.yaml.template > 00-nifi-operator.yaml
echo "Generated 00-nifi-operator.yaml"

sed "s/\${DOMAIN}/$DOMAIN/g" 01-certificates.yaml.template > 01-certificates.yaml
echo "Generated 01-certificates.yaml"

sed "s/\${DOMAIN}/$DOMAIN/g" 02-nifi-cluster.yaml.template > 02-nifi-cluster.yaml
echo "Generated 02-nifi-cluster.yaml"

echo "Done."
