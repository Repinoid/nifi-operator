resource "nubes_tubulus_instance" "example_instance" {
  # Имя экземпляра (Обязательно)
  display_name = "Bolvanka Full Test"

  # Описание (Опционально)
  description = "Testing all parameters transmission via Terraform"

  # Дополнительные параметры
  resource_realm   = "dummy" # Платформа (dummy)
  duration_ms      = 2000    # Время выполнения операции в мс
  fail_at_start    = false   # Упасть сразу при старте
  fail_in_progress = false   # Упасть в процессе выполнения
  where_fail       = 1       # Этап падения (1=prepare, 2=data_fill, 3=after_vault)

  # Расширенные данные
  body_message = "Hello from Terraform Client Package"
  map_example  = "{\"env\": \"production\", \"tier\": \"backend\"}"
  json_example = "{\"config_id\": 12345, \"active\": true}"
  # yaml_example    = "" 
}
