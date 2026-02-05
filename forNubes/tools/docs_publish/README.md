# Публикация документации (Nubes Terraform Provider)

## Назначение
Эта инструкция описывает полный цикл: генерация страниц ресурсов, сборка сайта и загрузка на https://terra.k8c.ru.

## Требования
- Docker
- `mc` (MinIO Client)
- Доступ к S3 (переменные `S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`)

## 1) Генерация страниц ресурсов
Источники параметров: `universal_rebuild/resources_yaml/*.yaml`.

Генератор страниц (создаёт/обновляет файлы в `docs/30_registry/resources/` и индекс):

```bash
/home/naeel/terra/.venv/bin/python - <<'PY'
import glob
import os
import re
import yaml

resources_yaml_dir = "/home/naeel/terra/universal_rebuild/resources_yaml"
resources_gen_dir = "/home/naeel/terra/universal_rebuild/internal/resources_gen"
resources_docs_dir = "/home/naeel/terra/docs/30_registry/resources"
all_resources_path = os.path.join(resources_docs_dir, "all_resources.md")
index_path = os.path.join(resources_docs_dir, "index.md")

os.makedirs(resources_docs_dir, exist_ok=True)

def parse_computed_fields(go_path: str):
    try:
        with open(go_path, "r", encoding="utf-8") as f:
            lines = f.readlines()
    except FileNotFoundError:
        return []

    computed = []
    i = 0
    while i < len(lines):
        line = lines[i]
        m = re.search(r'"([a-zA-Z0-9_]+)"\s*:\s*schema\.[A-Za-z]+Attribute\{', line)
        if not m:
            i += 1
            continue
        name = m.group(1)
        block_lines = [line]
        i += 1
        while i < len(lines) and "}," not in lines[i]:
            block_lines.append(lines[i])
            i += 1
        if i < len(lines):
            block_lines.append(lines[i])
        block_text = "".join(block_lines)
        if "Computed: true" in block_text:
            computed.append(name)
        i += 1
    return sorted(set(computed))

computed_by_name = {}
for go_path in glob.glob(os.path.join(resources_gen_dir, "*_resource.go")):
    name = None
    with open(go_path, "r", encoding="utf-8") as f:
        for line in f:
            if line.startswith("// Service:"):
                name = line.split(":", 1)[1].strip()
                break
    if not name:
        continue
    computed_by_name[name] = parse_computed_fields(go_path)

files = sorted(glob.glob(os.path.join(resources_yaml_dir, "service_*.yaml")))
resource_names = []

all_lines = []
all_lines.append("# Ресурсы провайдера (полный список)\n\n")
all_lines.append("Автогенерируемая сводка по ресурсам из `resources_yaml` и схем.\n\n")

for path in files:
    with open(path, "r", encoding="utf-8") as f:
        data = yaml.safe_load(f)
    if not data:
        continue
    name = data.get("name", "unknown")
    service_id = data.get("service_id", "unknown")
    resource_names.append(name)

    create = data.get("create", {}).get("params", []) or []
    modify = data.get("modify", {}).get("params", []) or []
    computed = computed_by_name.get(name, [])

    doc_path = os.path.join(resources_docs_dir, f"{name}.md")
    lines = []
    lines.append(f"# Resource nubes_{name}\n\n")
    lines.append(f"Service ID: `{service_id}`\n\n")

    lines.append("## Output поля\n\n")
    if computed:
        for c in computed:
            lines.append(f"- `{c}`\n")
    else:
        lines.append("Нет.\n")
    lines.append("\n")

    lines.append("## Create параметры\n\n")
    if create:
        lines.append("| Code | Type | Required | Default | ID |\n")
        lines.append("|---|---|---|---|---|\n")
        for p in create:
            code = p.get("code", "")
            ptype = p.get("type", "")
            required = str(p.get("required", False)).lower()
            default = p.get("default", "")
            pid = p.get("id", "")
            lines.append(f"| `{code}` | `{ptype}` | `{required}` | `{default}` | `{pid}` |\n")
    else:
        lines.append("Нет.\n")

    lines.append("\n## Modify параметры\n\n")
    if modify:
        lines.append("| Code | Type | Required | Default | ID |\n")
        lines.append("|---|---|---|---|---|\n")
        for p in modify:
            code = p.get("code", "")
            ptype = p.get("type", "")
            required = str(p.get("required", False)).lower()
            default = p.get("default", "")
            pid = p.get("id", "")
            lines.append(f"| `{code}` | `{ptype}` | `{required}` | `{default}` | `{pid}` |\n")
    else:
        lines.append("Нет.\n")

    with open(doc_path, "w", encoding="utf-8") as f:
        f.write("".join(lines))

    all_lines.append(f"## nubes_{name}\n")
    all_lines.append(f"- Service ID: {service_id}\n")
    all_lines.append("### Output поля\n")
    if computed:
        all_lines.append(", ".join(f"`{c}`" for c in computed) + "\n")
    else:
        all_lines.append("Нет.\n")

    all_lines.append("\n### Create параметры\n")
    if create:
        all_lines.append("| Code | Type | Required | Default | ID |\n")
        all_lines.append("|---|---|---|---|---|\n")
        for p in create:
            code = p.get("code", "")
            ptype = p.get("type", "")
            required = str(p.get("required", False)).lower()
            default = p.get("default", "")
            pid = p.get("id", "")
            all_lines.append(f"| `{code}` | `{ptype}` | `{required}` | `{default}` | `{pid}` |\n")
    else:
        all_lines.append("Нет.\n")

    all_lines.append("\n### Modify параметры\n")
    if modify:
        all_lines.append("| Code | Type | Required | Default | ID |\n")
        all_lines.append("|---|---|---|---|---|\n")
        for p in modify:
            code = p.get("code", "")
            ptype = p.get("type", "")
            required = str(p.get("required", False)).lower()
            default = p.get("default", "")
            pid = p.get("id", "")
            all_lines.append(f"| `{code}` | `{ptype}` | `{required}` | `{default}` | `{pid}` |\n")
    else:
        all_lines.append("Нет.\n")

    all_lines.append("\n")

resource_names_sorted = sorted(set(resource_names))
index_lines = ["# Resources\n\n", "Список всех ресурсов провайдера.\n\n"]
for name in resource_names_sorted:
    index_lines.append(f"- [{name}](./{name}.md)\n")

with open(index_path, "w", encoding="utf-8") as f:
    f.write("".join(index_lines))

with open(all_resources_path, "w", encoding="utf-8") as f:
    f.write("".join(all_lines))

print(f"Generated {len(resource_names_sorted)} resource pages.")
print(f"Wrote {index_path}")
print(f"Wrote {all_resources_path}")
PY
```

## 2) Навигация mkdocs
Проверьте, что `mkdocs.yml` содержит:
- `Resources` (index + all_resources)
- `Guides` (getting-started, terraform-basics)

## 3) Сборка сайта
```bash
docker run --rm -v ${PWD}:/docs squidfunk/mkdocs-material build
```

## 4) Публикация
```bash
set -a && . /home/naeel/terra/.env.s3 && set +a
/home/naeel/terra/scripts/publish-docs.sh site terra.k8c.ru nubes nubes 2.0.0
```

## Результат
Документация должна открываться по адресу:
`https://terra.k8c.ru/docs/nubes/nubes/2.0.0/`
