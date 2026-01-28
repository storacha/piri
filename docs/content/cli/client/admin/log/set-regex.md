# set-regex

Set log level for subsystems matching a regex pattern.

## Usage

```
piri client admin log set-regex <level> <expression>
```

## Arguments

| Argument | Description |
|----------|-------------|
| `<level>` | Log level to set (debug, info, warn, error) |
| `<expression>` | Regular expression pattern to match system names |

## Example

```bash
piri client admin log set-regex debug "piri/pdp.*"
```

```json
{
  "piri/pdp": "debug",
  "piri/pdp/scheduler": "debug",
  "piri/pdp/aggregation": "debug"
}
```
