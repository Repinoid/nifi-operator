terraform {
  required_providers {
    nubes = {
      source = "terraform.local/nubes/nubes"
    }
  }
}

provider "nubes" {
  api_token = var.api_token
}

variable "api_token" {
  type      = string
  sensitive = true
}

variable "bucket_name" {
  type = string
}

variable "max_size" {
  type = number
}

resource "nubes_s3_bucket" "test" {
  name                = var.bucket_name
  s3_root_service_uid = "e5375174-36ec-4512-bba9-b56f9eeba0bd" # Dummy or real key

  max_size_bytes      = var.max_size
  storage_class       = "HOT"

  read_all            = true
  list_all            = true
  cors_all            = true
}

output "bucket_id" {
  value = nubes_s3_bucket.test.id
}