# Installing Piri

This guide covers the installation of the Piri binary, which is used for both PDP and UCAN server deployments.

## Installation Methods

### Method 1: Build from Source (Recommended)

Building from source ensures you have the latest version with network-specific optimizations.

#### Clone and Build

```bash
# Clone the repository
git clone https://github.com/storacha/piri
cd piri

# Build for your target network
make calibnet    # For Calibration testnet
# OR
make mainnet     # For Mainnet (when available)

# Install binary
sudo cp piri /usr/local/bin/piri

# Verify installation
piri version
```

#### Build Targets

- `make calibnet`: Builds with Calibration network parameters
- `make mainnet`: Builds with Mainnet parameters

### Method 2: Download Pre-built Binary

Check the [releases page](https://github.com/storacha/piri/releases) for pre-built binaries:

```bash
# Example for Linux amd64
wget https://github.com/storacha/piri/releases/download/vX.Y.Z/piri-linux-amd64
chmod +x piri-linux-amd64
sudo mv piri-linux-amd64 /usr/local/bin/piri
```

### Method 3: Using Go Install

For Go developers:

```bash
go install github.com/storacha/piri/cmd/storage@latest
mv ~/go/bin/storage /usr/local/bin/piri
```

## Post-Installation Setup

### 1. Verify Installation

```bash
# Check version
piri version

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
  --lotus-host-url=wss://LOTUS_ENDPOINT/rpc/v1 \
  --pdp-address=YOUR_ETH_ADDRESS \
  --port=3001
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
ExecStart=/usr/local/bin/piri start \
  --key-file=/etc/piri/service.pem \
  --pdp-server-url=https://YOUR_PDP_DOMAIN \
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

## Docker Installation (Alternative)

For containerized deployments:

```dockerfile
FROM golang:1.23-alpine AS builder
RUN apk add --no-cache git make
WORKDIR /app
COPY . .
RUN make calibnet

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/piri /usr/local/bin/
ENTRYPOINT ["piri"]
```

Build and run:
```bash
docker build -t piri:latest .
docker run -p 3000:3000 piri:latest start --help
```

## Updating Piri

### From Source

```bash
cd piri
git pull origin main
make calibnet
sudo systemctl stop piri-pdp piri-ucan
sudo cp piri /usr/local/bin/
sudo systemctl start piri-pdp piri-ucan
```

### Binary Updates

```bash
# Download new version
wget https://github.com/storacha/piri/releases/download/vX.Y.Z/piri-linux-amd64

# Replace binary
sudo systemctl stop piri-pdp piri-ucan
sudo mv piri-linux-amd64 /usr/local/bin/piri
sudo chmod +x /usr/local/bin/piri
sudo systemctl start piri-pdp piri-ucan
```

## Next Steps

After installation:
1. [Generate PEM file](./key-generation) for identity
2. [Configure TLS](./tls-termination.md) for production
3. Follow specific guides:
   - [PDP Server Setup](../guides/pdp-server-piri.md)
   - [UCAN Server Setup](../guides/ucan-server.md)