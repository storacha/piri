# Prerequisites

This document outlines the common prerequisites for running Piri services. 
Specific services may have additional requirements noted in their respective guides.

## System Requirements

### Operating System
- Linux-based OS (Ubuntu 20.04+ recommended)
- macOS (for development)

### Hardware Requirements

**Minimum (Development)**
- 2 CPU cores
- 4 GB RAM
- 20 GB free disk space

**Recommended (Production)**
- 4+ CPU cores
- 8+ GB RAM
- 1+ TB storage (depends on data volume)
- 1+ Gbps Symmetric network connection (depends on data volume)

## Software Dependencies

### Required Packages

Install the following packages:

```bash
# Debian/Ubuntu
apt update
apt install -y make git jq curl wget

# macOS (using Homebrew)
brew install make git jq curl wget

# RHEL/CentOS
yum install -y make git jq curl wget
```

### Go Language

Building Piri from source requires Go 1.24 or later:

```bash
# Download and install Go
wget https://go.dev/dl/go1.24.4.linux-amd64.tar.gz
tar -xzf go1.24.4.linux-amd64.tar.gz
sudo mv go /usr/local

# Add to PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Verify installation
go version
```

### TLS/Reverse Proxy (Production)

For production deployments, install a reverse proxy:

```bash
# Option 1: Nginx (recommended)
apt install -y nginx certbot python3-certbot-nginx

# Option 2: Caddy
# See TLS termination guide for installation
```

## Network Requirements

### Domain Name
- A fully qualified domain name (FQDN)
- DNS A record pointing to your server's IP
- Examples: `storage.example.com`, `pdp.mycompany.io`

### Firewall Ports
Open the following ports:

```bash
# HTTP/HTTPS (required)
ufw allow 80/tcp
ufw allow 443/tcp

# Piri services (if not using reverse proxy)
ufw allow 3000/tcp  # UCAN server
ufw allow 3001/tcp  # PDP server
```

## Service-Specific Prerequisites

### PDP Server Additional Requirements

1. **Lotus Node** (Calibration Network)
   - Synced Lotus node with RPC access
   - WebSocket endpoint (e.g., `wss://lotus.example.com/rpc/v1`)
   - Basic understanding of Filecoin primitives

2. **Funded Wallet**
   - Delegated Filecoin address with funds
   - Access to Calibration network [faucet](https://faucet.calibnet.chainsafe-fil.io/funds.html)

### UCAN Server Additional Requirements

1. **PDP Backend**
   - Running PDP server (Piri or Curio)
   - HTTPS endpoint (e.g., `https://pdp.example.com`)

2. **Storacha Registration**
   - DID derived from your Ed25519 key in `service.pem`
   - Contact with Storacha team for delegation

## Environment Checks

Before proceeding, verify your environment:

```bash
# Check Go version
go version | grep -E "go1\.(2[3-9]|[3-9][0-9])"

# Check required commands
for cmd in make git jq curl wget; do
  command -v $cmd >/dev/null 2>&1 || echo "$cmd is not installed"
done

# Check domain resolution
dig +short YOUR_DOMAIN

# Check open ports (from external machine)
nc -zv YOUR_DOMAIN 443
```

## Security Considerations

1. **User Permissions**
   - Create dedicated user for running Piri services
   - Avoid running as root in production

2. **File Permissions**
   - Secure PEM files: `chmod 600 *.pem`

## Next Steps

After meeting prerequisites:
1. [Install Piri](./piri-installation.md)
2. [Generate PEM file](./key-generation)
3. [Configure TLS termination](./tls-termination.md)
4. Follow service-specific guides
