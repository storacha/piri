# Piri Full Node Tofu Deployment

This directory contains Terraform/OpenTofu infrastructure-as-code for deploying Piri full nodes on AWS. 

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

⚠️ **Important Note About PEM Keys**: 
- AWS Secrets Manager stores the PEM key with escaped newlines (`\n`) instead of actual line breaks
- The deployment automatically handles this by using `printf '%b\n'` to convert `\n` back to actual newlines
- Your PEM file can be stored as-is in AWS Secrets Manager - no manual formatting needed
- The user-data script handles both formats: proper newlines and escaped `\n` from AWS Secrets Manager

Then in your `tofu.tfvars`:
```hcl
use_secrets_manager = true
```

### Development
For development only, you can provide secrets inline in `tofu.tfvars` (never commit these!).

When providing PEM keys directly in `tofu.tfvars`:
```hcl
# The PEM content should include the actual newlines
service_pem_content = <<-EOT
-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQg...
...rest of key...
-----END PRIVATE KEY-----
EOT
```

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

## 🔐 SSL Certificates

### Let's Encrypt Configuration
- **Production environments** (`environment = "production", "prod" or "forge-prod"`): Use Let's Encrypt production certificates (trusted by browsers)
- **Non-production environments** (staging, dev, etc.): Use Let's Encrypt staging certificates (untrusted, no rate limits)

### Rate Limits
Let's Encrypt production environment has strict rate limits:
- **5 certificates per week** for the same domain
- Hitting this limit blocks certificate issuance for 168 hours
- Staging environment has no practical rate limits (perfect for testing)

### Staging Certificate Trust Issues
⚠️ **Important**: Staging certificates are signed by an untrusted CA. This causes:
- Browser security warnings when accessing the service (expected behavior)
- TLS verification errors for services connecting to Piri nodes
- Errors like: `x509: certificate signed by unknown authority`

To configure external services (like the delegator) to trust staging certificates:
```bash
# Install Let's Encrypt staging CA certificates on the client machine
wget https://letsencrypt.org/certs/staging/letsencrypt-stg-root-x1.pem
wget https://letsencrypt.org/certs/staging/letsencrypt-stg-root-x2.pem

# Install them in the system trust store (Ubuntu/Debian)
sudo cp letsencrypt-stg-root-x1.pem /usr/local/share/ca-certificates/letsencrypt-stg-root-x1.crt
sudo cp letsencrypt-stg-root-x2.pem /usr/local/share/ca-certificates/letsencrypt-stg-root-x2.crt
sudo update-ca-certificates

# Restart any services that need to trust the certificates
sudo systemctl restart your-service
```

## 🔄 Common Operations

### Update Piri Version
✅ **SAFE: Data is preserved during updates**

The module automatically replaces instances when `install_source` changes (via `user_data_replace_on_change = true`). Your data volume will persist and be reattached to the new instance.

**To update Piri:**
1. Update `install_source` in your `tofu.tfvars` (version tag or branch name)
2. Run `tofu apply -var-file=tofu.tfvars`
3. The instance will be automatically replaced with the new version
4. Your data volume remains intact and is reattached

**To force instance replacement** (if needed):
```bash
tofu apply -replace=module.piri_instance.aws_instance.piri -var-file=tofu.tfvars
```

**Note**: Volume protection is environment-based:
- **Production** (`environment = "production", "prod" or "forge-prod"`): Volumes are protected from destruction
- **Other environments** (staging, dev, etc.): Volumes can be destroyed with `tofu destroy`

### Scale Storage
✅ **SAFE: Manual resizing is supported**

The module now ignores volume size changes to allow manual resizing without data loss.

**To resize your volume:**
1. Use AWS Console or CLI to modify the volume size in-place:
   ```bash
   aws ec2 modify-volume --volume-id <volume-id> --size <new-size>
   ```
2. SSH to instance and extend the filesystem:
   ```bash
   ssh -i ~/.ssh/your-key.pem ubuntu@<instance-ip>
   sudo resize2fs /dev/nvme1n1
   ```
3. Update `ebs_volume_size` in your `tofu.tfvars` to match the new size (for documentation purposes)

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

## 🔧 Troubleshooting

### Certificate Rate Limit Errors
**Error**: `too many certificates (5) already issued for this exact set of identifiers in the last 168h0m0s`

**Cause**: You've hit Let's Encrypt's production rate limit (5 certificates per week per domain).

**Solution**: 
- Non-production environments automatically use staging certificates to avoid this
- Wait for the rate limit to reset (168 hours)
- Or use a different domain temporarily

### TLS Verification Errors
**Error**: `x509: certificate signed by unknown authority`

**Cause**: Services are trying to connect to Piri nodes that use staging certificates (non-production).

**Solution**: Configure the connecting service to trust Let's Encrypt staging CA certificates (see SSL Certificates section above).

### Instance Not Updating After Changing install_source
**Problem**: Changed `install_source` but Piri is still running the old version.

**Cause**: The instance wasn't replaced (check the instance launch time in AWS console).

**Solution**: Force instance replacement:
```bash
tofu apply -replace=module.piri_instance.aws_instance.piri -var-file=tofu.tfvars
```

### Volume Not Persisting Data
**Problem**: Data is lost after instance replacement.

**Possible Causes**:
- The volume was accidentally destroyed
- The volume didn't mount correctly

**Debug Steps**:
```bash
# SSH to the instance
ssh -i ~/.ssh/your-key.pem ubuntu@<instance-ip>

# Check if volume is mounted
df -h | grep /data

# Check mount logs
sudo journalctl -u cloud-init | grep -i mount
```

## ⚠️ Important Notes

1. **Secrets Security**: Never commit secrets to version control
2. **Region Consistency**: Ensure secrets are in the same region as your deployment
3. **SSH Keys**: The default `warm-storage-staging` key is in the Storacha 1Password vault
4. **Domain Management**: The default `pdp.storacha.network` zone is managed by Storacha
5. **Instance Replacement**: Changing certain parameters (like user-data) will replace the instance, but volumes are preserved
6. **Volume Protection**: Production volumes are protected from `tofu destroy`; other environments allow destruction
7. **Environment-Based Behavior**: Set `environment = "production"`, `"prod"` or `"forge-prod"` for maximum data protection

## 📚 Additional Resources

- [Piri Documentation](https://github.com/storacha/piri)
- [PDP Smart Contract](https://github.com/FilOzone/pdp)
- [Storacha Network](https://storacha.network)
- [OpenTofu Documentation](https://opentofu.org/docs)

---

For detailed configuration options and examples, refer to the template files:
- **[Single Instance Template](deployment-types/single-instance/tofu.tfvars.template)** - Complete guide for single node deployments
- **[Multi-Instance Template](deployment-types/multi-instance/tofu.tfvars.template)** - Complete guide for fleet deployments