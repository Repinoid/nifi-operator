terraform {
  required_providers {
    nubes = {
      source = "terraform.local/local/nubes"
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

variable "organization_id" {
  type = string
}

variable "s3_uid" {
  type = string
}

resource "nubes_postgres" "test_pg" {
  organization_id   = var.organization_id
  organization_name = "test-org"
  resource_realm    = "k8s-3.ext.nubes.ru"
  s3_uid           = var.s3_uid
  cpu              = 777
  memory           = 2048
  disk             = 11
  instances        = 1
  version          = "17"
  deletion_protection = false # Для тестов отключаем
}

output "pg_id" {
  value = nubes_postgres.test_pg.id
}
