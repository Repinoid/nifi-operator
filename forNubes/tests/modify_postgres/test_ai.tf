resource "nubes_tubulus_instance" "ai_test" {
  display_name = "AI Bolvanka v4 unique"
  description  = "Testing Gemini AI instruction"
  
  instruction = "Создай тубулус на 12 секунд, без ошибок, в вольт напиши: 'секретный ключ 123'"
}

output "ai_duration" {
  value = nubes_tubulus_instance.ai_test.duration_ms
}

output "ai_fail_in_progress" {
  value = nubes_tubulus_instance.ai_test.fail_in_progress
}

output "ai_where_fail" {
  value = nubes_tubulus_instance.ai_test.where_fail
}
