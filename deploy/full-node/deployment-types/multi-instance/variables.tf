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
  }))
}