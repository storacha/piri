# Setup Piri UCAN Server

This section walks you through setting up a Piri UCAN (User Controlled Authorization Network) server. The UCAN server accepts data uploads from Storacha network clients and routes them to a PDP backend.

## Overview

The Piri UCAN server:
- Provides the client-facing API for the Storacha network
- Handles UCAN-based authentication and authorization
- Routes uploaded data to PDP servers (Piri or Curio)
- Manages delegations from the Storacha network

## Prerequisites

Before starting, ensure you have:

1. ✅ [Met system prerequisites](../setup/prerequisites.md)
2. ✅ [Configured and Synced a Lotus Node](../setup/prerequisites.md#filecoin-prerequisites)
3. ✅ [Configured TLS Termination](../setup/tls-termination.md)
4. ✅ [Installed Piri](../setup/installation.md)
5. ✅ [Generated a Key-Pair](../setup/key-generation.md)
6. ✅ [Setup a Piri PDP Server](./pdp-server.md)

## Start a Piri UCAN Server

### Step 1: Register your DID with Storacha Team

Share the [DID you created](../setup/key-generation.md#generating-a-pem-file--did) with the storacha team. 

> **Note**: This is the Public Key: `did:key:z6MkhaXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX` value created previously.

### Step 2: Start Piri UCAN Server for Registration

```bash
piri serve ucan --key-file=service.pem
```

**Expected output:**
```bash
▗▄▄▖ ▄  ▄▄▄ ▄   ▗
▐▌ ▐▌▄ █    ▄   █▌
▐▛▀▘ █ █    █  ▗█▘
▐▌   █      █  ▀▘

🔥 v0.0.12
🆔 did:key:z6Mko7NFux3RoDDQUjmbnc7ccCqxnLV3tju8zwai2XFbRbU6
🚀 Ready!
```

### Step 3: Obtain a delegation from the Storacha Delegator

Visit https://staging.delegator.storacha.network and select 'Start Onboarding', following the guide until completion. 

**During onboarding you will:**
- Provide a delegation proof allowing the Storacha upload service to write to your piri node
- Receive a delegation proof allowing your Piri node to write to the Storacha indexing service
- Receive necessary configuration to connect to Storacha services

### Step 4: Restart Piri with Provided Configuration

Upon completing step 3, the delegator will have provided you with a set of environment variables to configure on your Piri node.

#### 4.1 Save Delegator Provided Configuration

Create a `.env` file with the configuration received:

```bash
# Example .env file
export INDEXING_SERVICE_DID="did:web:staging.indexer.warm.storacha.network"
export INDEXING_SERVICE_URL="https://staging.indexer.warm.storacha.network/claims"
export INDEXING_SERVICE_PROOF="mAYIEA..."
export UPLOAD_SERVICE_DID="did:web:staging.upload.warm.storacha.network"
export UPLOAD_SERVICE_URL="https://staging.upload.warm.storacha.network"
...
```

#### 4.2 Source the .env file

```bash
source .env
```

#### 4.3 Restart your Piri node

```bash
piri serve ucan --key-file=service.pem --pdp-server-url=https://up.piri.example.com --proof-set=123
```

> **Note**:
> - Replace `up.piri.example.com` with your actual PDP server domain configured in the [TLS Termination](../setup/tls-termination.md) section.
> - Replace `123` with the proof set ID you got when [a new proofset was created](./pdp-server.md#step-3-create-a-proof-set) as part of configuring the PDP server.

**Expected output:**
```bash
▗▄▄▖ ▄  ▄▄▄ ▄   ▗
▐▌ ▐▌▄ █    ▄   █▌
▐▛▀▘ █ █    █  ▗█▘
▐▌   █      █  ▀▘

🔥 v0.0.12
🆔 did:key:z6Mko7NFux3RoDDQUjmbnc7ccCqxnLV3tju8zwai2XFbRbU6
🚀 Ready!
```

---

## Next Steps

After setting up the UCAN server:
- [Validate Your Setup](../setup/validation.md)
