# Configuration

Piri uses a TOML configuration file. The `piri init` command generates a complete config file for you - this reference helps you understand and tune it.

## Priority

Configuration is evaluated in this order (highest to lowest priority):

1. CLI flags
2. Environment variables
3. Config file
4. Network preset defaults
5. Built-in defaults

## Config File Locations

Piri searches for config files in this order:

1. Path specified via `--config` flag
2. `~/.config/piri/config.toml`
3. `piri-config.toml` in current directory

## Environment Variables

Override any config value with `PIRI_` prefix, replacing dots with underscores:

```
repo.data_dir → PIRI_REPO_DATA_DIR
```

## Dynamic Configuration

Most settings require a restart. The following can be changed at runtime via the [admin config commands](../cli/client/admin/config/index.md):

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| [`pdp.aggregation.manager.poll_interval`](pdp/aggregation/manager.md#poll_interval) | duration | `30s` | How often the aggregation manager polls for new work |
| [`pdp.aggregation.manager.batch_size`](pdp/aggregation/manager.md#batch_size) | duration | `10` | Maximum number of items to process in a single batch |

## Minimal Example

```toml
network = "forge-prod"

[identity]
key_file = "/path/to/service.pem"

[repo]
data_dir = "/data/piri"
temp_dir = "/tmp/piri"

[server]
port = 3000
host = "0.0.0.0"
public_url = "https://piri.example.com"

[pdp]
owner_address = "0x..."
lotus_endpoint = "wss://lotus.example.com/rpc/v1"

[ucan]
proof_set = 123
```

## Sections

### [network](network.md)

Network selection (`forge-prod`, `warm-staging`).

### [identity](identity.md)

Node identity configuration.

### [repo](repo/index.md)

Storage directory configuration.

### [server](server.md)

HTTP server configuration.

### [pdp](pdp/index.md)

PDP (Provable Data Possession) configuration.

### [ucan](ucan.md)

UCAN service configuration.

### [telemetry](telemetry.md)

Observability configuration.
