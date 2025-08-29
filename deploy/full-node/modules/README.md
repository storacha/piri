# Piri Terraform Modules

This directory contains reusable Terraform modules for deploying Piri nodes on AWS.

## Architecture

The deployment is split into two main modules:

### 1. Base Infrastructure Module (`base-infrastructure`)
Manages shared infrastructure resources that are common across all Piri instances:
- VPC and networking components
- Security groups
- IAM roles and instance profiles

### 2. Piri Instance Module (`piri-instance`)
Manages resources specific to each Piri instance:
- EC2 instance
- EBS data volume
- Route53 DNS record
- Instance configuration and bootstrapping

## Module Structure

```
modules/
├── base-infrastructure/
│   ├── main.tf         # VPC, subnets, security groups, IAM
│   ├── variables.tf    # Input variables
│   └── outputs.tf      # Exported values
└── piri-instance/
    ├── main.tf         # EC2, EBS, Route53
    ├── variables.tf    # Input variables
    ├── outputs.tf      # Exported values
    ├── files/          # Configuration templates
    └── scripts/        # Installation scripts
```

## Usage Examples

### Single Instance Deployment

```hcl
module "base_infrastructure" {
  source = "./modules/base-infrastructure"
  
  environment = "staging"
}

module "piri_instance" {
  source = "./modules/piri-instance"
  
  name                      = "primary"
  environment               = "staging"
  subnet_id                 = module.base_infrastructure.public_subnet_id
  security_group_id         = module.base_infrastructure.security_group_id
  iam_instance_profile_name = module.base_infrastructure.iam_instance_profile_name
  internet_gateway_id       = module.base_infrastructure.internet_gateway_id
  
  domain_name          = "piri.example.com"
  route53_zone_id      = data.aws_route53_zone.primary.zone_id
  service_pem_content  = var.service_pem_content
  wallet_hex_content   = var.wallet_hex_content
  operator_email       = "operator@example.com"
  
  # ... other configuration
}
```

### Multi-Instance Deployment

```hcl
module "base_infrastructure" {
  source = "./modules/base-infrastructure"
  
  environment = "production"
}

module "piri_instances" {
  source   = "./modules/piri-instance"
  for_each = var.instances
  
  name                      = each.key
  environment               = "production"
  subnet_id                 = module.base_infrastructure.public_subnet_id
  security_group_id         = module.base_infrastructure.security_group_id
  iam_instance_profile_name = module.base_infrastructure.iam_instance_profile_name
  internet_gateway_id       = module.base_infrastructure.internet_gateway_id
  
  domain_name          = "${each.value.subdomain}.example.com"
  route53_zone_id      = data.aws_route53_zone.primary.zone_id
  service_pem_content  = each.value.service_pem_content
  wallet_hex_content   = each.value.wallet_hex_content
  operator_email       = each.value.operator_email
  
  # ... other configuration
}
```

## Key Features

### Modular Design
- **Separation of Concerns**: Infrastructure and instance resources are managed separately
- **Reusability**: Instance module can be instantiated multiple times
- **Flexibility**: Easy to customize per-instance configuration

### Security
- Security groups configured for HTTPS, HTTP, and SSH
- IAM roles with minimal required permissions
- Sensitive data handled through Terraform variables

### Scalability
- Support for single or multiple instances
- Shared infrastructure reduces resource duplication
- Per-instance customization (instance type, storage, etc.)

### High Availability
- EBS volumes persist independently of instances
- Route53 DNS management for easy failover
- Instance-specific monitoring and logging

## Complete Examples

See the `examples/` directory for complete, working examples:
- `examples/single-instance/` - Deploy a single Piri node
- `examples/multi-instance/` - Deploy multiple Piri nodes

## Migration from Monolithic Configuration

To migrate from the original monolithic configuration:

1. Move to the appropriate example directory
2. Copy `terraform.tfvars.template` to `terraform.tfvars`
3. Fill in your configuration values
4. Run:
   ```bash
   terraform init
   terraform plan
   terraform apply
   ```

## Best Practices

1. **State Management**: Use remote state backend (S3 + DynamoDB) for production
2. **Secrets Management**: Consider using AWS Secrets Manager or Parameter Store for sensitive data
3. **Monitoring**: Add CloudWatch alarms and monitoring for production deployments
4. **Backup Strategy**: Implement EBS snapshot policies for data volumes
5. **Network Segmentation**: Consider using private subnets with NAT gateways for enhanced security

## Module Inputs and Outputs

### Base Infrastructure Module

**Key Inputs:**
- `environment` - Environment name (dev/staging/prod)
- `vpc_cidr` - CIDR block for VPC
- `ssh_cidr_blocks` - Allowed CIDR blocks for SSH access

**Key Outputs:**
- `vpc_id` - VPC ID
- `public_subnet_id` - Public subnet ID
- `security_group_id` - Security group ID
- `iam_instance_profile_name` - IAM instance profile name

### Piri Instance Module

**Key Inputs:**
- `name` - Instance identifier
- `domain_name` - Full domain name for the instance
- `service_pem_content` - Service private key
- `wallet_hex_content` - Wallet private key
- `operator_email` - Operator email address

**Key Outputs:**
- `instance_id` - EC2 instance ID
- `public_ip` - Public IP address
- `domain_name` - Full domain name
- `ebs_volume_id` - EBS volume ID