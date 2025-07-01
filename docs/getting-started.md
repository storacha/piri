# Getting Started with Piri

This comprehensive guide walks you through setting up Piri services from scratch. Follow each section in order to ensure a successful deployment.

---

## Prerequisites

This document outlines the common prerequisites for running Piri services. Specific services may have additional requirements noted in their respective guides.

### System Requirements

#### Operating System
- **Linux-based OS** (Ubuntu 20.04+ recommended)

#### Hardware
- **CPU**: 4+ cores
- **RAM**: 8+ GB
- **Storage**: 1+ TB
- **Network**: 1+ Gbps symmetric connection

### Software Requirements

#### Required Packages

Install the following packages:

```bash
sudo apt update && sudo apt install -y make git jq curl wget nginx certbot python3-certbot-nginx
```

### Network Requirements

#### Domain Names
You'll need **two fully qualified domain names (FQDN)**:
- `piri.example.com` - for Piri UCAN server
- `up.piri.example.com` - for Piri PDP Server

#### Firewall Configuration
Open the following ports:

```bash
# HTTP/HTTPS (required)
ufw allow 80/tcp 
ufw allow 443/tcp
```

### Filecoin Prerequisites

#### Lotus Node Setup
A [Lotus node](https://github.com/filecoin-project/lotus) is required for interacting with the PDP Smart Contract deployed on the Filecoin [Calibration Network](https://docs.filecoin.io/networks/calibration).

**Requirements:**
- Latest Calibration Network [Snapshot](https://forest-archive.chainsafe.dev/latest/calibnet/)
- Synced Lotus node with [ETH RPC enabled](https://lotus.filecoin.io/lotus/configure/ethereum-rpc/#enableethrpc)
- WebSocket endpoint (e.g., `wss://lotus.example.com/rpc/v1`)
- Basic understanding of Filecoin primitives

#### Funded Delegated Wallet

A Lotus Delegated Address is required by Piri for interacting with the PDP Smart Contract. This guide assumes you have already setup a lotus node as described in the [pre-requisites document](./prerequisites.md). Please refer to the official [Filecoin Docs](https://docs.filecoin.io/smart-contracts/filecoin-evm-runtime/address-types#delegated-addresses) for more details on delegated addresses.

**Step 1: Generate a Delegated Address**

```bash
lotus wallet new delegated
```

Example output: `t410fzmmaqcn3j6jidbyrqqsvayejt6sskofwash6zoi`

**Step 2: Fund the Address**

1. Visit the [Calibration faucet](https://faucet.calibnet.chainsafe-fil.io/funds.html)
2. Request funds for your new address
3. Verify funding:

   ```bash
   lotus wallet balance YOUR_DELEGATED_ADDRESS
   ```

---

## TLS Termination

Piri servers (both PDP and UCAN) do not handle TLS termination directly. For production deployments, you must use a reverse proxy to handle HTTPS connections and route traffic from your domains to the appropriate Piri servers.

### Overview

This section configures how your domains (from the [Network Requirements](#network-requirements)) connect to your Piri servers:

```
Internet → Your Domain → Nginx (HTTPS) → Piri Server (HTTP)
         ↓                   ↓                ↓
   piri.example.com      Port 443        Port 3000 (UCAN)
   up.piri.example.com   Port 443        Port 3001 (PDP)
```

### Why TLS Termination is Required

- **Security**: Encrypts data in transit between clients and your server
- **Trust**: Required for browser connections and API integrations
- **Network Requirements**: Storacha Network requires HTTPS endpoints
- **Certificate Management**: Centralized SSL certificate handling

### DNS Configuration

Before proceeding, ensure your domains point to your server:

1. Configure DNS A records for both domains to point to your server's IP address:
   - `piri.example.com` → Your server IP
   - `up.piri.example.com` → Your server IP

2. Verify DNS propagation:
   ```bash
   dig piri.example.com
   dig up.piri.example.com
   ```

### Setting up Nginx

Nginx acts as a reverse proxy, accepting HTTPS connections on your domains and forwarding them to the appropriate Piri servers running locally.

#### Prerequisites

```bash
# Install Nginx and Certbot
sudo apt update
sudo apt install -y nginx certbot python3-certbot-nginx
```

#### Configuration Steps

**Step 1: Create Configuration Files**

Create separate configuration files for each domain:
- `/etc/nginx/sites-available/piri.example.com` (for UCAN server)
- `/etc/nginx/sites-available/up.piri.example.com` (for PDP server)

**Step 2: Configure UCAN Server (piri.example.com → Port 3000)**

Create `/etc/nginx/sites-available/piri.example.com`:

```nginx
server {
    server_name piri.example.com;  # Replace with your actual UCAN domain
    
    # For UCAN server handling client uploads
    client_max_body_size 0;           # Allow unlimited file uploads
    client_body_timeout 300s;         # Timeout for slow uploads
    client_header_timeout 300s;       # Timeout for slow connections
    send_timeout 300s;                # Timeout for sending responses
    
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
        
        proxy_request_buffering off; # Stream uploads directly to backend
    }
}
```

**Step 3: Configure PDP Server (up.piri.example.com → Port 3001)**

Create `/etc/nginx/sites-available/up.piri.example.com`:

```nginx
server {
    server_name up.piri.example.com;  # Replace with your actual PDP domain
    
    # PDP server also handles large uploads
    client_max_body_size 0;           # Allow unlimited file uploads
    client_body_timeout 300s;         # Timeout for slow uploads
    client_header_timeout 300s;       # Timeout for slow connections
    send_timeout 300s;                # Timeout for sending responses
    
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
        
        proxy_request_buffering off; # Stream uploads directly to backend
    }
}
```

**Step 4: Enable the Sites**

Enable both nginx configurations:

```bash
# Enable UCAN server configuration
sudo ln -s /etc/nginx/sites-available/piri.example.com /etc/nginx/sites-enabled/

# Enable PDP server configuration  
sudo ln -s /etc/nginx/sites-available/up.piri.example.com /etc/nginx/sites-enabled/

# Test configuration
sudo nginx -t

# Reload nginx
sudo systemctl reload nginx
```

**Step 5: Obtain SSL Certificates**

Obtain SSL certificates for both domains:

```bash
# For UCAN server domain (replace with your actual domain)
sudo certbot --nginx -d piri.example.com

# For PDP server domain (replace with your actual domain)
sudo certbot --nginx -d up.piri.example.com
```

### Port Configuration

**Default ports:**
- **UCAN Server**: 3000 (configurable via `--port`)
- **PDP Server**: 3001 (configurable via `--port`)
- **HTTPS**: 443 (standard)
- **HTTP**: 80 (redirect to HTTPS)

### Testing Your Configuration

After setting up TLS termination, verify HTTPS connectivity for both domains:

```bash
# Test UCAN server domain
curl -I https://piri.example.com

# Test PDP server domain  
curl -I https://up.piri.example.com
```

Both should return HTTP status 502 (Bad Gateway) until the Piri servers are started.

---

## Installing Piri

This section covers the installation of the Piri binary.

### Download Pre-compiled Binary

Download the latest release from [v0.0.9](https://github.com/storacha/piri/releases/tag/v0.0.9):

**For Linux AMD64:**
```bash
wget https://github.com/storacha/piri/releases/download/v0.0.9/piri_0.0.9_linux_amd64.tar.gz
tar -xzf piri_0.0.9_linux_amd64.tar.gz
sudo mv piri /usr/local/bin/piri
```

**For Linux ARM64:**
```bash
wget https://github.com/storacha/piri/releases/download/v0.0.9/piri_0.0.9_linux_arm64.tar.gz
tar -xzf piri_0.0.9_linux_arm64.tar.gz
sudo mv piri /usr/local/bin/piri
```

### Verify Installation

```bash
# View available commands
piri --help
```

---

## Generating and Managing DIDs & Cryptographic Keys

The `service.pem` file contains your storage provider's cryptographic identity, which corresponds to its DID. This single file is shared by all Piri services _you_ operate to maintain a consistent identity.

### Key Requirements

Piri requires an **Ed25519** private key. Ed25519 is a modern elliptic curve signature scheme.

### Generating a PEM File & DID

**Step 1: Generate a new Ed25519 key**

```bash
piri identity generate > service.pem
```

**Step 2: Verify and derive your DID**

```bash
piri identity parse service.pem
```

Example output: `did:key:z6MkhaXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX`

### Security Considerations

- **Protect this file**: It contains your private key
- **Set appropriate file permissions**: `chmod 600 service.pem`
- **Backup securely**: Loss of this file means loss of your provider identity

---

## Setup Piri PDP Server

This section walks you through setting up a Piri PDP (Proof of Data Possession) server. The PDP server stores data pieces and generates cryptographic proofs for the Filecoin network.

### Prerequisites

Before starting, ensure you have:

1. [Met system prerequisites](#prerequisites)
2. [Configured and Synced a Lotus Node](#filecoin-prerequisites)
3. [Configured TLS Termination](#tls-termination)
4. [Installed Piri](#installing-piri)
5. [Generated a Key-Pair](#generating-and-managing-dids--cryptographic-keys)

### Start a Piri PDP Server

#### Step 1: Import Delegated Filecoin Address into Piri

In this step, we will import the [delegated wallet we created with lotus](#funded-delegated-wallet) into our Piri node.

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

#### Step 2: Start Piri PDP Server

Here, we start a Piri PDP server, this is a long running process.

**Parameters:**
- `--lotus-url`: WebSocket endpoint of your [Lotus Node](#lotus-node-setup)
- `--eth-address`: Ethereum address (the [delegated address](#step-1-import-delegated-filecoin-address-into-piri) you imported)

```bash
piri serve pdp --lotus-url=wss://YOUR_LOTUS_ENDPOINT/rpc/v1 --eth-address=YOUR_ETH_ADDRESS
```

**Expected Output:**
```bash
Server started! Listening on  http://localhost:3001
```

#### Step 3: Create a Proof Set

Here, we create a Proof Set that Piri will interact with using the PDP Smart Contract.

**Parameters:**
- `--key-file`: PEM file containing your [identity](#generating-a-pem-file--did)
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

## Setup Piri UCAN Server

This section walks you through setting up a Piri UCAN (User Controlled Authorization Network) server. The UCAN server accepts data uploads from Storacha network clients and routes them to a PDP backend.

### Overview

The Piri UCAN server:
- Provides the client-facing API for the Storacha network
- Handles UCAN-based authentication and authorization
- Routes uploaded data to PDP servers (Piri or Curio)
- Manages delegations from the Storacha network

### Prerequisites

Before starting, ensure you have:

1. [Met system prerequisites](#prerequisites)
2. [Configured and Synced a Lotus Node](#filecoin-prerequisites)
3. [Configured TLS Termination](#tls-termination)
4. [Installed Piri](#installing-piri)
5. [Generated a Key-Pair](#generating-and-managing-dids--cryptographic-keys)
6. [Setup a Piri PDP Server](#setup-piri-pdp-server)

### Start a Piri UCAN Server

#### Step 1: Register your DID with Storacha Team

Share the [DID you created](#generating-a-pem-file--did) with the storacha team. 

> **Note**: This is the Public Key: `did:key:z6MkhaXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX` value created previously.

#### Step 2: Start Piri UCAN Server for Registration

```bash
piri serve ucan --key-file=service.pem
```

**Expected output:**
```bash
▗▄▄▖ ▄  ▄▄▄ ▄   ▗
▐▌ ▐▌▄ █    ▄   █▌
▐▛▀▘ █ █    █  ▗█▘
▐▌   █      █  ▀▘

🔥 v0.0.9
🆔 did:key:z6Mko7NFux3RoDDQUjmbnc7ccCqxnLV3tju8zwai2XFbRbU6
🚀 Ready!
```

#### Step 3: Obtain a delegation from the Storacha Delegator

Visit https://staging.delegator.storacha.network and select 'Start Onboarding', following the guide until completion. 

**During onboarding you will:**
- Provide a delegation proof allowing the Storacha upload service to write to your piri node
- Receive a delegation proof allowing your Piri node to write to the Storacha indexing service
- Receive necessary configuration to connect to Storacha services

#### Step 4: Restart Piri with Provided Configuration

Upon completing step 3, the delegator will have provided you with a set of environment variables to configure on your Piri node.

##### 4.1 Save Delegator Provided Configuration

Create a `.env` file with the configuration received:

```bash
# Example .env file
export INDEXING_SERVICE_DID="did:web:staging.indexing.storacha.network"
export INDEXING_SERVICE_URL="https://staging.indexing.storacha.network"
export INDEXING_SERVICE_PROOF="bafyrei..."
export UPLOAD_SERVICE_DID="did:web:staging.upload.storacha.network"
export UPLOAD_SERVICE_URL="https://staging.upload.storacha.network"
...
```

##### 4.2 Source the .env file

```bash
source .env
```

##### 4.3 Restart your Piri node

```bash
piri serve ucan --key-file=service.pem --pdp-server-url=https://up.piri.example.com
```

> **Note**: Replace `up.piri.example.com` with your actual PDP server domain configured in the [TLS Termination](#tls-termination) section.

**Expected output:**
```bash
▗▄▄▖ ▄  ▄▄▄ ▄   ▗
▐▌ ▐▌▄ █    ▄   █▌
▐▛▀▘ █ █    █  ▗█▘
▐▌   █      █  ▀▘

🔥 v0.0.9
🆔 did:key:z6Mko7NFux3RoDDQUjmbnc7ccCqxnLV3tju8zwai2XFbRbU6
🚀 Ready!
```

---

## Validate Your Piri Setup

You are now running both a UCAN and PDP server with Piri!

### Validation Steps

1. **Test data reception**: Visit https://staging.delegator.storacha.network/test-storage and follow the steps

2. **Inspect your proof set**: Visit https://pdpscan.vercel.app/calibration/proofsets and find your proof set by ID

---

Congratulations! Your Piri setup is complete and ready to receive data from the Storacha Network.