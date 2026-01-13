# Keycloak — установка и настройка для NiFi

Краткое руководство по развёртыванию Keycloak с помощью Helm-чарта и базовой настройке для интеграции с Apache NiFi.

---

## Содержание
- Требования
- Установка
- Доступ
- Realm и Client
- Mappers и Client scopes
- Группы и пользователи
- Примечания

---

## Требования
- Доступ к Kubernetes-кластеру с `kubectl` и правами для создания namespace и ресурсов.
- Helm (v3+).
- Настроенный Ingress и корректные TLS-сертификаты.


## Установка
1. Создать namespace:

```bash
kubectl create namespace keycloak
```

2. Подставьте ваш домен в файлах `keycloak-tls-secret.yaml` и `values.yaml`.

3. Создать PersistentVolumeClaim (если требуется):

```bash
kubectl apply -f PersistentVolumeClaim.yaml -n keycloak
```

4. Создать TLS-секрет:

```bash
kubectl apply -f keycloak-tls-secret.yaml -n keycloak
```

5. Установить Helm-чарт (замените `release-name` на своё имя):

```bash
helm install <release-name> . -n keycloak
```


## Доступ
Откройте в браузере Ingress-host (например `https://keycloak.example.com`).
По умолчанию: пользователь `admin`, пароль `admin` — рекомендуется сразу сменить пароль.


## Realm и Client
1. Войдите в Keycloak → **Manage realms** → **Create realm**
   - Realm name: `nifi-realm`
   - Сделайте `nifi-realm` текущим (Current realm)

2. Создать Client: **Clients** → **Create client**
   - Client type: **OpenID Connect**
   - Client ID: `nifi-keycloak-client`

3. На вкладке настроек клиента установите:
   - Client authentication: **ON**
   - Authorization: **OFF**
   - Authentication flow: отметьте **Standard flow**, **Direct access grants**, **Service accounts roles**
   - PKCE Method: оставить по умолчанию (не выбирать)
   - Require DPoP bound tokens: **OFF**

4. В разделе URL'ов клиента заполните (замените `<ваш-host>` на ваш домен):
   - Root URL: `https://<ваш-host>`  (например `https://nifi.example.com`)
   - Home URL: `https://<ваш-host>`
   - Valid Redirect URIs: `https://<ваш-host>:443/nifi-api/access/oidc/callback`
   - Valid Post Logout Redirect URIs: `https://<ваш-host>*`  (обратите внимание на `*`)
   - Web Origins: `https://<ваш-host>`

5. Сохраните изменения и перейдите в **Credentials** → `Client Authenticator: Client ID and Secret` — **скопируйте Client Secret** (он понадобится для CR NiFi).


## Mappers и Client scopes
1. В клиенте перейдите в **Client Scopes** → `nifi-keycloak-client-dedicated` → **Configure a new mapper**
   - Mapper: **Group Membership**
   - Name: `NIFI Groups Mapper`
   - Token Claim Name: `nifi_groups`
   - Full group path: **OFF**
   - Add to lightweight access token: **OFF**
   - Остальные опции: **ON**
   - Save

2. Добавьте ещё один mapper:
   - Add Mapper → By configuration → **User Realm Role**
   - Name: `Realm Roles Mapper`
   - Multivalued: **ON**
   - Token Claim Name: `nifi_groups`
   - Claim JSON Type: `string`

## Группы и пользователи
1. **Groups** → Create group
   - Name: `nifi_admins`
   - Description: администраторы NIFI

2. **Groups** → Create group
   - Name: `nifi_clients`
   - Description: пользователи NIFI

3. **Users** → Create user
   - Email verified: **ON**
   - Username: (администраторский логин)
   - Email: заполните (иначе Keycloak будет просить позже)
   - Join groups: добавьте в `nifi_admins` (обязательно для админов)
   - Вкладка **Credentials**: Set password → Temporary: **OFF** (убрать флажок Temporary)


---

> Готово 

