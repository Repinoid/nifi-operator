# Настройка NIFI
## Надо иметь три (суб)домена. 
- **NIFI.DOMEN.RU** - собственно для NIFI
- **API.DOMEN.RU** - для NIFI API
- **KEYCLOAK.DOMEN.RU** - keycloak, опционально, если нет иного функционирующего keycloak

### Во всех файлах заменить шаблонные 
- **NIFI.DOMEN.RU** 
- **API.DOMEN.RU**
- **KEYCLOAK.DOMEN.RU** на свои реальные

### Деплой/Настройка Keycloak
- инструкция в файле **KeyCloak.md**
- `Client Secret` - что был запомнен - прописать в **clientSecret** в **nifi.yaml**

### nifi.yaml
- для запуска достаточно заменить **NIFI.DOMEN.RU, API.DOMEN.RU, KEYCLOAK.DOMEN.RU** на реальные, **clientSecret** из настроек ***keycloak*** и всё.
- ⚠️ **ВАЖНО**: После первичной настройки установите `overWrite: false` в nifi.yaml, иначе при рестарте потеряете пользователей и policies!

### НОВОЕ в nifi.yaml - downloadFiles:
- перечисленные файлы будут загружены в папку ***/opt/nifi/nifi-current/lib/***

### Создаём namespace
- ```kubectl create namespace nifi```

### Создаём сертификаты

```bash
openssl s_client \
  -connect KEYCLOAK.DOMEN.RU:443 \
  -servername KEYCLOAK.DOMEN.RU \
  -showcerts \
  </dev/null 2>/dev/null \
  | sed -n '/-----BEGIN CERTIFICATE-----/,/-----END CERTIFICATE-----/p' \
  | kubectl create secret generic keycloak-nifi-secret \
      --from-file=tls.crt=/dev/stdin \
      -n nifi
```  

- `kubectl apply -f nifi-tls-secret.yaml -n nifi`

```bash
kubectl apply -f certificate-api.yaml -n nifi
```

```bash
kubectl apply -f nifi-cert-generator-job.yaml -n nifi && \
kubectl wait --for=condition=complete job/nifi-cert-generator -n nifi --timeout=300s && \
kubectl delete job nifi-cert-generator -n nifi
```

- `kubectl get secret -n nifi`  

Должно быть такое -
- keycloak-nifi-secret            Opaque              1      3m31s  
- nifi-admin-gateway-tls-secret   kubernetes.io/tls   2      6m19s  
- nifi-mtls-ca-secret             Opaque              1      16s  
- nifi-tls-secret                 kubernetes.io/tls   2      6m31s  
- nificl-sa-cert                  kubernetes.io/tls   2      16s

### Запускаем оператор
- `kubectl apply -f nifi-operator-deployment-v09.yaml`
### Запускаем NIFI
- `kubectl apply -f nifi.yaml -n nifi`
### Отслеживаем деплой
- `kubectl get pod -w -n nifi`


## В браузере https://NIFI.DOMEN.RU
- входим юзером которого создали в keycloak и поместили в группу nifi_admins
- в UI **группе** nifi_clients назначаем policy `view the user interface` (иначе пользователем из этой группы в UI не войдёте и зациклитесь на входе)

### https://API.DOMEN.RU 
- прокси, ретранслирует запросы к NIFI API. 
- Доступ **пока без пароля и сертификатов**

- пример - `https://API.DOMEN.RU/nifi-api/tenants/users`
- REST API definition for Apache NiFi web services - ***https://nifi.apache.org/nifi-docs/rest-api.html***

<hr>

- проверить загрузку JAR
- `kubectl exec -it nificl-0 -- ls -l /opt/nifi/nifi-current/lib/`
<br>nifi-wrapper
- `kubectl logs nificl-0 -n nifi | grep 🐺 `
<br>init copy-config
- `kubectl logs nificl-0 -c nifi-copy-config -n nifi | grep 🐸`  

<hr>

hardcoded
- keystorePassword := "changeme"
- truststorePassword := "changeme"
- keyPassword := "changeme"

Каждый компонент оператора использует уникальный эмодзи-символ (животное) для быстрой идентификации в логах и Kubernetes Events:

- 🦩 **ConfigMap** — фламинго (конфигурационные файлы NiFi)
- 🐧 **Headless Service** — пингвин (внутренний DNS для StatefulSet)
- 🐬 **Service** — дельфин (балансировщик для доступа к кластеру)
- 🦁 **StatefulSet** — лев (основные Pod'ы NiFi)
- 🐍 **Admin Gateway** — змея (HTTP прокси без client cert)
- 🦊 **Ingress** — лиса (внешний доступ к NiFi UI)
- 🦎 **Registry Clients** — ящерица (подключение к NiFi Registry)
- 🐘 **StorageClass** — слон (хранилище данных)
- 🦈 **Finalizer** — акула (очистка ресурсов при удалении)