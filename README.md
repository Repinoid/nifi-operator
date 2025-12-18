# Настройка NIFI
## Надо иметь три (суб)домена. 
- ``nifi.domen.ru`` - собственно для NIFI
- ``api.domen.ru`` - для NIFI API
- `keycloak.domen.ru` - keycloak, опционально, если нет иного функционирующего keycloak

### Во всех файлах заменить шаблоны nifi.domen.ru, api.domen.ru, keycloak.domen.ru на свои реальные

### Деплой/Настройка Keycloak
- в файле KeyCloak.md

### Создаём namespace
- ```kubectl create namespace nifi```

### Создаём сертификаты

- ` openssl s_client \  
  -connect keycloak.domen.ru:443 \  
  -servername keycloak.domen.ru \  
  -showcerts \  
  </dev/null 2>/dev/null \  
  | sed -n '/-----BEGIN CERTIFICATE-----/,/-----END CERTIFICATE-----/p' \  
  | kubectl create secret generic keycloak-nifi-secret \  
      --from-file=tls.crt=/dev/stdin \  
      -n nifi `  

- `kubectl apply -f nifi-tls-secret.yaml -n nifi`

- `kubectl apply -f certificate-api.yaml -n nifi`

- kubectl apply -f nifi-cert-generator-job.yaml -n nifi && \  
kubectl wait --for=condition=complete job/nifi-cert-generator -n nifi --timeout=300s && \  
kubectl delete job nifi-cert-generator -n nifi  

- `kubectl get secret -n nifi`  
Должно быть такое -
<pre> ``` keycloak-nifi-secret Opaque 1 3m31s nifi-admin-gateway-tls-secret kubernetes.io/tls 2 6m19s nifi-mtls-ca-secret Opaque 1 16s nifi-tls-secret kubernetes.io/tls 2 6m31s nificl-sa-cert kubernetes.io/tls 2 16s ``` </pre>

### Запускаем оператор
- `kubectl apply -f nifi-operator-deployment-v02.yaml`
### Запускаем NIFI
- `kubectl apply -f nifi.yaml -n nifi`
### Отслеживаем деплой
- `kubectl get pod -w -n nifi`


## В браузере https://nifi.domen.ru
- входим юзером которого создали в keycloak и поместили в группу nifi_admins
- в UI группе nifi_clients назначаем police view the user interface (иначе пользователем из этой группы в UI не войдёте и зациклитесь на входе)


