# set

Set a dynamic configuration value.

## Usage

```
piri client admin config set <key> <value> [flags]
```

## Arguments

| Argument | Description |
|----------|-------------|
| `<key>` | The configuration key to set |
| `<value>` | The new value |

## Flags

| Flag | Description |
|------|-------------|
| `--persist` | Persist the change to the config file |

## Example

```bash
piri client admin config set pdp.aggregation.manager.poll_interval 1m
```

```
pdp.aggregation.manager.poll_interval updated
```

See [Dynamic Configuration](../../../../configuration/index.md#dynamic-configuration) for the list of available keys.
