# client

Interact with a running Piri node.

All client commands require a running Piri server and authenticate using JWT tokens signed with your node's identity key. Pass the same `--config` file you used with `piri serve` - this provides both the server address and your identity key for authentication:

```bash
piri --config=config.toml client admin config list
```

## Usage

```
piri client [command] [flags]
```

## Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--node-url <url>` | URL of a Piri node | `http://localhost:3000` |

## Subcommands

### [admin](admin/index.md)

Administrative operations (logs, config).

### [pdp](pdp/index.md)

PDP (Provable Data Possession) operations.