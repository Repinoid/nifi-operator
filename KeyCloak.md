# Helm Chart Keycloak

### Создать namespace
- kubectl create namespace keycloak

### Внести изменения - заменить домен keycloak на свой в файлах
- keycloak-tls-secret.yaml
- values.yaml

### создать volume
- `kubectl apply -f PersistentVolumeClaim.yaml -n keycloak`

### создать секрет
- `kubectl apply -f keycloak-tls-secret.yaml  -n keycloak`

### Запустить Helm Chart, kkk - заменить имя на своё
- `helm install kkk . -n keycloak`

### В браузере - ваш keycloak, т.е. ingress.host (aka https://keycloak.domen.ru)
- username admin, password admin
- в дальнейшем заведёте новое имя для администратора 

## Настройки
### Manage realms - Create realm - Realm name - "nifi-realm" 
- nifi-realm должен быть Current realm
### Clients - Create client - 
- Client type - **OpenID connect**
- Client ID - **nifi-keycloak-client**
<br> **Next**
- Client authentication - **ON**
- Authorization - **OFF**
- Authentication flow - пометить **Standard flow, Direct access grants, Service accounts roles**
- PKCE Method - ничего не выбирать, оставить Choose ...
- Require DPoP bound tokens **OFF**
<br> **Next**
- Root URL: https://<ваш хост NIFI> ``(aka https://nifi.domen.ru)``
- Home URL: https://<ваш хост NIFI>
- Valid redirect URIs: https://<ваш хост NIFI>:443/nifi-api/access/oidc/callback
- Valid post logout redirect URIs: https://<ваш хост NIFI>* (именно звёздочка на конце)
- Web origins: https://<ваш хост NIFI>
<br> **Save**
- Перейти во вкладку Credentials (Clients-nifi-keycloak-client-Credentials)
- Client Authenticator: Client ID and Secret
- `Client Secret` - **скопировать и запомнить (для CR NIFI)**
- Перейти во вкладку Client scopes
- Войти в nifi-keycloak-client-dedicated 
- Configure a new mapper
- Choose any of the mappings from this table - Group Membership
- Name: **NIFI Groups Mapper**
- Token Claim Name: **nifi_groups**
- Full group path **OFF**
- Add to lightweight access token: **OFF**, остальные - **ON**
<br> **Save**, на страницу назад
- Add Mapper
- By configuration
- User Realm Role
- Name: **Realm Roles Mapper**
- Multivalued: **ON** 
- Token Claim Name: **nifi_groups**
- Claim JSON Type: **string**

### Из левого основного меню - Groups
- Create a group
- Name: **nifi_admins**
- Description: администраторы NIFI
- Create group
- Name: **nifi_clients**
- Description: юзерА

### Из левого основного меню - Users
- Create user
- Email verified: **ON**
- Username - админовское
- E-Mail имя - заполнить, иначе потом будет спрашивать
- Join groups: **nifi_admins** ***(ОБЯЗАТЕЛЬНО)***
- Вкладка Credentials
- Set password
- Temporary - УБРАТЬ в **off**

### готово !

