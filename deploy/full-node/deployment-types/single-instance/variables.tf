# Deployment Configuration
variable "owner" {
  description = "Owner of the resources"
  type        = string
  default     = "storacha"
}

variable "team" {
  description = "Name of team managing the project"
  type        = string
  default     = "Storacha Engineer"
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

variable "app" {
  description = "The name of the application"
  type        = string
  default     = "piri"
}

variable "root_domain" {
  description = "Root domain for the deployment"
  type        = string
  default     = "pdp.storacha.network"
}

# Instance Configuration
variable "instance_type" {
  description = "EC2 instance type"
  type        = string
  default     = "m6a.xlarge"
}

variable "ebs_volume_size" {
  description = "Size of the EBS volume in GB"
  type        = number
  default     = 100
}

variable "key_name" {
  description = "Name of the SSH key pair"
  type        = string
  default     = "warm-storage-staging"
}

# Installation Configuration
variable "install_method" {
  description = "Installation method: 'version' for release or 'branch' for building from source"
  type        = string
  validation {
    condition     = contains(["version", "branch"], var.install_method)
    error_message = "install_method must be either 'version' or 'branch'"
  }
}

variable "install_source" {
  description = "Version tag (e.g., 'v0.0.12') or branch name (e.g., 'main', 'dev')"
  type        = string
}

variable "network" {
  description = "Network the node will operate on. Valid values are 'forge-prod', 'warm-staging', 'prod' and 'staging'."
  type        = string
}

variable "registrar_url" {
  description = "URL of the registrar service for node registration"
  type        = string
  default     = "https://staging.registrar.warm.storacha.network"
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

variable "service_pem_content" {
  description = "Contents of the service.pem private key file (ignored if use_secrets_manager is true)"
  type        = string
  sensitive   = true
  default     = ""
}

variable "wallet_hex_content" {
  description = "Contents of the wallet.hex private key file (ignored if use_secrets_manager is true)"
  type        = string
  sensitive   = true
  default     = ""
}

variable "operator_email" {
  description = "Email address of the piri operator"
  type        = string
}