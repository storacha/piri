# Installing Piri

This guide covers the installation of the Piri binary, which is used for both PDP and UCAN server deployments.

## Installation Methods

### Method 1: Download Pre-compiled Binary (Recommended)

Downloading the pre-compiled binary is the quickest way to get started with Piri.

Download the latest release from [v0.0.6](https://github.com/storacha/piri/releases/tag/v0.0.6):

```bash
# For Linux AMD64
wget https://github.com/storacha/piri/releases/download/v0.0.6/piri_0.0.6_linux_amd64.tar.gz
tar -xzf piri_0.0.6_linux_amd64.tar.gz
sudo mv piri /usr/local/bin/piri

# For Linux ARM64
wget https://github.com/storacha/piri/releases/download/v0.0.6/piri_0.0.6_linux_arm64.tar.gz
tar -xzf piri_0.0.6_linux_arm64.tar.gz
sudo mv piri /usr/local/bin/piri
```

### Method 2: Build from Source (Alternative)

Building from source ensures you have the latest version with network-specific optimizations.

#### Clone and Build

```bash
# Clone the repository
git clone https://github.com/storacha/piri
cd piri

# Checkout the specific version
git checkout v0.0.6

# Build
make

# Install binary
sudo cp piri /usr/local/bin/piri
```

## Post-Installation Setup

### 1. Verify Installation

```bash
# View available commands
piri --help
```

### 2. Create Working Directory

```bash
# Create directory structure
sudo mkdir -p /etc/piri
sudo mkdir -p /var/lib/piri
sudo mkdir -p /var/log/piri

# Set permissions (if running as non-root user)
sudo chown -R $USER:$USER /etc/piri /var/lib/piri /var/log/piri
```

### 3. Configure Environment

Create `/etc/piri/env` for common environment variables:

```bash
# Logging
export GOLOG_LOG_LEVEL="info"
export GOLOG_FILE="/var/log/piri/piri.log"

# Data directory
export PIRI_DATA_DIR="/var/lib/piri"
```

Load in your shell:
```bash
source /etc/piri/env
```

## Systemd Service Setup (Optional)

For production deployments, run Piri as a systemd service.

### PDP Server Service

Create `/etc/systemd/system/piri-pdp.service`:

```ini
[Unit]
Description=Piri PDP Server
After=network.target

[Service]
Type=simple
User=piri
Group=piri
EnvironmentFile=/etc/piri/env
ExecStart=/usr/local/bin/piri serve pdp \
  --lotus-url=wss://LOTUS_ENDPOINT/rpc/v1 \
  --eth-address=YOUR_ETH_ADDRESS \
  --endpoint=:3001
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

### UCAN Server Service

Create `/etc/systemd/system/piri-ucan.service`:

```ini
[Unit]
Description=Piri UCAN Server
After=network.target piri-pdp.service
Requires=piri-pdp.service

[Service]
Type=simple
User=piri
Group=piri
EnvironmentFile=/etc/piri/env
ExecStart=/usr/local/bin/piri serve ucan \
  --key-file=/etc/piri/service.pem \
  --node-url=https://YOUR_PDP_DOMAIN \
  --port=3000
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

### Enable Services

```bash
# Create piri user
sudo useradd -r -s /bin/false piri

# Reload systemd
sudo systemctl daemon-reload

# Enable and start services
sudo systemctl enable piri-pdp
sudo systemctl enable piri-ucan
sudo systemctl start piri-pdp
sudo systemctl start piri-ucan

# Check status
sudo systemctl status piri-pdp
sudo systemctl status piri-ucan
```

## Updating Piri

### Binary Updates (Recommended)

```bash
# Download new version (replace with latest version as needed)
wget https://github.com/storacha/piri/releases/download/v0.0.6/piri_0.0.6_linux_amd64.tar.gz
tar -xzf piri_0.0.6_linux_amd64.tar.gz

# Replace binary
sudo systemctl stop piri-pdp piri-ucan
sudo mv piri /usr/local/bin/piri
sudo systemctl start piri-pdp piri-ucan
```

### From Source

```bash
cd piri
git fetch --tags
git checkout v0.0.6  # or latest version
make
sudo systemctl stop piri-pdp piri-ucan
sudo cp piri /usr/local/bin/
sudo systemctl start piri-pdp piri-ucan
```

## Next Steps

After installation:
1. [Generate PEM file](./key-generation.md) for identity
2. [Configure TLS](./tls-termination.md) for production
3. Follow specific guides:
   - [PDP Server Setup](../guides/pdp-server-piri.md)
   - [UCAN Server Setup](../guides/ucan-server.md)
