//
// Deployment Configuration
//

variable "owner" {
  description = "owner of the resources"
  type        = string
  default     = "storacha"
}

variable "team" {
  description = "name of team managing working on the project"
  type        = string
  default     = "Storacha Engineer"
}

variable "org" {
  description = "name of the organization managing the project"
  type        = string
  default     = "Storacha"
}

variable "allowed_account_ids" {
  description = "account IDs used for AWS"
  type        = list(string)
  default     = ["0"]
}

variable "region" {
  description = "AWS region to deploy resources"
  type        = string
  default     = "us-east-1"
}

variable "ebs_volume_size" {
  description = "Size of the EBS volume in GB"
  type        = number
  default     = 200
}

variable "instance_type" {
  description = "EC2 instance type"
  type        = string
  default     = "m6a.xlarge"
}

// the domain this deployment is reachable on is a concatenation of: environment.app.root_domain
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

//
// Installation Configuration
//

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

variable "pdp_lotus_endpoint" {
  description = "Lotus WebSocket endpoint for PDP"
  type        = string
}

variable "pdp_contract_address" {
  description = "PDP contract address"
  type        = string
}

variable "service_pem_content" {
  description = "Contents of the service.pem private key file"
  type        = string
  sensitive   = true
}

variable "wallet_hex_content" {
  description = "Contents of the wallet.hex private key file"
  type        = string
  sensitive   = true
}

variable "key_name" {
  description = "Name of the SSH key pair"
  type        = string
  default     = "warm-storage-staging"
}

variable "operator_email" {
  description = "Email address of the piri operator (for contact with the Storacha team)"
  type        = string
}

variable "registrar_url" {
  description = "URL of the registrar service for node registration"
  type        = string
  default     = "https://staging.registrar.storacha.network"
}

