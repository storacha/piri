# get

Get a specific dynamic configuration value.

## Usage

```
piri client admin config get <key>
```

## Arguments

| Argument | Description |
|----------|-------------|
| `<key>` | The configuration key to retrieve |

## Example

```bash
piri client admin config get pdp.aggregation.manager.poll_interval
```

```
30s
```

See [Dynamic Configuration](../../../../configuration/index.md#dynamic-configuration) for the list of available keys.
