terraform {
  required_providers {
    nubes = {
      # Сюда мы прописываем ВАШ приватный регистри
      source  = "terrareg.kube5s.ru/nubes/nubes"
      version = "1.0.0"
    }
  }
}

variable "api_token" {
  description = "Token for Nubes Cloud API"
  type        = string
  sensitive   = true
}

provider "nubes" {
  # Это адрес самого ОБЛАКА, которым мы управляем (Nubes API)
  api_endpoint = "https://deck-api.ngcloud.ru/api/v1/index.cfm"

  # Токен для авторизации
  api_token = var.api_token
}

