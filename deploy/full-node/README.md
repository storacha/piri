# Piri Full Node Tofu Deployment

This directory contains Terraform/OpenTofu infrastructure-as-code for deploying Piri full nodes on AWS. Piri is a Proof Data Processor (PDP) node that participates in the Filecoin storage network by validating and processing storage proofs.

## 🎯 Purpose

This Tofu configuration enables automated deployment of:
- **Single Piri nodes**
- **Multiple Piri nodes**
- Complete AWS infrastructure including networking, security, storage, and SSL certificates

## 📁 Directory Structure

```
deploy/full-node/
├── deployment-types/       # Ready-to-use deployment configurations
│   ├── single-instance/    # Deploy one Piri node
│   └── multi-instance/     # Deploy multiple Piri nodes
├── modules/                # Reusable Tofu modules
│   ├── base-infrastructure/  # Shared AWS resources (VPC, security groups, IAM)
│   └── piri-instance/        # Per-instance resources (EC2, EBS, Route53)
└── README.md              # This file ;)
```

## 🏗️ Architecture

The deployment uses a modular architecture for flexibility and reusability:

### Base Infrastructure Module
Manages shared resources across all instances:
- VPC with public subnet
- Internet gateway and routing
- Security groups (HTTP/HTTPS/SSH)
- IAM roles and instance profiles

### Piri Instance Module  
Manages per-instance resources:
- EC2 instance with user-data bootstrapping
- EBS volume for data persistence
- Route53 DNS record
- Nginx reverse proxy with SSL (Let's Encrypt)
- Systemd service configuration

## 🚀 Quick Start

### Choose Your Deployment Type

#### Option 1: Single Instance Deployment
Best for individual operators running one node.

```bash
cd deployment-types/single-instance
cp tofu.tfvars.template tofu.tfvars
# Edit tofu.tfvars with your configuration
tofu init
tofu apply -var-file=tofu.tfvars
```

**📖 [View Single Instance Configuration Guide](deployment-types/single-instance/tofu.tfvars.template)**

#### Option 2: Multi-Instance Deployment
Best for operators running multiple nodes.

```bash
cd deployment-types/multi-instance
cp tofu.tfvars.template tofu.tfvars
# Edit tofu.tfvars with your configuration
tofu init
tofu apply -var-file=tofu.tfvars
```

**📖 [View Multi-Instance Configuration Guide](deployment-types/multi-instance/tofu.tfvars.template)**

## 📋 Prerequisites

### Required Tools
- [OpenTofu](https://opentofu.org/docs/intro/install/) >= 1.0 or [Terraform](https://www.terraform.io/downloads) >= 1.0
- AWS CLI configured with appropriate credentials
- SSH key pair in your target AWS region

### AWS Requirements
- AWS account with appropriate permissions
- Route53 hosted zone (default: `pdp.storacha.network`)
- Secrets in AWS Secrets Manager (for production deployments)

### Piri Requirements
- Service private key (Ed25519)
- Wallet private key (for blockchain transactions)
- Access to a Lotus WebSocket endpoint

## 🔑 Secrets Management

### Production (Recommended)
Use AWS Secrets Manager to store sensitive data:

```bash
# Store secrets in AWS (example for staging environment)
aws secretsmanager create-secret \
  --name staging/piri/primary/service-key \
  --secret-string "$(cat service.pem)" \
  --region us-west-2

aws secretsmanager create-secret \
  --name staging/piri/primary/wallet-key \
  --secret-string "$(cat wallet.hex)" \
  --region us-west-2
```

Then in your `tofu.tfvars`:
```hcl
use_secrets_manager = true
```

### Development
For development only, you can provide secrets inline in `tofu.tfvars` (never commit these!).

## 🌐 Domain Structure

Domains follow this pattern:
- **Single Instance**: `{environment}.{app}.{root_domain}`
  - Example: `staging.piri.pdp.storacha.network`
- **Multi-Instance**: `{environment}.{subdomain}.{root_domain}`
  - Example: `staging.node1.pdp.storacha.network`

## 💾 Instance Recommendations

### Instance Types (m6a family recommended)
- **m6a.large** (2 vCPU, 8 GB RAM): Development/testing
- **m6a.xlarge** (4 vCPU, 16 GB RAM): Standard production
- **m6a.2xlarge** (8 vCPU, 32 GB RAM): High-performance production

The m6a family is recommended for SHA extension support, improving CommP hash performance.

### Storage Sizing
- **50-100 GB**: Development/testing
- **200-500 GB**: Standard production
- **500+ GB**: High-volume production

## 📊 Monitoring

After deployment, monitor your nodes:

```bash
# SSH to instance
ssh -i ~/.ssh/your-key.pem ubuntu@<instance-ip>

# View service logs of piri
sudo journalctl -u piri -f

# Check nginx logs
sudo tail -f /var/log/nginx/access.log
sudo tail -f /var/log/nginx/error.log

# View initialization logs
sudo cat /var/log/cloud-init-output.log
```

## 🔄 Common Operations

### Update Piri Version
⚠️ **WARNING: This process will DESTROY your data volume!**

1. **Backup your data first** (if needed):
   ```bash
   ssh -i ~/.ssh/your-key.pem ubuntu@<instance-ip>
   sudo tar -czf /tmp/piri-backup.tar.gz /data/piri
   # Copy backup locally
   scp -i ~/.ssh/your-key.pem ubuntu@<instance-ip>:/tmp/piri-backup.tar.gz .
   ```
2. Update `install_source` in your `tofu.tfvars`
3. Run `tofu apply -var-file=tofu.tfvars`
4. The instance AND data volume will be recreated (data will be lost)

**Note**: Due to how the EBS volume is configured, it depends on the instance's availability zone. When the instance is replaced, the volume is also destroyed and recreated. This is a limitation of the current module design.

### Scale Storage
⚠️ **WARNING: This will DESTROY your data volume!**

The current module design will replace the entire EBS volume when resizing, causing data loss.

**DO NOT use this method. Instead:**
1. Use AWS Console or CLI to modify the volume size in-place
2. SSH to instance and extend the filesystem manually
3. Or backup data first, then recreate with new size

### Add More Nodes (Multi-Instance)
✅ **Safe Operation**

1. Add new instance definitions to `instances` map in `tofu.tfvars`
2. Create corresponding secrets in AWS Secrets Manager
3. Run `tofu apply -var-file=tofu.tfvars`
4. New nodes will be created without affecting existing nodes

## 🛟 Support

- **Storacha Discord**: Join for community support
- **Issues**: Report bugs in the [Piri repository](https://github.com/storacha/piri)
- **Lotus Gateway Access**: Contact the Storacha team for access to their Lotus gateway

## ⚠️ Important Notes

1. **Secrets Security**: Never commit secrets to version control
2. **Region Consistency**: Ensure secrets are in the same region as your deployment
3. **SSH Keys**: The default `warm-storage-staging` key is in the Storacha 1Password vault
4. **Domain Management**: The default `pdp.storacha.network` zone is managed by Storacha
5. **Instance Replacement**: Changing certain parameters (like user-data) will replace the instance

## 📚 Additional Resources

- [Piri Documentation](https://github.com/storacha/piri)
- [PDP Smart Contract](https://github.com/FilOzone/pdp)
- [Storacha Network](https://storacha.network)
- [OpenTofu Documentation](https://opentofu.org/docs)

---

For detailed configuration options and examples, refer to the template files:
- **[Single Instance Template](deployment-types/single-instance/tofu.tfvars.template)** - Complete guide for single node deployments
- **[Multi-Instance Template](deployment-types/multi-instance/tofu.tfvars.template)** - Complete guide for fleet deployments