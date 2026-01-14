# NiFi Deployment

## Prerequisites

```bash
kubectl create namespace nifi
kubectl create secret generic keycloak-nifi-secret \
  --from-file=tls.crt=keycloak-cert.pem -n nifi
```

## Deploy

```bash
# 1. Generate manifests (edit DOMAIN if needed)
./generate_manifests.sh

# 2. Apply
kubectl apply -f 00-nifi-operator.yaml
kubectl apply -f 01-certificates.yaml
kubectl apply -f 02-nifi-cluster.yaml

# 3. Wait
kubectl get pods -n nifi -w
```

## Config

**Operator:** `naeel/nifi-operator:v1.7.52`  
**NiFi:** `apache/nifi:2.6.0`  
**OIDC:** `keycloak.k8c.ru/realms/nifi-realm`  
**Admin:** `regadmin` (from `preferred_username`)  
**Ingress:** `nifi.${DOMAIN}`, `api2.${DOMAIN}` (admin gateway)
