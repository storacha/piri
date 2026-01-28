# network

Network selection for connecting to Storacha.

| Key | Default | Env | Dynamic |
|-----|---------|-----|---------|
| `network` | - | `PIRI_NETWORK` | No |

## Overview

The `network` field selects which Storacha network to connect to. This automatically configures service URLs, contract addresses, and chain IDs.

## Values

| Network | Description |
|---------|-------------|
| `forge-prod` | Production network (recommended) |
| `warm-staging` | Staging environment for testing |

## TOML

```toml
network = "forge-prod"
```
