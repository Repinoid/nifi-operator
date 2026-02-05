# k8s

Здесь лежат Kubernetes‑манифесты, которые поднимают реестр Terraform‑провайдеров. Это рабочие файлы для текущего деплоя.

## Что реально используется сейчас
- [registry-deployment-new.yaml](registry-deployment-new.yaml) — deployment `registry-server` с настройкой S3.
- [registry-ingress-new.yaml](registry-ingress-new.yaml) — ingress для `terra.k8c.ru` (cert‑manager + Let's Encrypt).