terraform {
  required_providers {
    nubes = {
      source = "terra.k8c.ru/nubes/nubes"
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





