# init

Initialize your Piri node and register it with the Storacha network.

This is a **one-time setup command** that prepares your node to participate in the network. It performs several critical operations:

1. **Validates your configuration** - Checks that all required files and endpoints are accessible
2. **Imports your wallet** - Adds your Filecoin delegated address to Piri's wallet for on-chain transactions
3. **Creates a proof set** - Registers a proof set on the PDP smart contract (this can take up to 5 minutes as it waits for on-chain confirmation)
4. **Registers with the network** - Signs up your node with the Storacha delegator service and receives authorization proofs
5. **Generates a configuration file** - Outputs a complete TOML config file that [`piri serve`](serve/index.md) uses to run your node

After initialization completes, you use the generated config file with `piri serve` to start your node. You typically only run `piri init` once per node setup.

This command is **idempotent** - running it multiple times with the same parameters is safe and will reuse existing resources (like your proof set) rather than creating duplicates.

## Prerequisites

Before running this command, you need:

- A synced [Lotus node](../setup/prerequisites.md#lotus-node-setup) with ETH RPC enabled
- A [funded delegated wallet](../setup/prerequisites.md#funded-delegated-wallet) exported to hex format
- An [Ed25519 identity key](../setup/key-generation.md) in PEM format (generated with `piri identity generate`)
- A domain with [TLS configured](../setup/tls-termination.md) pointing to your server
- Directories created for data and temp storage

See the [Setup Guide](../setup/piri-server.md) for the complete walkthrough.

## Usage

```
piri init [flags]
```

## Flags

All flags are required:

| Flag | Description |
|------|-------------|
| `--network <network>` | Network to join (`forge-prod` for mainnet, `warm-staging` for calibration) |
| `--data-dir <path>` | Directory for permanent Piri data (blobs, database) |
| `--temp-dir <path>` | Directory for temporary data during processing |
| `--key-file <path>` | Path to PEM file containing your Ed25519 identity key |
| `--wallet-file <path>` | Path to hex file containing your delegated Filecoin wallet private key |
| `--lotus-endpoint <url>` | WebSocket URL of your Lotus node (e.g., `wss://lotus.example.com/rpc/v1`) |
| `--operator-email <email>` | Contact email for the Storacha team to reach you |
| `--public-url <url>` | Public HTTPS URL where your node will be accessible |

## Example

```bash
piri init \
  --network=forge-prod \
  --data-dir=/var/lib/piri/data \
  --temp-dir=/var/lib/piri/temp \
  --key-file=/etc/piri/service.pem \
  --wallet-file=/etc/piri/wallet.hex \
  --lotus-endpoint=wss://lotus.example.com/rpc/v1 \
  --operator-email=admin@example.com \
  --public-url=https://piri.example.com > config.toml
```

```
Initializing your Piri node on the Storacha Network...

[1/5] Validating configuration...
Configuration validated

[2/5] Creating Piri node...
Node created with DID: did:key:z6MkhaXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX

[3/5] Setting up proof set...
Creating new proof set...
Waiting for proof set creation to be confirmed on-chain...
Proof set created with ID: 123

[4/5] Registering with delegator service...
Successfully registered with delegator service
Received delegator proof

[5/5] Generating configuration file...

Initialization complete!
```

The config file is written to stdout. Use `> config.toml` to save it to a file, or place it directly in `~/.config/piri/config.toml` for automatic loading by `piri serve`.

## What's Next

After initialization, start your node:

```bash
piri serve --config=config.toml
```

See [`piri serve`](serve/index.md) for details on running the server.
