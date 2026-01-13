#!/bin/bash

# Configuration
DOMAIN_FILE="DOMAIN"
OPERATOR_TEMPLATE="00-registry-operator.yaml.template"
REGISTRY_TEMPLATE="02-nifi-registry.yaml.template"

OPERATOR_OUTPUT="00-registry-operator.yaml"
REGISTRY_OUTPUT="02-nifi-registry.yaml"

# Check if domain file exists
if [ ! -f "$DOMAIN_FILE" ]; then
  echo "Error: $DOMAIN_FILE not found. Please create it with your target domain (e.g. nifi.kube5s.ru)"
  exit 1
fi

DOMAIN=$(cat "$DOMAIN_FILE" | tr -d '[:space:]')
if [ -z "$DOMAIN" ]; then
  echo "Error: Domain in $DOMAIN_FILE is empty."
  exit 1
fi

echo "Generating manifests for domain: $DOMAIN"

# 1. Process Operator Manifest
# Note: Usually the operator manifest DOES NOT need domain replacement for the CRD name (API Group).
# However, if you are sure you want to create a clean specific version, we just copy it.
# Warning: Do NOT change `nifi.kube5s.ru` in apiVersion or CRD definitions unless you recompiled the operator!
cp "$OPERATOR_TEMPLATE" "$OPERATOR_OUTPUT"
echo "Generated $OPERATOR_OUTPUT"

# 2. Process Registry Manifest
# Here we replace the Identity CNs with the target domain.
# We DO NOT replace `apiVersion: nifi.kube5s.ru` because that is the static API Group.
cp "$REGISTRY_TEMPLATE" "$REGISTRY_OUTPUT"

# Perform replacement for Identities only
sed -i "s/CN=NIFI.DOMEN.RU/CN=$DOMAIN/g" "$REGISTRY_OUTPUT"

echo "Generated $REGISTRY_OUTPUT with Identities set to CN=$DOMAIN"
echo "Done."
