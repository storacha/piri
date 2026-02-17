# set

Set a log level for one system or all systems.

## Usage

```
piri client admin log set <level> [system]
```

## Arguments

| Argument | Description |
|----------|-------------|
| `<level>` | Log level to set (debug, info, warn, error) |
| `[system]` | Specific system name. If omitted, sets level for all systems |

## Example

```bash
piri client admin log set debug piri/pdp
```

```json
{
  "piri/pdp": "debug"
}
```
