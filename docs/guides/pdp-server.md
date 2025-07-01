# Setup Piri PDP Server

This section walks you through setting up a Piri PDP (Proof of Data Possession) server. The PDP server stores data pieces and generates cryptographic proofs for the Filecoin network.

## Prerequisites

Before starting, ensure you have:

1. ✅ [Met system prerequisites](../setup/prerequisites.md)
2. ✅ [Configured and Synced a Lotus Node](../setup/prerequisites.md#filecoin-prerequisites)
3. ✅ [Configured TLS Termination](../setup/tls-termination.md)
4. ✅ [Installed Piri](../setup/installation.md)
5. ✅ [Generated a Key-Pair](../setup/key-generation.md)

## Start a Piri PDP Server

### Step 1: Import Delegated Filecoin Address into Piri

In this step, we will import the [delegated wallet we created with lotus](../setup/prerequisites.md#funded-delegated-wallet) into our Piri node.

> **Note**: The first time Piri is run, it will create a "data directory" with a default location `$HOME/.storacha`. This directory contains the state kept by piri. The directory location may be configured by passing the `--data-dir` flag or setting the environment variable `PIRI_DATA_DIR` when running a Piri command. If no configuration is provided, the default location `$HOME/.storacha` will be used.

```bash
# Export wallet to hex format
lotus wallet export YOUR_DELEGATED_ADDRESS > wallet.hex
# exports the private key of the address to wallet.hex

# Import to Piri
piri wallet import wallet.hex
# Stores the wallet details in piris data dir `$HOME/.storacha`

# Verify import (note the Ethereum-style address)
piri wallet list
# Example output: `Address: 0x7469B47e006D0660aB92AE560b27A1075EEcF97F`
```

### Step 2: Start Piri PDP Server

Here, we start a Piri PDP server, this is a long running process.

**Parameters:**
- `--lotus-url`: WebSocket endpoint of your [Lotus Node](../setup/prerequisites.md#lotus-node-setup)
- `--eth-address`: Ethereum address (the [delegated address](#step-1-import-delegated-filecoin-address-into-piri) you imported)

```bash
piri serve pdp --lotus-url=wss://YOUR_LOTUS_ENDPOINT/rpc/v1 --eth-address=YOUR_ETH_ADDRESS
```

**Expected Output:**
```bash
Server started! Listening on  http://localhost:3001
```

### Step 3: Create a Proof Set

Here, we create a Proof Set that Piri will interact with using the PDP Smart Contract.

**Parameters:**
- `--key-file`: PEM file containing your [identity](../setup/key-generation.md#generating-a-pem-file--did)
- `--record-keeper`: Address of a [PDP Service Contract](https://github.com/FilOzone/pdp/?tab=readme-ov-file#contracts)
  - Use: `0x6170dE2b09b404776197485F3dc6c968Ef948505`

```bash
piri client pdp proofset create --key-file=service.pem --record-keeper=0x6170dE2b09b404776197485F3dc6c968Ef948505 --wait > proofset.id
```

**Expected Output (while creating):**
```bash
Proof set being created, check status at:
/pdp/proof-sets/created/0xbd18dfe5e92c5840d593a6ec3cf7c0caf98cbf6c209c3cd3bd4f6234b7a6d002
⠏ Polling proof set status...
  Status: pending
  Transaction Hash: 0xbd18dfe5e92c5840d593a6ec3cf7c0caf98cbf6c209c3cd3bd4f6234b7a6d002
  Created: false
  Service: storacha
  Ready: false
```

**Expected Output (completed):**
```bash
/pdp/proof-sets/created/0xbd18dfe5e92c5840d593a6ec3cf7c0caf98cbf6c209c3cd3bd4f6234b7a6d002
✓ Proof set created successfully!
  Transaction Hash: 0xbd18dfe5e92c5840d593a6ec3cf7c0caf98cbf6c209c3cd3bd4f6234b7a6d002
  Service: storacha 0xbd18dfe5e92c5840d593a6ec3cf7c0caf98cbf6c209c3cd3bd4f6234b7a6d002
  ProofSet ID: 411
  Service: storacha
  Ready: true
```

Your ProofSet ID will be written to `proofset.id`.

---

## Next Steps

After setting up the PDP server:
- [Setup UCAN Server](./ucan-server.md) to accept client uploads
- [Validate Your Setup](../setup/validation.md)