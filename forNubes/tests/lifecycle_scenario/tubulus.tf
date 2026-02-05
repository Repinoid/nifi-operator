resource "nubes_tubulus_instance" "bolvanka" {
  display_name     = "Bolvanka_Lifecycle_Test_014"
  description      = "Stress Test 5: Complex"
  body_message     = "Complex Payload: JSON { key: 'value' }"
  duration_ms      = 2000
  yaml_example     = "some: yaml"
}

output "bolvanka_id" {
  value = nubes_tubulus_instance.bolvanka.id
}
