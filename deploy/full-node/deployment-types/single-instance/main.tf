terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region              = var.region
  allowed_account_ids = var.allowed_account_ids
}

module "base_infrastructure" {
  source = "../../modules/base-infrastructure"

  environment = var.environment
  tags = {
    Owner = var.owner
    Team  = var.team
    Org   = var.org
  }
}

data "aws_route53_zone" "primary" {
  name = var.root_domain
}

module "piri_instance" {
  source = "../../modules/piri-instance"

  name                      = "primary"
  environment               = var.environment
  instance_type             = var.instance_type
  ebs_volume_size           = var.ebs_volume_size
  key_name                  = var.key_name
  subnet_id                 = module.base_infrastructure.public_subnet_id
  security_group_id         = module.base_infrastructure.security_group_id
  iam_instance_profile_name = module.base_infrastructure.iam_instance_profile_name
  internet_gateway_id       = module.base_infrastructure.internet_gateway_id
  availability_zone         = module.base_infrastructure.availability_zone
  protect_volume            = var.environment == "production" || var.environment == "prod" || var.environment == "forge-prod"
  domain_name               = "${var.environment}.${var.app}.${var.root_domain}"
  route53_zone_id           = data.aws_route53_zone.primary.zone_id
  
  install_method          = var.install_method
  install_source          = var.install_source
  network                 = var.network
  pdp_lotus_endpoint      = var.pdp_lotus_endpoint
  use_secrets_manager     = var.use_secrets_manager
  service_pem_content     = var.service_pem_content
  wallet_hex_content      = var.wallet_hex_content
  operator_email          = var.operator_email
  use_letsencrypt_staging = var.environment != "production" && var.environment != "prod" && var.environment != "forge-prod"

  tags = {
    Owner = var.owner
    Team  = var.team
    Org   = var.org
  }
}