# Integrating Piri UCAN Server with Curio

This guide helps existing Curio operators add Storacha network support by running a Piri UCAN server that uses Curio as the PDP backend.

## Overview

This integration allows you to:
- Keep your existing Curio infrastructure
- Add Storacha network compatibility
- Accept uploads from Storacha clients
- Leverage Curio's proven storage and proof capabilities

Architecture:
```
Storacha Clients → Piri UCAN Server → Curio → Filecoin Network
                  (This guide)      (Existing)
```

## Prerequisites

Before starting, ensure you have:

1. ✅ **Running Curio instance** with:
   - HTTPS API endpoint (e.g., `https://curio.example.com`)
   - Available storage capacity
   - Active proof generation

2. ✅ **System ready for Piri**:
   - [Prerequisites met](../common/prerequisites.md)
   - [Piri installed](../common/piri-installation.md)
   - [Keypair PEM file generated](../common/key-generation)

3. ✅ **Network requirements**:
   - Domain for UCAN server
   - [TLS termination configured](../common/tls-termination.md)

## Step 1: Configure Curio for Storacha

### Add Storacha Service to Curio

Curio needs to recognize the Piri UCAN server as an authorized service.

1. **Extract your public key**:
   ```bash
   # From your service.pem file
   openssl pkey -in service.pem -pubout
   ```

   Output example:
   ```
   -----BEGIN PUBLIC KEY-----
   MCowBQYDK2VwAyEAYRpJI+aZVmKSFqYyVhMcOOeKYqBisNpReAGj4qF8kEg=
   -----END PUBLIC KEY-----
   ```

2. **Configure Curio** to accept this key:
   
   Add the public key to Curio's configuration as a service named `storacha`. The exact method depends on your Curio version.

3. **Restart Curio** to apply configuration

## Step 2: Create Proof Set

Before accepting uploads, create a proof set in Curio:

```bash
piri proofset create \
  --key-file=service.pem \
  --pdp-server-url=https://curio.example.com \
  --record-keeper=0x6170dE2b09b404776197485F3dc6c968Ef948505
```

Monitor creation status:
```bash
piri proofset status \
  --key-file=service.pem \
  --curio-url=https://curio.example.com \
  --ref-url=/pdp/proof-set/created/HASH_FROM_CREATE
```

## Step 3: Register with Storacha

### Get Your DID

```bash
piri id parse service.pem | jq .did
```

### Registration Process

1. **Start Piri temporarily**:
   ```bash
   piri start --key-file=service.pem
   ```

2. **Contact Storacha team** with your DID

3. **Visit delegator**:
   - https://staging.delegator.storacha.network

4. **Save configuration** received to `.env` file

5. **Stop temporary server**

## Step 4: Configure and Start UCAN Server

### Environment Setup

```bash
# Load Storacha configuration
source .env
```

### Start Server

```bash
piri start \
  --key-file=service.pem \
  --curio-url=https://curio.example.com \
  --port=3000
```

### Production Systemd Service

Create `/etc/systemd/system/piri-ucan-curio.service`:

```ini
[Unit]
Description=Piri UCAN Server with Curio Backend
After=network.target

[Service]
Type=simple
User=piri
Group=piri
EnvironmentFile=/etc/piri/storacha.env
ExecStart=/usr/local/bin/piri start \
  --key-file=/etc/piri/service.pem \
  --curio-url=https://curio.example.com \
  --port=3000 \
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl daemon-reload
sudo systemctl enable piri-ucan-curio
sudo systemctl start piri-ucan-curio
```

## Additional Resources

- [UCAN Server Guide](../guides/ucan-server.md) - Detailed UCAN server information
- [Architecture Overview](../architecture.md) - System design details
- [Full Stack Setup](./full-stack-setup.md) - Running complete Piri infrastructure
- Curio documentation - Consult your Curio version's docs

For Curio-specific questions, consult the Filecoin community. For Piri integration issues, see the [GitHub repository](https://github.com/storacha/piri).