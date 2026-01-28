# CLI Reference

Piri provides a command-line interface for managing storage nodes on the Storacha network.

## Usage

```
piri [command] [flags]
```

## Global Flags

| Flag | Description |
|------|-------------|
| `--config <path>` | Config file path. Defaults to `~/.config/piri/config.toml` |
| `--log-level <level>` | Logging level (debug, info, warn, error) |
| `--data-dir <path>` | Storage service data directory |
| `--temp-dir <path>` | Storage service temp directory |
| `--key-file <path>` | Path to PEM file containing ed25519 private key |

## Subcommands

### [init](init.md)

Initialize a new Piri node.

### [serve](serve/index.md)

Start the Piri server.

### [client](client/index.md)

Interact with a running Piri node.

### [wallet](wallet/index.md)

Manage wallets.

### [identity](identity/index.md)

Generate and manage node identity.

### [status](status/index.md)

Show node status.

### [update](update.md)

Check for and apply updates to Piri.
