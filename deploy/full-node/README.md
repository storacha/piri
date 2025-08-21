# Piri Terraform Deployment

This Terraform configuration deploys a Piri node on AWS EC2 with the following components:

- EC2 instance (m6a.xlarge) with EBS storage
- Dedicated VPC with public subnet
- Auto-generated domain name with Route53
- Nginx with Let's Encrypt TLS certificate
- Systemd service management

## Prerequisites

1. AWS credentials configured
2. SSH key pair named `warm-storage-staging` in your AWS account
3. Access to the `warm.storacha.network` Route53 hosted zone

## Required Variables

Create a `terraform.tfvars` file with the following variables:

```hcl
# Installation method: "version" or "branch"
install_method = "version"
install_source = "v0.0.12"  # or branch name like "main"

# PDP Configuration
pdp_lotus_endpoint   = "wss://lotus.example.com/rpc/v1"
pdp_owner_address    = "0x..."
pdp_contract_address = "0x..."

# Operator Configuration
operator_email = "your-email@example.com"
# registrar_url = "https://staging.registrar.storacha.network"  # Optional, has default

# Sensitive files (temporary approach)
service_pem_content = <<EOF
-----BEGIN PRIVATE KEY-----
...
-----END PRIVATE KEY-----
EOF

wallet_hex_content = "..."

# Optional
aws_region      = "us-east-1"
ebs_volume_size = 200
```

## Deployment

1. Initialize Terraform:
   ```bash
   terraform init
   ```

2. Plan the deployment:
   ```bash
   terraform plan
   ```

3. Apply the configuration:
   ```bash
   terraform apply
   ```

## Outputs

After deployment, Terraform will output:

- `instance_id`: EC2 instance ID
- `public_ip`: Public IP address
- `domain_name`: Auto-generated domain (e.g., `staging.piri-abc123.warm.storacha.network`)
- `ssh_command`: SSH connection command
- `service_url`: HTTPS URL for the Piri service
- `ebs_volume_id`: EBS volume ID

## Access

SSH into the instance:
```bash
ssh -i ~/.ssh/warm-storage-staging.pem ubuntu@<public_ip>
```

Check service status:
```bash
sudo systemctl status piri
sudo journalctl -u piri -f
```

## Future Improvements

- TODO: Migrate sensitive files to AWS Secrets Manager for GitHub CD
- TODO: Add CloudWatch monitoring and alerts
- TODO: Implement automated backups

## Multiple Deployments

Each deployment creates a unique instance with its own domain. Run `terraform apply` multiple times with different state files or workspaces to create a network of nodes.