# Deployment Configuration
variable "owner" {
  description = "Owner of the resources"
  type        = string
  default     = "storacha"
}

variable "team" {
  description = "Name of team managing the project"
  type        = string
  default     = "Storacha Engineering"
}

variable "org" {
  description = "Name of the organization managing the project"
  type        = string
  default     = "Storacha"
}

variable "allowed_account_ids" {
  description = "Account IDs used for AWS"
  type        = list(string)
  default     = ["505595374361"]
}

variable "region" {
  description = "AWS region to deploy resources"
  type        = string
  default     = "us-west-2"
}

variable "environment" {
  description = "Environment name"
  type        = string
  default     = "staging"
}

variable "root_domain" {
  description = "Root domain for the deployment"
  type        = string
  default     = "pdp.storacha.network"
}

variable "key_name" {
  description = "Name of the SSH key pair"
  type        = string
  default     = "warm-storage-staging"
}

# Default Instance Configuration
variable "default_instance_type" {
  description = "Default EC2 instance type"
  type        = string
  default     = "m6a.xlarge"
}

variable "default_ebs_volume_size" {
  description = "Default size of the EBS volume in GB"
  type        = number
  default     = 100
}

variable "default_install_method" {
  description = "Default installation method: 'version' for release or 'branch' for building from source"
  type        = string
  default     = "version"
}

variable "default_install_source" {
  description = "Default version tag or branch name"
  type        = string
  default     = "v0.0.12"
}

# Shared Configuration
variable "network" {
  description = "Network the node will operate on. Valid values are 'forge-prod', 'warm-staging', 'prod' and 'staging'."
  type        = string
}

variable "pdp_lotus_endpoint" {
  description = "Lotus WebSocket endpoint for PDP"
  type        = string
}

variable "use_secrets_manager" {
  description = "Use AWS Secrets Manager for sensitive data instead of variables"
  type        = bool
  default     = true
}

# =============================================================================
# Storage Backend Configuration (defaults)
# =============================================================================

variable "default_storage_backend" {
  description = "Default blob storage backend: 'filesystem' (default) or 'minio'"
  type        = string
  default     = "filesystem"
  validation {
    condition     = contains(["filesystem", "minio"], var.default_storage_backend)
    error_message = "default_storage_backend must be 'filesystem' or 'minio'"
  }
}

variable "default_database_backend" {
  description = "Default database backend: 'sqlite' (default) or 'postgres'"
  type        = string
  default     = "sqlite"
  validation {
    condition     = contains(["sqlite", "postgres"], var.default_database_backend)
    error_message = "default_database_backend must be 'sqlite' or 'postgres'"
  }
}

# MinIO configuration (when storage_backend = "minio")
variable "minio_root_user" {
  description = "MinIO root user"
  type        = string
  default     = "minioadmin"
}

variable "minio_root_password" {
  description = "MinIO root password (required when storage_backend = 'minio')"
  type        = string
  sensitive   = true
  default     = ""
}

variable "minio_bucket_prefix" {
  description = "Prefix for MinIO buckets"
  type        = string
  default     = "piri-"
}

# PostgreSQL configuration (when database_backend = "postgres")
variable "postgres_user" {
  description = "PostgreSQL user"
  type        = string
  default     = "piri"
}

variable "postgres_password" {
  description = "PostgreSQL password (required when database_backend = 'postgres')"
  type        = string
  sensitive   = true
  default     = ""
}

variable "postgres_database" {
  description = "PostgreSQL database name"
  type        = string
  default     = "piri"
}

# Instance Definitions
variable "instances" {
  description = "Map of instances to create"
  type = map(object({
    subdomain           = string
    operator_email      = string
    service_pem_content = optional(string, "")
    wallet_hex_content  = optional(string, "")
    instance_type       = optional(string)
    ebs_volume_size     = optional(number)
    install_method      = optional(string)
    install_source      = optional(string)
    # Backend overrides (optional)
    storage_backend     = optional(string)
    database_backend    = optional(string)
    minio_root_password = optional(string)
    postgres_password   = optional(string)
  }))
}