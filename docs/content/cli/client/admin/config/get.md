# get

Get the current runtime value of a dynamic configuration key.

This is the value Piri is using right now, which may differ from the config file if a runtime override has been applied via `set`.

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
