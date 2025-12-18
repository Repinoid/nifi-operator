# Настройка NIFI
## Надо иметь три (суб)домена. 
- `NIFI.DOMEN.RU` - собственно для NIFI
- `API.DOMEN.RU` - для NIFI API
- `KEYCLOAK.DOMEN.RU` - keycloak, опционально, если нет иного функционирующего keycloak

### Во всех файлах заменить шаблоны NIFI.DOMEN.RU, API.DOMEN.RU, KEYCLOAK.DOMEN.RU на свои реальные

### Деплой/Настройка Keycloak
- в файле KeyCloak.md

### Создаём namespace
- ```kubectl create namespace nifi```

### Создаём сертификаты

- ` openssl s_client \  
  -connect KEYCLOAK.DOMEN.RU:443 \  
  -servername KEYCLOAK.DOMEN.RU \  
  -showcerts \  
  </dev/null 2>/dev/null \  
  | sed -n '/-----BEGIN CERTIFICATE-----/,/-----END CERTIFICATE-----/p' \  
  | kubectl create secret generic keycloak-nifi-secret \  
      --from-file=tls.crt=/dev/stdin \  
      -n nifi `  

- `kubectl apply -f nifi-tls-secret.yaml -n nifi`

- `kubectl apply -f certificate-api.yaml -n nifi`

- `kubectl apply -f nifi-cert-generator-job.yaml -n nifi && \  
kubectl wait --for=condition=complete job/nifi-cert-generator -n nifi --timeout=300s && \  
kubectl delete job nifi-cert-generator -n nifi  `

- `kubectl get secret -n nifi`  

Должно быть такое -
- keycloak-nifi-secret            Opaque              1      3m31s  
- nifi-admin-gateway-tls-secret   kubernetes.io/tls   2      6m19s  
- nifi-mtls-ca-secret             Opaque              1      16s  
- nifi-tls-secret                 kubernetes.io/tls   2      6m31s  
- nificl-sa-cert                  kubernetes.io/tls   2      16s

### Запускаем оператор
- `kubectl apply -f nifi-operator-deployment-v02.yaml`
### Запускаем NIFI
- `kubectl apply -f nifi.yaml -n nifi`
### Отслеживаем деплой
- `kubectl get pod -w -n nifi`


## В браузере https://NIFI.DOMEN.RU
- входим юзером которого создали в keycloak и поместили в группу nifi_admins
- в UI группе nifi_clients назначаем police view the user interface (иначе пользователем из этой группы в UI не войдёте и зациклитесь на входе)


