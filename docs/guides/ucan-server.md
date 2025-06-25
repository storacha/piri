# Piri UCAN Server Setup Guide

This guide walks you through setting up a Piri UCAN (User Controlled Authorization Network) server. 
The UCAN server accepts data uploads from Storacha network clients and routes them to a PDP backend.

## Overview

The Piri UCAN server:
- Provides the client-facing API for the Storacha network
- Handles UCAN-based authentication and authorization
- Routes uploaded data to PDP servers (Piri or Curio)
- Manages delegations from the Storacha network

## Prerequisites

Before starting, ensure you have:

1. ✅ [Met system prerequisites](../common/prerequisites.md)
2. ✅ [Installed Piri](../common/piri-installation.md)
3. ✅ [Generated a PEM file](../common/key-generation.md) with Ed25519 key
4. ✅ Access to a PDP server (Piri or Curio) at an HTTPS endpoint
5. ✅ [Configured TLS termination](../common/tls-termination.md) for your domain

### PDP Backend Requirement

You need ONE of the following:
- **Piri PDP Server**: Follow the [PDP server guide](./pdp-server-piri.md)
- **Curio**: See [Curio integration guide](../integrations/piri-with-curio.md)

Your PDP backend must be accessible via HTTPS (e.g., `https://pdp.example.com`).

## Step 1: Prepare Your Identity

### Derive Your DID

Using the PEM file you generated:

```bash
# Extract your DID
piri identity parse service.pem
```

Example output: `did:key:z6MkhaXgD8CkBJPgWK2mPx7kW6KnvgCx6LJfnv4jkLd4VWQR`

Save this DID - you'll need it for Storacha registration.

### For Curio Integration

If using Curio as your PDP backend:

1. Extract the public key from your PEM:
   ```bash
   openssl pkey -in service.pem -pubout
   ```

2. Add this public key to Curio's configuration as a service named `storacha`
3. See the [Curio integration guide](../integrations/piri-with-curio.md) for details

## Step 2: Create a Proof Set

Before accepting data, you need to create a proof set on the PDP backend:

```bash
piri client pdp proofset create \
  --key-file=service.pem \
  --node-url=https://YOUR_PDP_DOMAIN \
  --record-keeper=0x6170dE2b09b404776197485F3dc6c968Ef948505
```

Note: The [record-keeper](https://github.com/FilOzone/pdp/?tab=readme-ov-file#contracts) address is the contract address on the Calibration network.

### Monitor Creation

Check the status of your proof set creation:

```bash
piri client pdp proofset status \
  --key-file=service.pem \
  --node-url=https://YOUR_PDP_DOMAIN \
  --ref-url=/pdp/proof-set/created/HASH_FROM_CREATE_OUTPUT
```

Wait until you see:
```json
{
  "proofsetCreated": true,
  "txStatus": "confirmed",
  "ok": true,
  "proofSetId": YOUR_PROOFSET_ID
}
```

Save your `proofSetId` for reference.

## Step 3: Register with Storacha Delegator

### Registration Process

1. **Start Piri temporarily** to prepare for delegation:
   ```bash
   piri serve ucan --key-file=service.pem
   ```

2. **Share your DID** with the Storacha team via appropriate channels

3. **Visit the Delegator**:
   - https://staging.delegator.storacha.network

4. **Follow instructions** to:
   - Receive your delegation proof
   - Generate a delegation proof
   - Environment configuration

5. **Stop the temporary server** (Ctrl+C) after receiving your configuration

### Save Configuration

Create a `.env` file with the configuration received:

```bash
# Example .env file
INDEXING_SERVICE_DID="did:web:staging.indexing.storacha.network"
INDEXING_SERVICE_URL="https://staging.indexing.storacha.network"
INDEXING_SERVICE_PROOF="bafyrei..."
UPLOAD_SERVICE_DID="did:web:staging.upload.storacha.network"
UPLOAD_SERVICE_URL="https://staging.upload.storacha.network"
...
```

## Step 4: Start the UCAN Server

### Load Configuration

```bash
source .env
```

### Start Command

```bash
piri serve ucan \
  --key-file=service.pem \
  --pdp-server-url=https://YOUR_PDP_DOMAIN \
  --port=3000
```

### Configuration Options

| Flag | Description | Default |
|------|-------------|---------|
| `--key-file` | Path to your PEM file | Required |
| `--pdp-server-url` | HTTPS URL of your PDP backend | Required |
| `--port` | Local port to listen on | 3000 |

### Production Example

```bash
piri serve ucan \
  --key-file=/etc/piri/service.pem \
  --pdp-server-url=https://pdp.example.com \
  --port=3000 \
```

## Step 5: Verify Operation

### Monitor Logs

```bash
# If running in foreground
# Logs appear in terminal

# If using systemd
journalctl -u piri-ucan -f
```

## Next Steps

- Configure monitoring and alerting
- Set up log aggregation
- Plan for scaling (load balancing)
- Review [architecture](../architecture.md) for system design
- Consider [full stack setup](../integrations/full-stack-setup.md)