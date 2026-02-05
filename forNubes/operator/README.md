# Terra Operator Manifests

## Minimal Setup for Namespace `terra`

### 1. Structure
- **Namespace**: `terra`
- **Storage**: внешний S3 (Nubes Cloud).
- **CRD**: `TerraformProviderRelease` definitions.

### 2. How to apply
```bash
kubectl apply -f operator/manifests/00-namespace.yaml
kubectl apply -f operator/manifests/02-crd.yaml
kubectl apply -f operator/manifests/04-operator-deployment.yaml
kubectl apply -f operator/manifests/05-registry-server.yaml
```

### 3. Usage
Create a release request:
```yaml
apiVersion: terra.core.nubes.ru/v1alpha1
kind: TerraformProviderRelease
metadata:
  name: mycloud-v0-1-0
  namespace: terra
spec:
  providerName: "mycloud"
  version: "0.1.0"
  gitRepo: "https://github.com/Start-Ops/terraform-provider-mycloud.git"
  gitRef: "v0.1.0"
```

---

## Простыми словами: что это и зачем

Это набор манифестов и небольшой сервис, который автоматизирует сборку Terraform‑провайдеров.
Идея простая: я описываю в CRD, какой провайдер и какую версию собрать, а оператор сам запускает Job,
собирает бинарники и складывает их в хранилище. Дальше registry‑server отдает эти артефакты Terraform‑клиентам.

Зачем это нужно:
- чтобы не собирать провайдеры руками;
- чтобы хранить артефакты в одном месте;
- чтобы Terraform мог скачивать провайдеры по обычному источнику.
