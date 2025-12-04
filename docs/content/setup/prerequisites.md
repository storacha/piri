# Prerequisites

This document outlines the common prerequisites for running Piri services. Specific services may have additional requirements noted in their respective guides.

## System Requirements

### Operating System
- **Linux-based OS** (Ubuntu 20.04+ recommended)

### Hardware
- **CPU**: 4+ cores
- **RAM**: 8+ GB
- **Storage**: 1+ TB
- **Network**: 1+ Gbps symmetric connection

## Software Requirements

### Required Packages

Install the following packages:

```bash
sudo apt update && sudo apt install -y make git jq curl wget nginx certbot python3-certbot-nginx
```

## Network Requirements

### Domain
You'll need **a domain**, e.g. `piri.example.com` 
### Firewall Configuration
Ensure the following ports are open for ingress and egress:

- `80` 
- `443`

## Filecoin Prerequisites

### Lotus Node Setup
A [Lotus node](https://github.com/filecoin-project/lotus) is required for interacting with the PDP Smart Contract. Use the correct network for your environment:

<div class="env-block" data-env="production" markdown="1">
  <div class="env-block__label"><span class="env-block__pill">Prod</span> Use a mainnet Lotus node</div>

- Network: **Mainnet**
- Snapshot: [Latest Mainnet Snapshot](https://forest-archive.chainsafe.dev/latest/mainnet/)
- ETH RPC: [Enable ETH RPC](https://lotus.filecoin.io/lotus/configure/ethereum-rpc/#enableethrpc)
- Endpoint: `wss://<your-mainnet-lotus>/rpc/v1`
</div>

<div class="env-block" data-env="staging" markdown="1">
  <div class="env-block__label"><span class="env-block__pill">Staging</span> Use a calibration Lotus node</div>

- Network: **Calibration**
- Snapshot: [Latest Calibration Snapshot](https://forest-archive.chainsafe.dev/latest/calibnet/)
- ETH RPC: [Enable ETH RPC](https://lotus.filecoin.io/lotus/configure/ethereum-rpc/#enableethrpc)
- Endpoint: `wss://<your-calib-lotus>/rpc/v1`
</div>

### Funded Delegated Wallet

A Lotus Delegated Address is required by Piri for interacting with the PDP Smart Contract. This guide assumes you have already setup a lotus node as described 'Filecoin Prerequisite' above. Please refer to the official [Filecoin Docs](https://docs.filecoin.io/smart-contracts/filecoin-evm-runtime/address-types#delegated-addresses) for more details on delegated addresses.

**Step 1: Generate a Delegated Address**

```bash
lotus wallet new delegated
```

Example output: `t410fzmmaqcn3j6jidbyrqqsvayejt6sskofwash6zoi`

**Step 2: Fund the Address**

<div class="env-block" data-env="production" markdown="1">
  <div class="env-block__label"><span class="env-block__pill">Prod</span> Fund with mainnet FIL</div>

1. Send mainnet FIL to your delegated address (from your preferred wallet/exchange).
2. Verify funding:

   ```bash
   lotus wallet balance YOUR_DELEGATED_ADDRESS
   ```
</div>

<div class="env-block" data-env="staging" markdown="1">
  <div class="env-block__label"><span class="env-block__pill">Staging</span> Fund with calibration FIL</div>

1. Visit the [Calibration faucet](https://faucet.calibnet.chainsafe-fil.io/funds.html)
2. Request funds for your new address
3. Verify funding:

   ```bash
   lotus wallet balance YOUR_DELEGATED_ADDRESS
   ```
</div>

---

## Next Steps

Once you've completed all prerequisites:

- [Install Piri](./installation.md)
- [Generate Cryptographic Keys](./key-generation.md)
- [Configure TLS Termination](./tls-termination.md)
