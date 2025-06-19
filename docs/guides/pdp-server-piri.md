# Piri PDP Server Setup Guide

This guide walks you through setting up a Piri PDP (Proof of Data Possession) server. The PDP server stores data pieces and generates cryptographic proofs for the Filecoin network.

## Overview

The Piri PDP server:
- Accepts and stores data pieces from UCAN servers
- Generates proofs of data possession
- Submits proofs to smart contracts on Filecoin
- Manages piece lifecycle and retrieval

## Prerequisites

Before starting, ensure you have:

1. ✅ [Met system prerequisites](../common/prerequisites.md)
2. ✅ [Installed Piri](../common/piri-installation.md)
3. ✅ A synced Lotus node on Calibration network with WebSocket RPC endpoint
4. ✅ Basic understanding of Filecoin addresses and wallets

## Step 1: Create a Delegated Filecoin Address

PDP servers require a [delegated address](https://docs.filecoin.io/smart-contracts/filecoin-evm-runtime/address-types#delegated-addresses) for contract interactions.

### Generate Address

```bash
lotus wallet new delegated
```

Example output: `t410fzmmaqcn3j6jidbyrqqsvayejt6sskofwash6zoi`

### Fund the Address

1. Visit the [Calibration faucet](https://faucet.calibnet.chainsafe-fil.io/funds.html)
2. Request funds for your new address
3. Verify funding:
   ```bash
   lotus wallet balance YOUR_DELEGATED_ADDRESS
   ```

### Export for Piri

```bash
# Export wallet to hex format
lotus wallet export YOUR_DELEGATED_ADDRESS > wallet.hex

# Import to Piri
piri wallet import wallet.hex

# Verify import (note the Ethereum-style address)
piri wallet list
```

Example output: `Address: 0x7469B47e006D0660aB92AE560b27A1075EEcF97F`

## Step 2: Configure TLS Termination

For production deployments, configure HTTPS access:

1. Follow the [TLS termination guide](../common/tls-termination.md)
2. Use port 3001 for the PDP server
3. Ensure your domain (e.g., `pdp.example.com`) is properly configured

## Step 3: Start the PDP Server

### Basic Start Command

```bash
piri serve pdp \
  --lotus-host-url=wss://YOUR_LOTUS_ENDPOINT/rpc/v1 \
  --pdp-address=YOUR_ETH_ADDRESS \
  --port=3001
```

### Configuration Options

| Flag | Description | Default |
|------|-------------|---------|
| `--lotus-host-url` | Lotus WebSocket RPC endpoint (provides both Lotus and Ethereum API) | Required |
| `--pdp-address` | Your Ethereum-style wallet address | Required |
| `--port` | Local port to listen on | 3001 |
| `--storage-dir` | Directory for storing pieces | `./storage` |
| `--log-level` | Logging verbosity (debug/info/warn/error) | info |

### Production Configuration

For production, consider:

```bash
piri serve pdp \
  --lotus-host-url=wss://lotus.example.com/rpc/v1 \
  --pdp-address=0x7469B47e006D0660aB92AE560b27A1075EEcF97F \
  --port=3001 \
  --storage-dir=/var/lib/piri/pdp-storage \
```

## Step 4: Verify Operation

### View Logs

Monitor server activity:
```bash
# If running in foreground
# Logs appear in terminal

# If using systemd
journalctl -u piri-pdp -f
```

## Step 5: Monitor Your ProofSets

You can inspect your ProofSets and verify proof submissions using the PDP explorer:

🔍 **[PDP Explorer (Calibration Network)](https://pdpscan.vercel.app/calibration)**

This explorer allows you to:
- View all ProofSets associated with your address
- Check proof submission status
- Monitor proving periods
- Verify successful proof validations

## Step 6: Integration

Your PDP server is now ready to:
1. Accept pieces from UCAN servers
2. Generate and submit proofs to the network
3. Serve piece retrievals

### For UCAN Server Integration

When setting up a UCAN server, use:
- PDP URL: `https://YOUR_PDP_DOMAIN`
- Ensure the same `service.pem` is used (if running both services)

## Next Steps

- Set up a [UCAN server](./ucan-server.md) to accept client uploads
- Configure monitoring and alerting
- Plan for storage scaling
- Review [architecture documentation](../architecture.md) for system overview