#!/bin/bash

# Configuration
DOMAIN_FILE="DOMAIN"
OPERATOR_TEMPLATE="00-nifi-operator.yaml.template"
CLUSTER_TEMPLATE="02-nifi-cluster.yaml.template"

OPERATOR_OUTPUT="00-nifi-operator.yaml"
CLUSTER_OUTPUT="02-nifi-cluster.yaml"

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
# Note: The operator manifest works with the API Group `nifi.kube5s.ru`. 
# We generally DO NOT replace the domain here because it defines the CRD structure.
cp "$OPERATOR_TEMPLATE" "$OPERATOR_OUTPUT"
echo "Generated $OPERATOR_OUTPUT"

# 2. Process Cluster Manifest
# We replace NIFI.DOMEN.RU with the actual domain for Ingress, Proxy, etc.
# We DO NOT replace `apiVersion: nifi.kube5s.ru` (it was preserved in the template).
cp "$CLUSTER_TEMPLATE" "$CLUSTER_OUTPUT"

sed -i "s/NIFI.DOMEN.RU/$DOMAIN/g" "$CLUSTER_OUTPUT"

# 3. Generate Cert-Manager Resources (Internal & External)
# User Requirement: Use cert-manager for automatic updates.

echo "Generating Cert-Manager Manifests..."

CERT_MAN_DIR="../cert-manager"
CM_INTERNAL_OUTPUT="01-cert-manager-internal.yaml"
CM_EXTERNAL_OUTPUT="03-ingress-certificate.yaml"

# --- A. Internal Infrastructure (CA, Issuer, Internal Certs) ---
echo "  [1/2] Assembling Internal Certs ($CM_INTERNAL_OUTPUT)..."

# 0. Secret with CA (CRITICAL: Must be applied first)
cat "$CERT_MAN_DIR/02-ca-secret-fixed.yaml" > "$CM_INTERNAL_OUTPUT"
echo "---" >> "$CM_INTERNAL_OUTPUT"

# 1. Internal CA Issuer Setup
cat "$CERT_MAN_DIR/05-internal-ca-issuer.yaml" >> "$CM_INTERNAL_OUTPUT"
echo "---" >> "$CM_INTERNAL_OUTPUT"

# 2. Internal Node Certificate
cat "$CERT_MAN_DIR/06-internal-nifi-certificate.yaml" >> "$CM_INTERNAL_OUTPUT"
echo "---" >> "$CM_INTERNAL_OUTPUT"

# 3. Operator/Client Certificate (Fixing issuer reference)
cat "$CERT_MAN_DIR/04-client-certificate.yaml" | \
  sed 's/nifi-mtls-ca-issuer/nifi-internal-ca-issuer/g' >> "$CM_INTERNAL_OUTPUT"

# --- POST-PROCESSING: Inject Domain into Internal Cert ---
# NiFi 2.0+ requires the Internal Certificate to be valid for the External Domain
# because the Ingress sends the External Domain as the SNI value.
sed -i "s/NIFI_EXTERNAL_DOMAIN_PLACEHOLDER/$DOMAIN/g" "$CM_INTERNAL_OUTPUT"

echo "        Generated $CM_INTERNAL_OUTPUT"

# --- B. External Ingress Certificate (Let's Encrypt) ---
echo "  [2/2] Creating Ingress Certificate ($CM_EXTERNAL_OUTPUT)..."

cat > "$CM_EXTERNAL_OUTPUT" <<EOF
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: nifi-tls
  namespace: nifi
spec:
  secretName: nifi-tls-secret
  issuerRef:
    # IMPORTANT: Assuming 'letsencrypt-prod' ClusterIssuer exists!
    name: letsencrypt-prod
    kind: ClusterIssuer
  commonName: $DOMAIN
  dnsNames:
  - $DOMAIN
EOF

echo "        Generated $CM_EXTERNAL_OUTPUT"

# Cleanup NOT needed as we are not using tmp files


echo "Generated $CLUSTER_OUTPUT with Host configuraton for $DOMAIN"
echo "Done."
