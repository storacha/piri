# list

List the current runtime values of all dynamic configuration keys.

These are the values Piri is using right now. They may differ from the config file if runtime overrides have been applied via `set`.

## Usage

```
piri client admin config list
```

## Example

```bash
piri client admin config list
```

```json
{
  "pdp.aggregation.manager.batch_size": 10,
  "pdp.aggregation.manager.poll_interval": "30s"
}
```

See [Dynamic Configuration](../../../../configuration/index.md#dynamic-configuration) for the list of available keys.
