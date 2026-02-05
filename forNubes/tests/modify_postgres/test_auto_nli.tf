resource "nubes_tubulus_instance" "auto_test" {
  display_name = "Test1-Fast"
  description  = "NLI Auto Test"
  instruction  = "сделай быстро"
}

output "duration" {
  value = nubes_tubulus_instance.auto_test.duration_ms
}

output "body_msg" {
  value = nubes_tubulus_instance.auto_test.body_message
}

output "where_fail" {
  value = nubes_tubulus_instance.auto_test.where_fail
}

output "status" {
  value = nubes_tubulus_instance.auto_test.status
}
