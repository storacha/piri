variable "name" {
  description = "Name identifier for the instance (e.g., 'node1', 'primary')"
  type        = string
}

variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "staging"
}

variable "instance_type" {
  description = "EC2 instance type"
  type        = string
  default     = "m6a.xlarge"
}

variable "ebs_volume_size" {
  description = "Size of the EBS volume in GB"
  type        = number
  default     = 200
}

variable "availability_zone" {
  description = "The availability zone for the EBS volume (should match subnet's AZ)"
  type        = string
}

variable "protect_volume" {
  description = "Enable volume destruction protection (prevent_destroy lifecycle rule)"
  type        = bool
  default     = false
}

variable "key_name" {
  description = "Name of the SSH key pair"
  type        = string
}

variable "subnet_id" {
  description = "Subnet ID to launch the instance in"
  type        = string
}

variable "security_group_id" {
  description = "Security group ID to attach to the instance"
  type        = string
}

variable "iam_instance_profile_name" {
  description = "IAM instance profile name"
  type        = string
}

variable "internet_gateway_id" {
  description = "Internet gateway ID for dependencies"
  type        = string
}

variable "domain_name" {
  description = "Full domain name for the instance (e.g., 'node1.piri.storacha.network')"
  type        = string
}

variable "route53_zone_id" {
  description = "Route53 hosted zone ID"
  type        = string
}

variable "install_method" {
  description = "Installation method: 'version' for release or 'branch' for building from source"
  type        = string
  default     = "version"
  validation {
    condition     = contains(["version", "branch"], coalesce(var.install_method, "version"))
    error_message = "install_method must be either 'version' or 'branch'"
  }
}

variable "install_source" {
  description = "Version tag (e.g., 'v0.0.12') or branch name (e.g., 'main', 'dev')"
  type        = string
  default     = "v0.0.12"
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

variable "pdp_contract_address" {
  description = "PDP contract address"
  type        = string
}

variable "use_secrets_manager" {
  description = "Use AWS Secrets Manager for sensitive data instead of variables"
  type        = bool
  default     = false
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

variable "use_letsencrypt_staging" {
  description = "Use Let's Encrypt staging environment (for testing, no rate limits)"
  type        = bool
  default     = true
}

variable "tags" {
  description = "Additional tags to apply to all resources"
  type        = map(string)
  default     = {}
}