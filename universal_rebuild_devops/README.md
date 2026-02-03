# DevOps workflow (universal_rebuild)

## 0) Требования
- Go >= 1.21
- Terraform >= 1.5
- Токен Nubes API (access_token)

## 1) Токен
Сохранить `access_token` в файл `terra.token` в корне проекта.
Либо задать переменную `TOKEN_FILE`.

## 2) Генерация YAML
```
scripts/gen_yaml.sh <service_id>
```
Результат: `resources_yaml/<service_name>.yaml` (имя берётся из API).

Пример:
```
scripts/gen_yaml.sh 28
```

## 3) Генерация Go + сборка провайдера + минимальный HCL
```
scripts/build_provider.sh <service_id>
```
Делает:
- `go run ./tools/gen/main.go`
- `go build -o ./terraform-provider-nubes .`
- добавляет минимальный ресурс в `test_dummy/main.tf`

## 4) Terraform (локальные тесты)
Файл: `test_dummy/main.tf`

Минимальный ресурс добавляется автоматически и содержит:
- `resource_name`
- обязательные параметры (с дефолтами или `REPLACE_ME`)

Если есть `REPLACE_ME` — заменить вручную.

## 5) Пример запуска
```
export TOKEN_FILE=./terra.token
scripts/gen_yaml.sh 90
scripts/build_provider.sh 90

TF_CLI_CONFIG_FILE=./test_dummy/dev_override.tfrc \
TF_VAR_api_token="$(cat $TOKEN_FILE)" \
terraform -chdir=./test_dummy plan
```

## 6) Важные правила
- Ядро и CRUD не менять без прямого разрешения.
- В `test_dummy/main.tf` держать минимум параметров.
- Для ссылочных параметров (refSvcId) использовать имена сервисов, не UUID, если разрешено.

## 7) Частые проблемы
- 401: токен истёк. Обновить файл токена.
- 404 при apply после ручного удаления: удалить ресурс из state (`terraform state rm ...`).
- 500/502/timeout: повторить операцию позже.
