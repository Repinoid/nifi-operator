variable "api_endpoint" {
  type    = string
  default = "https://deck-api.ngcloud.ru/api/v1/index.cfm"
}

variable "api_token" {
  type      = string
  sensitive = true
}

variable "organization_id" {
  type    = string
  default = "dummy"
}

variable "organization_name" {
  type    = string
  default = "dummy"
}

variable "s3_user_uid" {
  type        = string
  description = "UID of the S3 User (required for S3 bucket)"
  default     = "dummy"
}

variable "s3_bucket_name" {
  type    = string
  default = "tf-test-bucket-lifecycle"
}

variable "pg_version" {
  type    = string
  default = "15"
}

variable "pg_cpu" {
  type    = number
  default = 2
}

variable "pg_ram" {
  type    = number
  default = 4
}

variable "pg_disk" {
  type    = number
  default = 10
}

variable "backup_schedule" {
    type = string
    default = "0 0 * * *"
}
