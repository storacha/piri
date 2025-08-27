# Piri Configuration

This document describes the configuration file format for Piri nodes.



## Overview

Piri uses a TOML configuration file to manage all settings. The recommended approach is to generate this file using the `piri init` command, which ensures all required configuration is present.

### Generating a Configuration

The configuration file is generated automatically when you run `piri init`. See the [Piri Node Setup Guide](../guides/piri-server.md#initialize-your-piri-node) for detailed instructions on initialization.

### Using a Configuration File

```bash
piri serve full --config=config.toml
```

## Configuration File Format

The configuration file uses TOML format with the following sections:

### Example Configuration

```toml
[identity]
key_file = "service.pem"  # Path to Ed25519 PEM file containing node identity

[repo]
data_dir = "/var/lib/piri/data"  # Directory for permanent Piri state
temp_dir = "/var/tmp/piri"       # Directory for temporary/ephemeral data

[server]
host = "0.0.0.0"                        # Bind address for the HTTP server
port = 3000                             # Port number for the HTTP server
public_url = "https://piri.example.com" # Public-facing URL for your node

[pdp]
owner_address = "0x7469B47e006D0660aB92AE560b27A1075EEcF97F"  # Ethereum address for PDP operations
contract_address = "0x6170dE2b09b404776197485F3dc6c968Ef948505"  # PDP smart contract address
lotus_endpoint = "wss://lotus.example.com/rpc/v1"              # Lotus node WebSocket endpoint

[ucan]
proof_set = 123  # Proof set ID created during initialization

[ucan.services.indexer]
proof = "mAYIEA..."  # Delegation proof for indexing service (obtained during init)
did = "did:web:staging.indexer.warm.storacha.network"           # Indexing service DID
url = "https://staging.indexer.warm.storacha.network/claims"   # Indexing service URL

[ucan.services.upload]
did = "did:web:staging.upload.warm.storacha.network"    # Upload service DID
url = "https://staging.upload.warm.storacha.network"    # Upload service URL

[ucan.services.publisher]
ipni_announce_urls = [
  "https://cid.contact/announce",
  "https://dev.cid.contact/announce"
]  # IPNI announcement endpoints

[ucan.services.principal_mapping]
# Maps service DIDs to their principal DIDs
"did:web:staging.upload.warm.storacha.network" = "did:key:z6MkrZ1r5XBFZjBU34qyD8fueMbMRkKw17BZaq2ivKFjnz2z"
```

### Configuration Sections

#### `[identity]`
Contains the path to your node's cryptographic identity file (Ed25519 key pair in PEM format).

#### `[repo]`
Defines where Piri stores its data:
- `data_dir`: Persistent state that survives restarts
- `temp_dir`: Temporary data that can be cleared

#### `[server]`
HTTP server settings:
- `host`: Network interface to bind to
- `port`: Port number for the HTTP server
- `public_url`: The public URL where your node is accessible

#### `[pdp]`
Proof of Data Possession configuration:
- `owner_address`: Ethereum address with funds for on-chain transactions
- `contract_address`: Address of the PDP smart contract
- `lotus_endpoint`: WebSocket URL of your Lotus node

#### `[ucan]`
UCAN and Storacha network settings:
- `proof_set`: ID of the proof set created during initialization
- `services`: Configuration for various Storacha services including indexer, upload, and publisher settings
