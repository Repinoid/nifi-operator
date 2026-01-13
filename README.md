В обоих manual_deploy в файле DOMAIN прописать ваш домен
запустить ./generate_manifests.sh - создадутся CRD

Keycloak - 
Создать реалм nifi-realm
Импортировать клиента из registry_manual_deploy/nifi-registry-keycloak-client.json

В файле nifi_manual_deploy/nifi-keycloak-client.json заменить все NIFI.DOMEN.RU на ваш домен
Импортировать клиента из этого файла

В обоих клиентах регенерировать Client Secret и прописать в соответствующие CRD в папках *_manual_deploy

Создать namespace nifi & registry

Далее - kubectl apply -f всего последовательно

kubectl port-forward -n registry service/registry 18443:18443

UI Registry через localhost:18443/nifi-registry/
пока так ... может и не надо по другому, т.к. типа безопаснее и траблы великие настраивать через ингресс и чтобы с самим нифи контачило ...

Пользователь regadmin

В своих кластерах всё разворачивается на ура, обпроверяялся ... а вот что на чужом ...

