resource "nubes_tubulus_instance" "example_instance33" {
  # Имя экземпляра (Обязательно)
  display_name = "Bolvanka00"

  # Описание (Опционально)
  description = "Testing Terraform Plan output"

  # Дополнительные параметры
  # resource_realm   = "dummy"
  # duration_ms      = 5000 
  # fail_at_start    = true  
  # fail_in_progress = false 
  # where_fail       = 1     

  # AI Инструкция
  instruction = "создай машину которая работает 5 секунд и падает при старте"


}
