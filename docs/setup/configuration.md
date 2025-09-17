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
	key_file = "/path/to/service.pem"

[repo]
	data_dir = "/var/lib/piri/data"
	temp_dir = "/var/tmp/piri"

[server]
	host = "0.0.0.0"
	port = 3000
	public_url = "https://piri.example.com"

[pdp]
	owner_address = "0x..."
	contract_address = "0x..."
	lotus_endpoint = "wss://lotus.example.com/rpc/v1"

[ucan]
	proof_set = 123
	[ucan.services]
		[ucan.services.indexer]
			proof = "mAYIEA..."
		[ucan.services.etracker]
			proof = "mAYIEA..."
```

### Configuration Sections

#### `[identity]`
- `key_file`: Has the path to your node's identity key file (Ed25519 key pair in PEM format).

#### `[repo]`
- `data_dir`: Directory piri maintains state and persists permentant data.
- `temp_dir`: Directory piri maintains ephemeral data.

#### `[server]`
- `host`: Network address piri listens on
- `port`: Port number piri listens on
- `public_url`: The public web address where people can reach your piri node

#### `[pdp]`
- `owner_address`: Your Ethereum-style address blockchain transactions will be sent from
- `contract_address`: Ethereum-style address of the [PDP Service smart contract](https://github.com/FilOzone/pdp/?tab=readme-ov-file#v110)
- `lotus_endpoint`: WebSocket (`ws://`) or WebSocket Secure (`wss://`) URL of your Lotus node

#### `[ucan]`
- `proof_set`: ID piri will submit proofs to

#### `[ucan.services.indexer]`
- `proof`: UCAN delegation proof permitting Piri to communicate with the Storacha Network
