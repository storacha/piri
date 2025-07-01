variable "app" {
  description = "name of the application"
  type        = string
  default     = "pdp"
}

variable "owner" {
  description = "owner of the resources"
  type        = string
  default     = "storacha"
}

variable "team" {
  description = "name of team managing working on the project"
  type        = string
  default     = "Storacha Engineering"
}

variable "org" {
  description = "name of the organization managing the project"
  type        = string
  default     = "Storacha"
}

variable "region" {
  description = "aws region for all services"
  type        = string
  default     = "us-west-2"
}

variable "domain" {
  description = "domain name to use for the deployment (will be prefixed with app name)"
  type = string
  default = "storacha.network"
}

variable "allowed_account_ids" {
  description = "account IDs used for AWS"
  type        = list(string)
  default     = ["505595374361"]
}

variable "instance_type" {
  type    = string
  default = "t2.2xlarge"
}

variable "volume_type" {
  type    = string
  default = "gp3"
}

variable "volume_size" {
  type    = number
  default = 256
}

variable "go_version" {
  type    = string
  default = "1.24.4"
}

variable "yugabyte_version" {
  type    = string
  default = "2.21.0.1"
}

variable "lotus_version" {
  type    = string
  default = "v1.31.1"
}

variable "filecoin_network" {
  type    = string
  default = "calibnet"
}

variable "lotus_snapshot_url" {
  type = string
  default = "https://forest-archive.chainsafe.dev/latest/calibnet/"
}

variable "curio_version" {
  type    = string
  default = "feat/pdp"
}

variable "lotus_wallet_bls_file" {
  description = "Path on the local machine (running Terraform) to the lotus BLS wallet file"
  type        = string
}

variable "lotus_wallet_delegated_file" {
  description = "Path on the local machine (running Terraform) to the lotus delegated wallet file"
  type        = string
}

variable "curio_service_pem_key_file" {
  description = "Path on the local machine (running Terraform) to the pem key used to authorize api calls to curio"
  type        = string
}
