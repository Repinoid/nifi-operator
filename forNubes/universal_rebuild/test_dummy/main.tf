terraform {
  required_providers {
    nubes = {
      source = "terra.k8c.ru/nubes/nubes"
      version = "2.0.0"
    }
  }
}

variable "api_token" {
  type        = string
  sensitive   = true
  description = "Nubes API token"
}

provider "nubes" {
  api_token = var.api_token
}

resource "nubes_dummy" "test_dummy" {
  resource_name   = "tf-dummy-20260203-01"
  duration_ms     = 1700
  fail_at_start   = false
  fail_in_progress = false
  where_fail      = 1
  bodymessage     = "update-2"
  resource_realm  = "dummy"
  delete_mode     = "suspend"
}

resource "nubes_s3bucket" "test_bucket" {
  resource_name = "tf-bucket-20260203-01"
  s3_user_uid   = "s3-111805"
  bucket_name   = "tf-bucket-20260203-01-bucket"
  max_size      = "-1"
  read_all      = false
  list_all      = false
  cors_all      = false
  placement     = "HOT"
  delete_mode   = "delete"
}

resource "nubes_postgres" "test_postgres" {
  resource_name                 = "tf-postgres-20260203-01"
  s3_uid                        = "s3-111805"
  resource_instances            = 1
  resource_memory               = 512
  resource_c_p_u                = 500
  resource_disk                 = 1
  resource_realm                = "k8s-3.ext.nubes.ru"
  app_version                   = "17"
  json_parameters               = "{ \"log_connections\": \"off\", \"log_disconnections\": \"off\" }"
  enable_pg_pooler_master       = false
  enable_pg_pooler_slave        = false
  allow_no_s_s_l                = false
  auto_scale                    = false
  auto_scale_percentage         = 10
  auto_scale_tech_window        = 0
  auto_scale_quota_gb           = "1"
  delete_mode                   = "suspend"
}

resource "nubes_vc_vm_v3" "test_vm" {
  resource_name          = "tf-vm-20260203-01"
  vapp_uid               = "vapp-2"
  vm_name                = "tfvm2026020301"
  vm_cpu                 = 1
  vm_ram                 = 1
  vm_disk                = 11
  image_vm               = "Ubuntu_22-20G"
  user_login             = "ubuntu"
  user_public_key        = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGuwLgU4/+T5FXiFpZZA7Qh3hml1AcGX+Ya+6c+224BX"
  access_port_list       = jsonencode(["22"])
  need_add_zabbix_template = false
  delete_mode            = "suspend"
}

resource "nubes_lucee" "test_lucee" {
  resource_name      = "tf-lucee-20260203-01"
  domain             = "lucee-20260203-01"
  git_path           = "https://github.com/xahys/testlucee"
  resource_c_p_u     = 300
  resource_memory    = 512
  resource_realm     = "k8s-3.ext.nubes.ru"
  resource_instances = 1
  app_version        = "5.4"
  delete_mode        = "suspend"
}

resource "nubes_kafka" "test_kafka" {
  resource_name               = "tf-kafka-20260203-01"
  resource_instances          = 1
  resource_memory             = 1000
  resource_c_p_u              = 1000
  resource_disk               = 1
  need_external_address_master = false
  resource_realm              = "k8s-3.ext.nubes.ru"
  delete_mode                 = "suspend"
}

resource "nubes_flask" "test_flask" {
  resource_name      = "tf-flask-20260203-01"
  domain             = "flask-20260203-01"
  resource_realm     = "k8s-3.ext.nubes.ru"
  resource_c_p_u     = 300
  resource_memory    = 512
  resource_instances = 1
  git_path           = "https://github.com/Foxyhhd/Baldurs-Gate-test.git"
  delete_mode        = "suspend"
}

resource "nubes_redis" "test_redis" {
  resource_name                = "tf-redis-20260203-01"
  resource_c_p_u               = 500
  resource_memory              = 512
  resource_disk                = 10
  resource_instances           = 1
  resource_realm               = "k8s-3.ext.nubes.ru"
  need_external_address_master = false
  need_external_address_slave  = false
  delete_mode                  = "suspend"
}

resource "nubes_mongodb" "test_mongodb" {
  resource_name                = "tf-mongodb-20260203-01"
  resource_instances           = 1
  resource_memory              = 1000
  resource_c_p_u               = 500
  resource_disk                = 10
  resource_realm               = "k8s-3.ext.nubes.ru"
  need_external_address_master = "false"
  delete_mode                  = "suspend"
}

resource "nubes_rabbitmq" "test_rabbitmq" {
  resource_name                = "tf-rabbitmq-20260203-01"
  resource_c_p_u               = 500
  resource_memory              = 512
  resource_disk                = 10
  resource_realm               = "k8s-3.ext.nubes.ru"
  resource_instances           = 1
  need_external_address_master = false
  delete_mode                  = "suspend"
}

resource "nubes_nodejs" "test_nodejs" {
  resource_name      = "tf-nodejs-20260203-01"
  domain             = "nodejs-20260203-01"
  git_path           = "https://github.com/xahys/testnode.git"
  resource_c_p_u     = 500
  resource_memory    = 1024
  resource_instances = 1
  resource_realm     = "k8s-3.ext.nubes.ru"
  app_version        = "23"
  delete_mode        = "suspend"
}

resource "nubes_pgadmin" "test_pgadmin" {
  resource_name   = "tf-pgadmin-20260203-01"
  domain          = "pgadmin-20260203-01"
  resource_c_p_u  = 200
  resource_memory = 256
  resource_disk   = 1
  resource_realm  = "k8s-3.ext.nubes.ru"
  login           = "admin@example.com"
  password        = "Admin123!"
  delete_mode     = "suspend"
}

resource "nubes_nodered" "test_nodered" {
  resource_name      = "tf-nodered-20260203-01"
  resource_c_p_u     = 500
  resource_memory    = 512
  resource_disk      = 1
  resource_realm     = "k8s-3.ext.nubes.ru"
  domain             = "nodered-20260203-01"
  resource_instances = 1
  delete_mode        = "suspend"
}

resource "nubes_gitea" "test_gitea" {
  resource_name      = "tf-gitea-20260203-01"
  resource_c_p_u     = 500
  resource_memory    = 1024
  resource_disk      = 10
  resource_instances = 1
  domain             = "gitea-20260203-01"
  psql_uid           = nubes_postgres.test_postgres.id
  delete_mode        = "suspend"
}

resource "nubes_mariadb" "test_mariadb" {
  resource_name                = "tf-mariadb-20260203-01"
  resource_realm               = "k8s-3.ext.nubes.ru"
  resource_c_p_u               = 500
  resource_memory              = 1024
  resource_disk                = 1
  resource_instances           = 1
  need_external_address_master = false
  app_version                  = "9.4.0"
  auto_scale                   = false
  s3_uid                       = "s3-111805"
  delete_mode                  = "suspend"
}





