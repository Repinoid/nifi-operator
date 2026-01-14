# NiFi Registry Deployment

## Prerequisites

```bash
kubectl create namespace registry
# NiFi cluster в namespace nifi должен быть запущен
```

## Deploy

```bash
# 1. Generate manifests (edit DOMAIN if needed)
./generate_manifests.sh

# 2. Apply
kubectl apply -f 00-registry-operator.yaml
kubectl apply -f 02-nifi-registry.yaml

# 3. Wait
kubectl get pods -n registry -w
```

## Config

**Operator:** `naeel/nifi-registry-operator:v2.12.9`  
**Registry:** `apache/nifi-registry:2.6.0`  
**PostgreSQL:** автоматически (password: `registrypass`)  
**Identities:**
- `initialAdminIdentity`: `CN=nifi.${DOMAIN}`
- `nifiIdentity`: `CN=nifi.${DOMAIN}`

**OIDC:** `keycloak.k8c.ru/realms/nifi-realm`  
**Admin:** `regadmin`
