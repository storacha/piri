# Full Stack Piri Setup Guide

This comprehensive guide walks you through deploying both Piri UCAN and PDP servers together, creating a complete storage provider setup for the Storacha network.

## Overview

The full stack deployment includes:
- **UCAN Server**: Accepts client uploads with authorization
- **PDP Server**: Stores data and generates proofs
- **Shared Identity**: Single PEM file for both services
- **Unified Infrastructure**: Simplified management and monitoring

Architecture:
```
┌─────────────────┐
│ Storacha Client │
└────────┬────────┘
         │ HTTPS
         ▼
    ┌─────────┐     ┌─────────────────┐     ┌──────────────┐
    │  Nginx  ├────▶│  UCAN Server    ├────▶│  PDP Server  │
    │  :443   │     │  :3000          │     │  :3001       │
    └─────────┘     └─────────────────┘     └──────┬───────┘
                                                   │
                                                   ▼
                                          ┌─────────────────┐
                                          │ Filecoin Network│
                                          └─────────────────┘
```

## Prerequisites

Before starting, ensure you have:

1. ✅ [System prerequisites met](../common/prerequisites.md)
2. ✅ [Piri installed](../common/piri-installation.md)
3. ✅ Lotus node access (Calibration or Mainnet)
4. ✅ Two domain names configured:
   - `storage.example.com` for UCAN server
   - `pdp.example.com` for PDP server
5. ✅ Sufficient disk space for piece storage

## Step 1: Initial Setup

### Create Directory Structure

```bash
# Create working directories
sudo mkdir -p /etc/piri
sudo mkdir -p /var/lib/piri/{ucan,pdp}
sudo mkdir -p /var/log/piri

# Create piri user
sudo useradd -r -s /bin/false piri

# Set ownership
sudo chown -R piri:piri /etc/piri /var/lib/piri /var/log/piri
```

### Generate Shared Identity

Create a single PEM file for both services:

```bash
# Generate Ed25519 key
cd /etc/piri
sudo -u piri piri identity generate > service.pem
sudo chmod 600 service.pem

# Extract and save your DID
sudo -u piri piri identity parse service.pem | jq .did > did.txt
cat did.txt
```

## Step 2: Setup PDP Server

### Create Delegated Wallet

```bash
# Create new delegated address
lotus wallet new delegated

# Example output: t410fzmmaqcn3j6jidbyrqqsvayejt6sskofwash6zoi

# Fund the wallet (use faucet for Calibration)
# https://faucet.calibnet.chainsafe-fil.io/funds.html

# Export wallet
lotus wallet export YOUR_DELEGATED_ADDRESS > /tmp/wallet.hex

# Import to Piri
piri wallet import /tmp/wallet.hex
rm /tmp/wallet.hex  # Clean up

# Get Ethereum-style address
piri wallet list
# Example: 0x7469B47e006D0660aB92AE560b27A1075EEcF97F
```

### Configure TLS for PDP

Following the [TLS guide](../common/tls-termination.md), create `/etc/nginx/sites-available/pdp.example.com`:

```nginx
server {
    server_name pdp.example.com;
    
    client_max_body_size 0;
    client_body_timeout 300s;
    client_header_timeout 300s;
    send_timeout 300s;
    
    location / {
        proxy_pass http://localhost:3001;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
        proxy_request_buffering off;
    }
}
```

Enable and secure:
```bash
sudo ln -s /etc/nginx/sites-available/pdp.example.com /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
sudo certbot --nginx -d pdp.example.com
```

### Create PDP Service

Create `/etc/systemd/system/piri-pdp.service`:

```ini
[Unit]
Description=Piri PDP Server
After=network.target

[Service]
Type=simple
User=piri
Group=piri
WorkingDirectory=/var/lib/piri/pdp
ExecStart=/usr/local/bin/piri serve pdp \
  --lotus-url=wss://YOUR_LOTUS_ENDPOINT/rpc/v1 \
  --eth-address=YOUR_ETH_ADDRESS \
  --endpoint=:3001 \
  --data-dir=/var/lib/piri/pdp \
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

### Start PDP Server

```bash
sudo systemctl daemon-reload
sudo systemctl enable piri-pdp
sudo systemctl start piri-pdp

# Check status
sudo systemctl status piri-pdp
sudo journalctl -u piri-pdp -f
```

## Step 3: Setup UCAN Server

### Create Proof Set

Using the running PDP server:

```bash
cd /etc/piri
sudo -u piri piri client pdp proofset create \
  --key-file=service.pem \
  --node-url=https://pdp.example.com \
  --record-keeper=0x6170dE2b09b404776197485F3dc6c968Ef948505

# Monitor status
sudo -u piri piri client pdp proofset status \
  --key-file=service.pem \
  --node-url=https://pdp.example.com \
  --ref-url=/pdp/proof-set/created/HASH_FROM_CREATE
```

### Register with Storacha

1. **Start temporary UCAN server**:
   ```bash
   piri serve ucan --key-file=/etc/piri/service.pem
   ```

2. **Share your DID** (from did.txt) with Storacha team

3. **Visit delegator**:
   - https://staging.delegator.storacha.network

4. **Save configuration**:
   ```bash
   vim /etc/piri/storacha.env
   # Add all environment variables received
   ```

5. **Stop temporary server** (Ctrl+C)

### Configure TLS for UCAN

Create `/etc/nginx/sites-available/storage.example.com`:

```nginx
server {
    server_name storage.example.com;
    
    client_max_body_size 0;
    client_body_timeout 300s;
    client_header_timeout 300s;
    send_timeout 300s;
    
    location / {
        proxy_pass http://localhost:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
        proxy_request_buffering off;
    }
}
```

Enable and secure:
```bash
sudo ln -s /etc/nginx/sites-available/storage.example.com /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
sudo certbot --nginx -d storage.example.com
```

### Create UCAN Service

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
WorkingDirectory=/var/lib/piri/ucan
EnvironmentFile=/etc/piri/storacha.env
ExecStart=/usr/local/bin/piri serve ucan \
  --key-file=/etc/piri/service.pem \
  --node-url=https://pdp.example.com \
  --port=3000 \
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

### Start UCAN Server

```bash
sudo systemctl daemon-reload
sudo systemctl enable piri-ucan
sudo systemctl start piri-ucan

# Check status
sudo systemctl status piri-ucan
sudo journalctl -u piri-ucan -f
```

## Next Steps

Your full Piri stack is now operational! Consider:

1. **Set up monitoring** with Prometheus/Grafana
2. **Configure alerts** for service failures
3. **Plan capacity** based on expected usage
4. **Document** your specific configuration
5. **Join** the Storacha provider community

## Additional Resources

- [Architecture Overview](../architecture.md) - System design
- [PDP Server Guide](../guides/pdp-server-piri.md) - PDP details
- [UCAN Server Guide](../guides/ucan-server.md) - UCAN details

Congratulations on joining the Storacha network! 🎉