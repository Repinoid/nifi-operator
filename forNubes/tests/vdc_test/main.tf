terraform {
  required_providers {
    nubes = {
      source = "terraform.local/nubes/nubes"
    }
  }
}

variable "api_token" {
  type = string
}

variable "organization_id" {
  type = string
}

variable "provider_vdc" {
  type = string
}

variable "network_pool" {
  type = string
}

variable "storage_profiles" {
  type = string
}

variable "cpu_quota" {
  type        = number
  default     = 10
}

variable "ram_quota" {
  type        = number
  default     = 20
}

provider "nubes" {
  api_token = var.api_token
}

resource "nubes_vdc" "test" {
  display_name       = "terraform-vdc-modify-test"
  description        = "Testing VDC modify operation"
  organization_uid   = var.organization_id
  provider_vdc       = var.provider_vdc
  network_pool       = var.network_pool
  storage_profiles   = var.storage_profiles
  cpu_allocation_pct = 20
  ram_allocation_pct = 20
  cpu_quota          = var.cpu_quota
  ram_quota          = var.ram_quota
  
  deletion_protection = true
}

output "vdc_id" {
  value = nubes_vdc.test.id
}
