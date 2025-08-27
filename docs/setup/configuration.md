# Piri Configuration

This document explains the configuration file for Piri nodes.

## Overview

Piri uses a TOML configuration file for all settings. The best way is to create this file using the `piri init` command, which makes sure you have everything you need.

### Generating a Configuration

The configuration file is created when you run `piri init`. See the [Piri Node Setup Guide](../guides/piri-server.md#initialize-your-piri-node) for how to do this.

### Using a Configuration File

```bash
piri serve full --config=config.toml
```

## Configuration File Format

The configuration file uses TOML format with these parts:

### Example Configuration

```toml
[identity]
key_file = "service.pem"  # Path to PEM file with your node's identity key

[repo]
data_dir = "/var/lib/piri/data"  # Folder for permanent Piri data
temp_dir = "/var/tmp/piri"       # Folder for temporary data

[server]
host = "0.0.0.0"                        # Network address to listen on
port = 3000                             # Port number to listen on
public_url = "https://piri.example.com" # Public web address for your node

[pdp]
owner_address = "0x7469B47e006D0660aB92AE560b27A1075EEcF97F"  # Your Ethereum address with funds
contract_address = "0x6170dE2b09b404776197485F3dc6c968Ef948505"  # Address of the PDP smart contract
lotus_endpoint = "wss://lotus.example.com/rpc/v1"              # WebSocket address of your Lotus node

[ucan]
proof_set = 123  # Proof set ID created during setup

[ucan.services.indexer]
proof = "mAYIEA..."  # Permission proof for indexing service (got during setup)
did = "did:web:staging.indexer.warm.storacha.network"           # Indexing service ID
url = "https://staging.indexer.warm.storacha.network/claims"   # Indexing service address

[ucan.services.upload]
did = "did:web:staging.upload.warm.storacha.network"    # Upload service ID
url = "https://staging.upload.warm.storacha.network"    # Upload service address

[ucan.services.publisher]
ipni_announce_urls = [
  "https://cid.contact/announce",
  "https://dev.cid.contact/announce"
]  # IPNI announcement addresses

[ucan.services.principal_mapping]
# Links service IDs to their main IDs
"did:web:staging.upload.warm.storacha.network" = "did:key:z6MkrZ1r5XBFZjBU34qyD8fueMbMRkKw17BZaq2ivKFjnz2z"
```

### Configuration Sections

#### `[identity]`
Has the path to your node's identity key file (Ed25519 key pair in PEM format).

#### `[repo]`
Says where Piri keeps its data:
- `data_dir`: Data that stays after restart
- `temp_dir`: Data that can be deleted

#### `[server]`
Web server settings:
- `host`: Network address to listen on
- `port`: Port number to listen on
- `public_url`: The public web address where people can reach your node

#### `[pdp]`
Proof of Data Possession settings:
- `owner_address`: Your Ethereum address with money for blockchain transactions
- `contract_address`: Address of the PDP smart contract
- `lotus_endpoint`: WebSocket address of your Lotus node

#### `[ucan]`
UCAN and Storacha network settings:
- `proof_set`: ID of the proof set created during setup

#### `[ucan.services.indexer]`
Indexing service settings:
- `proof`: Permission proof from the delegator service
- `did`: Service identifier
- `url`: Service web address

#### `[ucan.services.upload]`
Upload service settings:
- `did`: Service identifier
- `url`: Service web address

#### `[ucan.services.publisher]`
Publisher settings:
- `ipni_announce_urls`: List of IPNI announcement addresses

#### `[ucan.services.principal_mapping]`
Links service DIDs to their main DIDs for authentication
