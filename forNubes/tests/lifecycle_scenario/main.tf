terraform {
  required_providers {
    nubes = {
      source = "terraform.local/nubes/nubes"
    }
  }
}

provider "nubes" {
  api_endpoint = var.api_endpoint
  api_token    = var.api_token
}

/*
resource "nubes_s3_bucket" "test_bucket" {
  name                = var.s3_bucket_name
  s3_root_service_uid = "e5375174-36ec-4512-bba9-b56f9eeba0bd" # Dummy

  max_size_bytes      = -1
  storage_class       = "HOT"

  read_all            = false
  list_all            = false
  cors_all            = false
}

resource "nubes_postgres" "test_pg" {
  organization_id   = var.organization_id
  organization_name = var.organization_name
  resource_realm    = "kvm-v1cl1-ssd" # Hardcoded or generic
  s3_uid            = nubes_s3_bucket.test_bucket.id
  
  cpu               = var.pg_cpu
  memory            = var.pg_ram
  disk              = var.pg_disk
  instances         = 1
  version           = var.pg_version
  
  backup_schedule   = var.backup_schedule
  backup_retention  = 14
  parameters        = "{}"
  
  enable_pgpooler_master = false
  enable_pgpooler_slave  = false
  allow_no_ssl           = false 
  auto_scale             = false
  auto_scale_percentage  = 20
  auto_scale_tech_window = 1
  auto_scale_quota_gb    = 20

  need_external_address_master = false
  need_external_address_slave  = false
  ip_space_name_master        = ""
  ip_space_name_slave         = ""

  depends_on = [nubes_s3_bucket.test_bucket]
}

output "pg_state" {
  value = nubes_postgres.test_pg.id
}

output "monitoring_url" {
  value = nubes_postgres.test_pg.monitoring_url
}

output "connection_string" {
  value = nubes_postgres.test_pg.internal_connect_jdbc
}
*/
