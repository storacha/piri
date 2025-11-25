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

module "piri_instances" {
  source = "../../modules/piri-instance"
  
  for_each = var.instances

  name                      = each.key
  environment               = var.environment
  instance_type             = coalesce(lookup(each.value, "instance_type", null), var.default_instance_type)
  ebs_volume_size           = coalesce(lookup(each.value, "ebs_volume_size", null), var.default_ebs_volume_size)
  key_name                  = var.key_name
  subnet_id                 = module.base_infrastructure.public_subnet_id
  security_group_id         = module.base_infrastructure.security_group_id
  iam_instance_profile_name = module.base_infrastructure.iam_instance_profile_name
  internet_gateway_id       = module.base_infrastructure.internet_gateway_id
  availability_zone         = module.base_infrastructure.availability_zone
  protect_volume            = var.environment == "production" || var.environment == "prod"
  domain_name               = "${var.environment}.${each.value.subdomain}.${var.root_domain}"
  route53_zone_id           = data.aws_route53_zone.primary.zone_id
  
  install_method          = coalesce(lookup(each.value, "install_method", null), var.default_install_method)
  install_source          = coalesce(lookup(each.value, "install_source", null), var.default_install_source)
  network                 = var.network
  registrar_url           = var.registrar_url
  pdp_lotus_endpoint      = var.pdp_lotus_endpoint
  use_secrets_manager     = var.use_secrets_manager
  service_pem_content     = lookup(each.value, "service_pem_content", "")
  wallet_hex_content      = lookup(each.value, "wallet_hex_content", "")
  operator_email          = each.value.operator_email
  use_letsencrypt_staging = var.environment != "production" && var.environment != "prod"

  tags = {
    Owner = var.owner
    Team  = var.team
    Org   = var.org
  }
}