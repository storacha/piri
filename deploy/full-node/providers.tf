terraform {
  required_version = ">= 1.0"
  
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.5"
    }
  }
}

provider "aws" {
  region = var.region
  #allowed_account_ids = var.allowed_account_ids
  default_tags {
    tags = {
      "Environment" = terraform.workspace
      "ManagedBy" = "OpenTofu"
      Owner         = var.owner
      Team          = var.team
      Organization  = var.org
      Project       = var.app
    }
  }
}
